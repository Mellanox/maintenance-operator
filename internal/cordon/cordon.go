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
package cordon

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

const (
	// NodeInitialStateUnschedulableAnnot stores the "unschedulable" initial state of the node
	NodeInitialStateUnschedulableAnnot string = "maintenance.nvidia.com/node-initial-state.unschedulable"

	// FalseString is a string representation of "false" boolean value.
	FalseString string = "false"
)

// Handler is an interface to handle cordon/uncordon of nodes
type Handler interface {
	// HandleCordon handles cordon of nodes. in an idempotent manner.
	HandleCordon(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) error
	// HandleUnCordon handles uncordon for node. in an idempotent mannere.
	HandleUnCordon(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) error
}

// NewCordonHandler creates a new cordon Handler
func NewCordonHandler(c client.Client, k kubernetes.Interface) Handler {
	return &cordonHandler{
		k8sclient:    c,
		k8sInterface: k,
	}
}

// cordonHandler implements Handler interface
type cordonHandler struct {
	k8sclient    client.Client
	k8sInterface kubernetes.Interface
}

// HandleCordon handles cordon of nodes
func (c *cordonHandler) HandleCordon(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) error {
	// conditionally set node-initial-state annot
	if !metav1.HasAnnotation(nm.ObjectMeta, NodeInitialStateUnschedulableAnnot) {
		// set annotation
		metav1.SetMetaDataAnnotation(&nm.ObjectMeta, NodeInitialStateUnschedulableAnnot, fmt.Sprintf("%v", node.Spec.Unschedulable))
		err := c.k8sclient.Update(ctx, nm)
		if err != nil {
			return err
		}
	}

	// cordon node if its initial state was not unschedulable and the node is currently schedulable (i.e not cordoned)
	if nm.Annotations[NodeInitialStateUnschedulableAnnot] == FalseString &&
		!node.Spec.Unschedulable {
		helper := &drain.Helper{Ctx: ctx, Client: c.k8sInterface}
		err := drain.RunCordonOrUncordon(helper, node, true)
		if err != nil {
			reqLog.Error(err, "failed to cordon node", "name", node.Name)
			return err
		}
	}

	return nil
}

// HandleUnCordon handles uncordon for node
func (c *cordonHandler) HandleUnCordon(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) error {
	// uncordon node
	if nm.Annotations[NodeInitialStateUnschedulableAnnot] == FalseString &&
		node.Spec.Unschedulable {
		helper := &drain.Helper{Ctx: ctx, Client: c.k8sInterface}
		err := drain.RunCordonOrUncordon(helper, node, false)
		if err != nil {
			reqLog.Error(err, "failed to uncordon node", "name", node.Name)
			return err
		}
	}

	// remove nodeInitialStateUnschedulableAnnot annotation
	if metav1.HasAnnotation(nm.ObjectMeta, NodeInitialStateUnschedulableAnnot) {
		delete(nm.Annotations, NodeInitialStateUnschedulableAnnot)
		if err := c.k8sclient.Update(ctx, nm); err != nil {
			reqLog.Error(err, "failed to update NodeMaintenance annotations")
			return err
		}
	}

	return nil
}
