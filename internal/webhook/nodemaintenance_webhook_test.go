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

package webhook_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("NodeMaintenance Webhook", func() {
	var testCtx context.Context

	BeforeEach(func() {
		testCtx = context.Background()
		nodes := testutils.GetTestNodes("node", 1, false)
		Expect(k8sClient.Create(testCtx, nodes[0])).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, nodes[0])).To(Succeed())
		})

	})

	Context("When creating NodeMaintenance under Validating Webhook", func() {
		It("Should deny if Node with spec.NodeName does not exists", func() {
			nm := testutils.GetTestNodeMaintenance("test-nm", "some-node", "test.nvidia.com", "")
			err := k8sClient.Create(testCtx, nm)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("nodes \"some-node\" not found"))

		})

		It("Should admit Node with spec.NodeName exists", func() {
			nm := testutils.GetTestNodeMaintenance("test-nm", "node-0", "test.nvidia.com", "")
			Expect(k8sClient.Create(testCtx, nm)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, nm)).To(Succeed())
			})
		})
	})
})
