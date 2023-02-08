package nodeconfig

import (
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorhandler "github.com/operator-framework/operator-lib/handler"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/version"
)

var log = logf.Log.WithName("controller_nodeconfig")

type ReconcileNodeConfig struct {
	apiReader  client.Reader
	client     rest.Interface
	ownVersion string
}

func RegisterReconciler(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return add(mgr, r)
}

func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	ownVersion := os.Getenv(hcoutil.HcoKvIoVersionName)
	if ownVersion == "" {
		ownVersion = version.Version
	}

	node := corev1.Node{}
	restClient, err := hcoutil.GetRESTClientFor(&node, mgr.GetConfig(), mgr.GetScheme())
	if err != nil {
		return nil, err
	}

	r := &ReconcileNodeConfig{
		apiReader:  mgr.GetAPIReader(),
		client:     restClient,
		ownVersion: ownVersion,
	}

	return r, nil
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New("nodeconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Node{}},
		&operatorhandler.InstrumentedEnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (r ReconcileNodeConfig) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	kc := newKubeletConfig(logger, r.apiReader, r.client)
	kc.updateNodeImageMetrics(ctx)

	return reconcile.Result{}, nil
}
