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

// Package testutils is used for unit-tests
package testutils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
)

// GetTestNodes used to create node objects for testing controllers
func GetTestNodes(nodePrefix string, numOfNodes int, unschedulable bool) []*corev1.Node {
	nodes := make([]*corev1.Node, 0, numOfNodes)
	for i := range numOfNodes {
		n := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-%d", nodePrefix, i)},
			Spec: corev1.NodeSpec{
				Unschedulable: unschedulable,
			},
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// GetTestNodeMaintenance used to create NodeMaintenance object for tests
func GetTestNodeMaintenance(name, nodeName, requestorID, reason string) *maintenancev1.NodeMaintenance {
	nm := &maintenancev1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: maintenancev1.NodeMaintenanceSpec{
			RequestorID: requestorID,
			NodeName:    nodeName,
		},
	}
	// add condition if reason was specified
	// NOTE(if you end up Creating it via k8s API, status must be set separately)
	if reason != "" {
		nm.Status.Conditions = []metav1.Condition{
			{
				Type:   maintenancev1.ConditionTypeReady,
				Status: metav1.ConditionFalse,
				Reason: reason,
			},
		}
	}
	return nm
}

// GetTestPod used to create pod objects for tests
func GetTestPod(name, nodeName string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name:  "foo",
					Image: "bar",
				},
			},
		},
	}
}

// GetReadyConditionReasonForFn returns a function that when called it returns NodeMaintenance
// Ready condition Reason to be used in Tests e.g in Eventually() or Consistently() blocks.
func GetReadyConditionReasonForFn(ctx context.Context, c client.Client, ok client.ObjectKey) func() string {
	return func() string {
		nm := &maintenancev1.NodeMaintenance{}
		err := c.Get(ctx, ok, nm)
		if err == nil {
			return k8sutils.GetReadyConditionReason(nm)
		}
		return ""
	}
}

// EventsForObjFn returns returns a function that when called it returns Event messages for
// NodeMaintenance to be used in Tests e.g in Eventually() or Consistently() blocks.
func EventsForObjFn(ctx context.Context, c client.Client, objUID types.UID) func() []string {
	return func() []string {
		el := &corev1.EventList{}
		err := c.List(ctx, el, client.MatchingFields{"involvedObject.uid": string(objUID)})
		if err == nil && len(el.Items) > 0 {
			eMsgs := make([]string, 0, len(el.Items))
			for _, e := range el.Items {
				eMsgs = append(eMsgs, e.Message)
			}
			return eMsgs
		}
		return nil
	}
}
