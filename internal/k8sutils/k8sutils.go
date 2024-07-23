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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
// returns true if conditions were updated in nm object or false otherwise.
func SetReadyConditionReason(nm *maintenancev1.NodeMaintenance, reason string) (changed bool) {
	return SetReadyConditionReasonMsg(nm, reason, "")
}

// SetReadyConditionReasonMsg sets or updates Ready condition in nm.Status with reason and msg.
// returns true if conditions were updated in nm object or false otherwise.
func SetReadyConditionReasonMsg(nm *maintenancev1.NodeMaintenance, reason string, msg string) (changed bool) {
	status := metav1.ConditionFalse
	if reason == maintenancev1.ConditionReasonReady {
		status = metav1.ConditionTrue
	}

	cond := metav1.Condition{
		Type:    maintenancev1.ConditionTypeReady,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}
	return meta.SetStatusCondition(&nm.Status.Conditions, cond)
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
