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

package k8sutils

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

// GetReadyConditionReason returns NodeMaintenance Ready condition.Reason or empty string if unset
func GetReadyConditionReason(nm *maintenancev1.NodeMaintenance) string {
	cond := meta.FindStatusCondition(nm.Status.Conditions, maintenancev1.ConditionTypeReady)
	if cond == nil {
		return maintenancev1.ConditionReasonUninitialized
	}
	return cond.Reason
}

// SetReadyConditionReason sets or updates Ready condition in nm.Status with reason.
// in addition, it updates status of the object in k8s api if required.
// returns error if occurred.
func SetReadyConditionReason(ctx context.Context, client client.Client, nm *maintenancev1.NodeMaintenance, reason string) error {
	return SetReadyConditionReasonMsg(ctx, client, nm, reason, "")
}

// SetReadyConditionReasonMsg sets or updates Ready condition in nm.Status with reason and msg.
// in addition, it updates status of the object in k8s api if required.
// returns error if occurred.
func SetReadyConditionReasonMsg(ctx context.Context, client client.Client, nm *maintenancev1.NodeMaintenance, reason string, msg string) error {
	status := metav1.ConditionFalse
	if reason == maintenancev1.ConditionReasonReady {
		status = metav1.ConditionTrue
	}

	cond := metav1.Condition{
		Type:               maintenancev1.ConditionTypeReady,
		Status:             status,
		ObservedGeneration: nm.Generation,
		Reason:             reason,
		Message:            msg,
	}

	changed := meta.SetStatusCondition(&nm.Status.Conditions, cond)
	var err error
	if changed {
		err = client.Status().Update(ctx, nm)
	}

	return err
}

// IsUnderMaintenance returns true if NodeMaintenance is currently undergoing maintenance
func IsUnderMaintenance(nm *maintenancev1.NodeMaintenance) bool {
	reason := GetReadyConditionReason(nm)
	switch reason {
	case maintenancev1.ConditionReasonUninitialized, maintenancev1.ConditionReasonPending:
		return false
	}
	return true
}

// IsNodeReady returns true if Node is ready
func IsNodeReady(n *corev1.Node) bool {
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
			return false
		}
	}
	return true
}

// IsNodeUnschedulable returns true if node is Unschedulabel
func IsNodeUnschedulable(n *corev1.Node) bool {
	return n.Spec.Unschedulable
}

// AddFinalizer conditionally adds finalizer from NodeMaintenance
func AddFinalizer(ctx context.Context, k8sClient client.Client, nm *maintenancev1.NodeMaintenance, finalizer string) error {
	instanceFinalizers := nm.GetFinalizers()
	if !slices.Contains(instanceFinalizers, finalizer) {
		nm.SetFinalizers(append(instanceFinalizers, finalizer))
		if err := k8sClient.Update(ctx, nm); err != nil {
			return err
		}
	}
	return nil
}

// RemoveFinalizer conditionally removes finalizer from NodeMaintenance
func RemoveFinalizer(ctx context.Context, k8sClient client.Client, nm *maintenancev1.NodeMaintenance, finalizer string) error {
	instanceFinalizers := nm.GetFinalizers()
	i := slices.Index(instanceFinalizers, finalizer)
	if i >= 0 {
		newFinalizers := slices.Delete(instanceFinalizers, i, i+1)
		nm.SetFinalizers(newFinalizers)
		if err := k8sClient.Update(ctx, nm); err != nil {
			return err
		}
	}
	return nil
}

// SetOwnerRef conditionally sets owner referece of object to owner
func SetOwnerRef(ctx context.Context, k8sClient client.Client, owner metav1.Object, object client.Object) error {
	err := controllerutil.SetOwnerReference(owner, object, k8sClient.Scheme())
	if err != nil {
		return err
	}
	return k8sClient.Update(ctx, object)
}

// HasOwnerRef returns true if object owned by owner
func HasOwnerRef(owner metav1.Object, object metav1.Object) bool {
	for _, o := range object.GetOwnerReferences() {
		if o.UID == owner.GetUID() {
			return true
		}
	}
	return false
}
