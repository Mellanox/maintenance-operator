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

package k8sutils_test

import (
	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("k8sutils Tests", func() {
	Context("GetReadyConditionReason", func() {
		nm := &maintenancev1.NodeMaintenance{}
		Expect(k8sutils.GetReadyConditionReason(nm)).To(Equal(maintenancev1.ConditionReasonUninitialized))
		_ = k8sutils.SetReadyConditionReason(nm, maintenancev1.ConditionReasonPending)
		Expect(k8sutils.GetReadyConditionReason(nm)).To(Equal(maintenancev1.ConditionReasonPending))

	})

	Context("SetReadyConditionReason", func() {
		nm := &maintenancev1.NodeMaintenance{}
		changed := k8sutils.SetReadyConditionReason(nm, maintenancev1.ConditionReasonCordon)
		Expect(changed).To(BeTrue())
		Expect(nm.Status.Conditions[0].Type).To(Equal(maintenancev1.ConditionTypeReady))
		Expect(nm.Status.Conditions[0].Reason).To(Equal(maintenancev1.ConditionReasonCordon))
	})

	Context("SetReadyConditionReasonMsg", func() {
		It("Sets condition as expected", func() {
			nm := &maintenancev1.NodeMaintenance{}
			changed := k8sutils.SetReadyConditionReasonMsg(nm, maintenancev1.ConditionReasonCordon, "foobar")
			Expect(changed).To(BeTrue())
			Expect(nm.Status.Conditions[0].Type).To(Equal(maintenancev1.ConditionTypeReady))
			Expect(nm.Status.Conditions[0].Reason).To(Equal(maintenancev1.ConditionReasonCordon))
			Expect(nm.Status.Conditions[0].Message).To(Equal("foobar"))
			changed = k8sutils.SetReadyConditionReasonMsg(nm, maintenancev1.ConditionReasonCordon, "foobar")
			Expect(changed).To(BeFalse())
			Expect(nm.Status.Conditions[0].Type).To(Equal(maintenancev1.ConditionTypeReady))
			Expect(nm.Status.Conditions[0].Reason).To(Equal(maintenancev1.ConditionReasonCordon))
			Expect(nm.Status.Conditions[0].Message).To(Equal("foobar"))
		})
	})

	Context("IsUnderMaintenance", func() {
		nm := &maintenancev1.NodeMaintenance{}
		It("Returns false if not under maintenance", func() {
			By("no condition")
			nm.Status.Conditions = nil
			Expect(k8sutils.IsUnderMaintenance(nm)).To(BeFalse())
			By("ready condition with pending reason")
			nm.Status.Conditions = []metav1.Condition{
				{
					Type:   maintenancev1.ConditionTypeReady,
					Status: metav1.ConditionFalse,
					Reason: maintenancev1.ConditionReasonPending,
				},
			}
			Expect(k8sutils.IsUnderMaintenance(nm)).To(BeFalse())
		})

		It("Returns true if under maintenance", func() {
			nm.Status.Conditions = []metav1.Condition{
				{
					Type:   maintenancev1.ConditionTypeReady,
					Status: metav1.ConditionFalse,
					Reason: maintenancev1.ConditionReasonScheduled,
				},
			}
			Expect(k8sutils.IsUnderMaintenance(nm)).To(BeTrue())
		})
	})

	Context("IsNodeReady", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}

		It("Returns true when node is Ready", func() {
			node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}
			Expect(k8sutils.IsNodeReady(node)).To(BeTrue())
		})

		It("Returns false when node is not Ready", func() {
			node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
			Expect(k8sutils.IsNodeReady(node)).To(BeFalse())
		})
	})

	Context("IsNodeUnschedulable", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}

		It("Returns true when node is unschedulable", func() {
			node.Spec.Unschedulable = true
			Expect(k8sutils.IsNodeUnschedulable(node)).To(BeTrue())
		})

		It("Returns false when node is unschedulable", func() {
			node.Spec.Unschedulable = false
			Expect(k8sutils.IsNodeUnschedulable(node)).To(BeFalse())
		})
	})

})
