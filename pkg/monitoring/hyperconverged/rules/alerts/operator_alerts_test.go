package alerts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/metrics"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/rules"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/rules/alerts"
)

const (
	rulesFile = "/tmp/rules.json"
	testFile  = "/tmp/rules.test"
)

var _ = Describe("OperatorAlerts", func() {
	var (
		unitTest *unitTest

		unsafeModificationsMetric *operatormetrics.GaugeVec
	)

	BeforeEach(func() {
		var err error

		err = rules.SetupRules()
		Expect(err).NotTo(HaveOccurred())

		promRule, err := rules.BuildPrometheusRule("kubevirt", metav1.OwnerReference{})
		Expect(err).NotTo(HaveOccurred())

		unitTest, err = NewUnitTest(promRule)
		Expect(err).NotTo(HaveOccurred())

		unsafeModificationsMetric = metrics.GetUnsafeModifications()
	})

	Context("UnsupportedHCOModification", func() {
		var (
			expectedLabels = map[string]string{
				"kubernetes_operator_component": "hyperconverged-cluster-operator",
				"kubernetes_operator_part_of":   "kubevirt",
				"operator_health_impact":        "none",
				"severity":                      "info",
			}

			expectedAnnotations = map[string]string{
				"runbook_url": "https://kubevirt.io/monitoring/runbooks/UnsupportedHCOModification",
			}
		)

		It("should alert when there are unsafe modifications", func() {
			unsafeModificationsMetric.WithLabelValues("kubevirt.kubevirt.io/jsonpatch")
			unitTest.AddMetric(unsafeModificationsMetric, "0 0 0 0 1")

			unsafeModificationsMetric.WithLabelValues("ssp.kubevirt.io/jsonpatch")
			unitTest.AddMetric(unsafeModificationsMetric, "0 0 0 0 0")

			expectedLabels["annotation_name"] = "kubevirt.kubevirt.io/jsonpatch"
			expectedAnnotations["summary"] = "1 unsafe modifications were detected in the HyperConverged resource."
			expectedAnnotations["description"] = "unsafe modification for the kubevirt.kubevirt.io/jsonpatch annotation in the HyperConverged resource."

			unitTest.ExpectNoAlert(1*time.Minute, alerts.UnsafeModificationAlert)
			unitTest.ExpectAlert(5*time.Minute, alerts.UnsafeModificationAlert, expectedLabels, expectedAnnotations)

			Expect(unitTest.Run()).To(Succeed())
		})

		It("should not alert when there are no unsafe modifications", func() {
			unitTest.ExpectNoAlert(1*time.Minute, alerts.UnsafeModificationAlert)
			unitTest.ExpectNoAlert(5*time.Minute, alerts.UnsafeModificationAlert)
			unitTest.ExpectNoAlert(10*time.Minute, alerts.UnsafeModificationAlert)

			Expect(unitTest.Run()).To(Succeed())
		})

		It("should not alert when there are 0 unsafe modifications", func() {
			unsafeModificationsMetric.WithLabelValues("kubevirt.kubevirt.io/jsonpatch")
			unitTest.AddMetric(unsafeModificationsMetric, "0 0 0 0 0")

			unitTest.ExpectNoAlert(1*time.Minute, alerts.UnsafeModificationAlert)
			unitTest.ExpectNoAlert(5*time.Minute, alerts.UnsafeModificationAlert)
			unitTest.ExpectNoAlert(10*time.Minute, alerts.UnsafeModificationAlert)

			Expect(unitTest.Run()).To(Succeed())
		})
	})
})

func NewUnitTest(promRule *promv1.PrometheusRule) (*unitTest, error) {
	err := dumpPromSpec(promRule)
	if err != nil {
		return nil, err
	}

	return &unitTest{
		RuleFiles:          []string{"/tmp/rules.verify"},
		EvaluationInterval: model.Duration(1 * time.Minute),
		GroupEvalOrder:     []string{"recordingRules.rules", "alerts.rules"},
		Tests: []testGroup{
			{
				Interval:       model.Duration(1 * time.Minute),
				InputSeries:    []series{},
				AlertRuleTests: []alertTestCase{},
			},
		},
	}, nil
}

func (u *unitTest) AddMetric(metric operatormetrics.Metric, values string) {
	ch := make(chan prometheus.Metric, 1)
	metric.GetCollector().(prometheus.Collector).Collect(ch)
	close(ch)

	for collectedMetric := range ch {
		dtoMetric := &dto.Metric{}
		err := collectedMetric.Write(dtoMetric)
		Expect(err).NotTo(HaveOccurred())

		var mType *dto.MetricType

		switch metric.GetBaseType() {
		case operatormetrics.CounterType:
			mType = dto.MetricType_COUNTER.Enum()
		case operatormetrics.GaugeType:
			mType = dto.MetricType_GAUGE.Enum()
		case operatormetrics.HistogramType:
			mType = dto.MetricType_HISTOGRAM.Enum()
		case operatormetrics.SummaryType:
			mType = dto.MetricType_SUMMARY.Enum()
		}

		metricFamily := &dto.MetricFamily{
			Name:   &[]string{metric.GetOpts().Name}[0],
			Help:   &[]string{metric.GetOpts().Help}[0],
			Type:   mType,
			Metric: []*dto.Metric{dtoMetric},
		}

		var out bytes.Buffer
		encoder := expfmt.NewEncoder(&out, expfmt.NewFormat(expfmt.TypeTextPlain))
		err = encoder.Encode(metricFamily)
		Expect(err).NotTo(HaveOccurred())

		// print 3rd line, 1st column
		lines := strings.Split(out.String(), "\n")
		m := lines[2]

		i := len(m) - 1
		for i >= 0 && m[i*1] != ' ' {
			m = m[:i]
			i--
		}

		u.Tests[0].InputSeries = append(u.Tests[0].InputSeries, series{
			Series: m,
			Values: values,
		})

		switch metric.GetType() {
		case operatormetrics.CounterVecType:
			c := metric.(*operatormetrics.CounterVec)
			c.Reset()
		case operatormetrics.GaugeVecType:
			g := metric.(*operatormetrics.GaugeVec)
			g.Reset()
		case operatormetrics.HistogramVecType:
			h := metric.(*operatormetrics.HistogramVec)
			h.Reset()
		case operatormetrics.SummaryVecType:
			s := metric.(*operatormetrics.SummaryVec)
			s.Reset()
		}

		return
	}
}

func (u *unitTest) AddCustomMetric(serie string, values string) {
	u.Tests[0].InputSeries = append(u.Tests[0].InputSeries, series{
		Series: serie,
		Values: values,
	})
}

func (u *unitTest) ExpectAlert(evalTime time.Duration, name string, labels map[string]string, annotations map[string]string) {
	labels["alertname"] = name

	u.Tests[0].AlertRuleTests = append(u.Tests[0].AlertRuleTests, alertTestCase{
		EvalTime:  model.Duration(evalTime),
		Alertname: name,
		ExpAlerts: []alert{
			{
				ExpLabels:      labels,
				ExpAnnotations: annotations,
			},
		},
	})
}

func (u *unitTest) ExpectNoAlert(evalTime time.Duration, name string) {
	u.Tests[0].AlertRuleTests = append(u.Tests[0].AlertRuleTests, alertTestCase{
		EvalTime:  model.Duration(evalTime),
		Alertname: name,
		ExpAlerts: []alert{},
	})
}

func (u *unitTest) Run() error {
	b, err := yaml.Marshal(u)
	if err != nil {
		return err
	}

	err = os.WriteFile(testFile, b, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"podman", "run", "--rm", "--entrypoint=/bin/promtool",
		"-v", "/tmp/rules.json:/tmp/rules.verify:ro,Z",
		"-v", "/tmp/rules.test:/tmp/rules.test:ro,Z",
		"prom/prometheus:v2.44.0",
		"test", "rules", "/tmp/rules.test",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("promtool error: %v, details: %s", err, stderr.String())
	}

	GinkgoWriter.Println(stdout.String())

	return nil
}

func dumpPromSpec(promRule *promv1.PrometheusRule) error {
	b, err := json.Marshal(promRule.Spec)
	if err != nil {
		return err
	}

	err = os.WriteFile(rulesFile, b, 0644)
	if err != nil {
		return err
	}

	return nil
}

type unitTest struct {
	RuleFiles          []string       `yaml:"rule_files"`
	EvaluationInterval model.Duration `yaml:"evaluation_interval,omitempty"`
	GroupEvalOrder     []string       `yaml:"group_eval_order"`
	Tests              []testGroup    `yaml:"tests"`
}

type testGroup struct {
	Interval       model.Duration  `yaml:"interval"`
	InputSeries    []series        `yaml:"input_series"`
	AlertRuleTests []alertTestCase `yaml:"alert_rule_test,omitempty"`
}

type series struct {
	Series string `yaml:"series"`
	Values string `yaml:"values"`
}

type alertTestCase struct {
	EvalTime  model.Duration `yaml:"eval_time"`
	Alertname string         `yaml:"alertname"`
	ExpAlerts []alert        `yaml:"exp_alerts"`
}

type alert struct {
	ExpLabels      map[string]string `yaml:"exp_labels"`
	ExpAnnotations map[string]string `yaml:"exp_annotations"`
}
