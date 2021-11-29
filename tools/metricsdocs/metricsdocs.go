package main

import (
	"bufio"
	"fmt"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/metrics"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
)

// constant parts of the file
const (
	title      = "# Hyperconverged Cluster Operator metrics\n"
	background = "This document aims to help users that are not familiar with all metrics exposed by different KubeVirt components.\n" +
		"All metrics documented here are auto-generated by the utility tool `tools/metricsdocs` and reflects exactly what is being exposed.\n\n"

	KVSpecificMetrics = "## Hyperconverged Cluster Operator Metrics List\n"

	opening = title +
		background +
		KVSpecificMetrics

	// footer
	footerHeading = "## Developing new metrics\n"
	footerContent = "After developing new metrics or changing old ones, please run `make generate-doc` to regenerate this document.\n\n" +
		"If you feel that the new metric doesn't follow these rules, please change `tools/metricsdocs` with your needs.\n"

	footer = footerHeading + footerContent
)

func main() {
	handler := metrics.Handler(1)
	RegisterFakeCollector()

	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
	checkError(err)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	var metricsList metricList
	if status := recorder.Code; status == http.StatusOK {
		err := parseVirtMetrics(recorder.Body, &metricsList)
		checkError(err)

	} else {
		panic(fmt.Errorf("got HTTP status code of %d from /metrics", recorder.Code))
	}
	writeToFile(metricsList)
}

func writeToFile(metricsList metricList) {
	fmt.Print(opening)
	metricsList.writeOut()
	fmt.Print(footer)
}

type metric struct {
	name        string
	description string
}

func (m metric) writeOut() {
	fmt.Println("###", m.name)
	fmt.Println(m.description)
}

type metricList []metric

// Len implements sort.Interface.Len
func (m metricList) Len() int {
	return len(m)
}

// Less implements sort.Interface.Less
func (m metricList) Less(i, j int) bool {
	return m[i].name < m[j].name
}

// Swap implements sort.Interface.Swap
func (m metricList) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m *metricList) add(line string) {
	split := strings.Split(line, " ")
	name := split[2]
	split[3] = strings.Title(split[3])
	description := strings.Join(split[3:], " ")
	*m = append(*m, metric{name: name, description: description})
}

func (m metricList) writeOut() {
	for _, met := range m {
		met.writeOut()
	}
}

const filter = "kubevirt_hco_"

func parseVirtMetrics(r io.Reader, metricsList *metricList) error {
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		helpLine := scan.Text()
		if strings.HasPrefix(helpLine, "# HELP ") {
			if strings.Contains(helpLine, filter) {
				metricsList.add(helpLine)
			}
		}
	}

	if scan.Err() != nil {
		return fmt.Errorf("failed to parse metrics from prometheus endpoint, %w", scan.Err())
	}

	sort.Sort(metricsList)

	return nil
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}
