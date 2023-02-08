package nodeconfig

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/metrics"
)

type kubeletConfig struct {
	logger    logr.Logger
	apiReader client.Reader
	client    rest.Interface

	KubeletConfig struct {
		NodeStatusMaxImages int `json:"nodeStatusMaxImages"`
	} `json:"kubeletconfig"`
}

func newKubeletConfig(logger logr.Logger, apiReader client.Reader, client rest.Interface) *kubeletConfig {
	return &kubeletConfig{
		logger:    logger,
		apiReader: apiReader,
		client:    client,
	}
}

func (kc *kubeletConfig) updateNodeImageMetrics(ctx context.Context) {
	nodeList := corev1.NodeList{}
	err := kc.apiReader.List(ctx, &nodeList)
	if err != nil {
		kc.logger.Error(err, "Failed to list nodes")
		return
	}

	for _, node := range nodeList.Items {
		err = metrics.HcoMetrics.SetHCOMetricNumberOfImages(node.Name, len(node.Status.Images))
		if err != nil {
			kc.logger.Error(err, "Failed to set number of images metric")
			continue
		}

		kc.setNodeMaxImagesMetrics(ctx, node.Name)
	}
}

func (kc *kubeletConfig) setNodeMaxImagesMetrics(ctx context.Context, nodeName string) {
	resp, err := kc.client.Get().
		Resource("nodes").Name(nodeName).
		Suffix("proxy", "configz").
		Do(ctx).Raw()
	if err != nil {
		kc.logger.Error(err, "Failed to get node configz")
		return
	}

	err = json.Unmarshal(resp, &kc)
	if err != nil {
		kc.logger.Error(err, "Failed to unmarshal kubelet configz")
		return
	}

	err = metrics.HcoMetrics.SetHCOMetricNodeMaxImages(nodeName, kc.KubeletConfig.NodeStatusMaxImages)
	if err != nil {
		kc.logger.Error(err, "Failed to set node max images metric")
		return
	}
}
