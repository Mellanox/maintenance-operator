/*
 Copyright 2025, NVIDIA CORPORATION & AFFILIATES

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

package openshift_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mcv1 "github.com/openshift/api/machineconfiguration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/openshift"
)

const (
	testNodeName = "test-node"
	testMCPName  = "test-mcp"
	testMCName   = "test-mc"
	testNMame    = "test-nm"
)

var _ = Describe("mcp manager tests", func() {
	Context("mcp no-op tests", func() {
		It("should return correct values", func() {
			testCtx := context.TODO()
			noOpMcpManager := openshift.NewNoOpMcpManager()
			Expect(noOpMcpManager.PauseMCP(testCtx, nil, nil)).To(Succeed())
			Expect(noOpMcpManager.UnpauseMCP(testCtx, nil, nil)).To(Succeed())
		})
	})

	Context("mcp manager tests", func() {
		var testClient client.Client
		var testCtx context.Context
		var mcpManager openshift.MCPManager
		var node *corev1.Node
		var nm *maintenancev1.NodeMaintenance
		var mcp *mcv1.MachineConfigPool
		var mc *mcv1.MachineConfig

		BeforeEach(func() {
			testCtx = context.TODO()
			s := runtime.NewScheme()
			mcv1.Install(s)
			corev1.AddToScheme(s)
			maintenancev1.AddToScheme(s)

			node = getTestNode()
			nm = getTestNM()
			mcp = getTestMCP()
			mc = getTestMC()
			testClient = fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(node, mc, mcp, nm).
				WithStatusSubresource(&mcv1.MachineConfigPool{}).
				Build()
			mcpManager = openshift.NewOpenshiftMcpManager(testClient)
		})

		Context("PauseMCP", func() {
			It("Succeeds if not paused by the operator", func() {
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).To(Succeed())

				// validate mcp
				Expect(testClient.Get(testCtx, types.NamespacedName{Name: testMCPName}, mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeTrue())
				Expect(mcp.Annotations[openshift.McpPausedAnnotKey]).To(Equal(openshift.McpPausedAnnotValue))

				// validate node maintenance
				Expect(testClient.Get(testCtx, types.NamespacedName{Name: testNMame}, nm)).To(Succeed())
				Expect(nm.Labels[openshift.McpNameLabelKey]).To(Equal(testMCPName))
			})

			It("Succeed if already paused by the operator", func() {
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).To(Succeed())
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).To(Succeed())
			})

			It("Fails if paused by someone else", func() {
				mcp.Spec.Paused = true
				Expect(testClient.Update(testCtx, mcp)).To(Succeed())
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).To(MatchError(openshift.ErrMachineConfigBusy))

				// validate mcp
				Expect(testClient.Get(testCtx, types.NamespacedName{Name: testMCPName}, mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeTrue())
				Expect(mcp.Annotations).ToNot(HaveKey(openshift.McpPausedAnnotKey))

				// validate node maintenance
				Expect(testClient.Get(testCtx, types.NamespacedName{Name: testNMame}, nm)).To(Succeed())
				Expect(nm.Labels).To(HaveKey(openshift.McpNameLabelKey))
			})

			It("Fails if MCP is under configuration", func() {
				mcp.Status.Configuration.Name = "other-config"
				Expect(testClient.Status().Update(testCtx, mcp)).To(Succeed())
				err := mcpManager.PauseMCP(testCtx, node, nm)
				Expect(err).To(MatchError(openshift.ErrMachineConfigBusy))

				// validate mcp
				Expect(testClient.Get(testCtx, types.NamespacedName{Name: testMCPName}, mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeFalse())
				Expect(mcp.Annotations).ToNot(HaveKey(openshift.McpPausedAnnotKey))

				// validate node maintenance
				Expect(testClient.Get(testCtx, types.NamespacedName{Name: testNMame}, nm)).To(Succeed())
				Expect(nm.Labels).To(HaveKey(openshift.McpNameLabelKey))
			})

			It("Fails if MCP annotation on node is not present", func() {
				node.Annotations = map[string]string{}
				Expect(testClient.Update(testCtx, node)).To(Succeed())
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).ToNot(Succeed())
			})

			It("Fails if failed to get MCP name", func() {
				mc.OwnerReferences = nil
				Expect(testClient.Update(testCtx, mc)).To(Succeed())
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).ToNot(Succeed())
				Expect(testClient.Delete(testCtx, mc)).To(Succeed())
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).ToNot(Succeed())
			})

			It("fails if failed to get MCP", func() {
				Expect(testClient.Delete(testCtx, mcp)).To(Succeed())
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).ToNot(Succeed())
			})
		})

		Context("UnpauseMCP", func() {
			It("Succeeds if paused by the operator", func() {
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).To(Succeed())
				Expect(testClient.Get(testCtx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeTrue())
				Expect(mcpManager.UnpauseMCP(testCtx, node, nm)).To(Succeed())
				Expect(testClient.Get(testCtx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeFalse())
				Expect(mcp.Annotations).ToNot(HaveKey(openshift.McpPausedAnnotKey))
			})

			It("Succeeds if not paused by the operator, mcp remains paused", func() {
				mcp.Spec.Paused = true
				Expect(testClient.Update(testCtx, mcp)).To(Succeed())
				Expect(mcpManager.UnpauseMCP(testCtx, node, nm)).To(Succeed())

				Expect(testClient.Get(testCtx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeTrue())
			})

			It("Succeeds - multiple nodeMaintenance same pool", func() {
				// create another node that belongs to same pool and another node maintenance referenccing the new node
				nm2 := getTestNM()
				nm2.Name = "test-nm2"
				nm2.Spec.NodeName = "test-node2"
				Expect(testClient.Create(testCtx, nm2)).To(Succeed())

				node2 := getTestNode()
				node2.Name = "test-node2"
				Expect(testClient.Create(testCtx, node2)).To(Succeed())

				// call pause on both nodes/nm (pausing the same machine config pool)
				Expect(mcpManager.PauseMCP(testCtx, node, nm)).To(Succeed())
				Expect(mcpManager.PauseMCP(testCtx, node2, nm2)).To(Succeed())

				// unpause one of the nodes
				Expect(mcpManager.UnpauseMCP(testCtx, node, nm)).To(Succeed())
				// should not affect MCP
				Expect(testClient.Get(testCtx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeTrue())
				Expect(mcp.Annotations[openshift.McpPausedAnnotKey]).To(Equal(openshift.McpPausedAnnotValue))

				// delete the first node maintenance and call unpause with the other
				Expect(testClient.Delete(testCtx, nm)).To(Succeed())
				Expect(mcpManager.UnpauseMCP(testCtx, node, nm2)).To(Succeed())
				// should unpause machine config pool as its the last node maintenance referencing it
				Expect(testClient.Get(testCtx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
				Expect(mcp.Spec.Paused).To(BeFalse())
				Expect(mcp.Annotations).ToNot(HaveKey(openshift.McpPausedAnnotKey))
			})

			It("fails if failed to get MCP name", func() {
				mc.OwnerReferences = nil
				Expect(testClient.Update(testCtx, mc)).To(Succeed())
				Expect(mcpManager.UnpauseMCP(testCtx, node, nm)).ToNot(Succeed())
				Expect(testClient.Delete(testCtx, mc)).To(Succeed())
				Expect(mcpManager.UnpauseMCP(testCtx, node, nm)).ToNot(Succeed())
			})

			It("fails if failed to get MCP", func() {
				Expect(testClient.Delete(testCtx, mcp)).ToNot(HaveOccurred())
				Expect(mcpManager.UnpauseMCP(testCtx, node, nm)).ToNot(Succeed())
			})
		})
	})
})

func getTestNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
			Annotations: map[string]string{
				openshift.DesiredMachineConfigAnnotationKey: testMCName,
			},
			Labels: map[string]string{
				"mcp-pool": testMCName,
			},
		},
	}
}

func getTestMCP() *mcv1.MachineConfigPool {
	return &mcv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: testMCPName,
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
		Spec: mcv1.MachineConfigPoolSpec{
			Paused: false,
			Configuration: mcv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{Name: testMCName},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"mcp-pool": testMCName},
			},
		},
		Status: mcv1.MachineConfigPoolStatus{
			Configuration: mcv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{Name: testMCName},
			},
		},
	}
}

func getTestMC() *mcv1.MachineConfig {
	return &mcv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testMCName,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "MachineConfigPool", Name: testMCPName},
			},
		},
		Spec: mcv1.MachineConfigSpec{},
	}
}

func getTestNM() *maintenancev1.NodeMaintenance {
	return &maintenancev1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNMame,
		},
		Spec: maintenancev1.NodeMaintenanceSpec{
			RequestorID: "foo.bar.baz",
			NodeName:    testNodeName,
		},
	}
}
