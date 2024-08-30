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

package webhook

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

// NewNodeMaintenanceWebhook creates a new NodeMaintenanceWebhook
func NewNodeMaintenanceWebhook(kClient client.Client) *NodeMaintenanceWebhook {
	return &NodeMaintenanceWebhook{
		Client: kClient,
	}
}

// NodeMaintenanceWebhook is webhook for NodeMaintenance
type NodeMaintenanceWebhook struct {
	client.Client
}

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *NodeMaintenanceWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&maintenancev1.NodeMaintenance{}).
		WithValidator(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-maintenance-nvidia-com-v1alpha1-nodemaintenance,mutating=false,failurePolicy=fail,sideEffects=None,groups=maintenance.nvidia.com,resources=nodemaintenances,verbs=create,versions=v1alpha1,name=vnodemaintenance.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &NodeMaintenanceWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *NodeMaintenanceWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("validate create")

	nm, ok := obj.(*maintenancev1.NodeMaintenance)
	if !ok {
		log.Error(nil, "failed to convert runtime object to NodeMaintenance")
		return nil, nil
	}

	// Validate Node exists for NodeMaintenance
	node := &corev1.Node{}
	err = r.Get(ctx, types.NamespacedName{Name: nm.Spec.NodeName}, node)
	if err != nil {
		log.Error(err, "failed to get node from spec", "name", nm.Spec.NodeName)
		return nil, fmt.Errorf("invalid spec.nodeName, failed to get node %s for NodeMaintenance. %s", nm.Spec.NodeName, err)
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *NodeMaintenanceWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	// no validation for update
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *NodeMaintenanceWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	// no validation for update
	return nil, nil
}
