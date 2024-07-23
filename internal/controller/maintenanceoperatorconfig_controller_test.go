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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

var _ = Describe("MaintenanceOperatorConfig Controller", func() {
	// test context, TODO(adrianc): use ginkgo spec context
	var testCtx context.Context

	BeforeEach(func() {
		testCtx = context.Background()
	})

	It("Should Reconcile MaintenanceOperatorConfig resource", func() {
		oc := &maintenancev1.MaintenanceOperatorConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"},
			Spec:       maintenancev1.MaintenanceOperatorConfigSpec{},
		}
		Expect(k8sClient.Create(testCtx, oc)).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, oc)).ToNot(HaveOccurred())
		})
	})
})
