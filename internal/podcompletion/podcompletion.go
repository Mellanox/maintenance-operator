/*
Copyright 2024, NVIDIA CORPORATION & AFFILIATES

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package podcompletion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

const (
	WaitForPodCompletionStartAnnot string = "maintenance.nvidia.com/wait-pod-completion-start-time"
)

// ErrPodCompletionTimeout is a custom Error to convey that Pod completion timeout has reached.
var ErrPodCompletionTimeout = errors.New("PodCompletionTimeoutError")

// Handler is an interface to handle waiting for pod completion for NodeMaintenance
type Handler interface {
	// HandlePodCompletion handles waiting for pods to complete, returns list of pods that need to be waited or error if occurred.
	// a special PodCompletionTimeoutError is returned if timeout exceeded.
	HandlePodCompletion(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance) ([]string, error)
}

// NewPodCompletionHandler creates a new WaitPodCompletion Handler
func NewPodCompletionHandler(c client.Client) Handler {
	return &podCompletionHandler{
		k8sclient: c,
	}
}

// podCompletionHandler implements Handler interface
type podCompletionHandler struct {
	k8sclient client.Client
}

// HandlePodCompletion handles pod completion for node
func (p *podCompletionHandler) HandlePodCompletion(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance) ([]string, error) {
	var err error
	var startTime time.Time

	if !metav1.HasAnnotation(nm.ObjectMeta, WaitForPodCompletionStartAnnot) ||
		nm.Annotations[WaitForPodCompletionStartAnnot] == "" {
		// set waitForPodCompletion time annotation
		startTime = time.Now().UTC()
		metav1.SetMetaDataAnnotation(&nm.ObjectMeta, WaitForPodCompletionStartAnnot, startTime.Format(time.RFC3339))
		err = p.k8sclient.Update(ctx, nm)
		if err != nil {
			return nil, err
		}
	} else {
		// take start time from waitForPodCompletion time annotation
		startTime, err = time.Parse(time.RFC3339, nm.Annotations[WaitForPodCompletionStartAnnot])
		if err != nil {
			// if we failed to parse annotation, reset it so we can eventually succeed.
			reqLog.Error(err, "failed to parse annotation. resetting annotation", "key", WaitForPodCompletionStartAnnot, "value", nm.Annotations[WaitForPodCompletionStartAnnot])
			delete(nm.Annotations, WaitForPodCompletionStartAnnot)
			innerErr := p.k8sclient.Update(ctx, nm)
			if innerErr != nil {
				reqLog.Error(innerErr, "failed to reset wait for pod completion annotation", "key", WaitForPodCompletionStartAnnot)
			}
			return nil, err
		}
	}
	reqLog.Info("HandlePodCompletion", "start-time", startTime)

	// check expire time
	if nm.Spec.WaitForPodCompletion.TimeoutSecond > 0 {
		timeNow := time.Now()
		timeExpire := startTime.Add(time.Duration(nm.Spec.WaitForPodCompletion.TimeoutSecond * int32(time.Second)))
		if timeNow.After(timeExpire) {
			reqLog.Error(nil, "HandlePodCompletion timeout reached")
			return nil, ErrPodCompletionTimeout
		}
	}

	// list pods with given selector for given node
	podList := &corev1.PodList{}
	selectorLabels, err := labels.Parse(nm.Spec.WaitForPodCompletion.PodSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Spec.WaitForPodCompletion.PodSelector as label selectors")
	}
	selectorFields := fields.OneTermEqualSelector("spec.nodeName", nm.Spec.NodeName)

	err = p.k8sclient.List(ctx, podList, &client.ListOptions{LabelSelector: selectorLabels, FieldSelector: selectorFields})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods. %w", err)
	}

	waitingPods := make([]string, 0, len(podList.Items))
	for _, p := range podList.Items {
		waitingPods = append(waitingPods, types.NamespacedName{Namespace: p.Namespace, Name: p.Name}.String())
	}

	// update status
	nm.Status.WaitForCompletion = waitingPods
	if err = p.k8sclient.Status().Update(ctx, nm); err != nil {
		return nil, fmt.Errorf("failed to update NodeMaintenance status. %w", err)
	}

	return waitingPods, nil
}
