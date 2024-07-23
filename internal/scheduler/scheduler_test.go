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

package scheduler_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/scheduler"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("Scheduler tests", func() {
	var clusterState *scheduler.ClusterState
	var schedulerCtx *scheduler.SchedulerContext
	var sched scheduler.Scheduler

	BeforeEach(func() {
		clusterState = nil
		schedulerCtx = nil
		sched = scheduler.NewDefaultScheduler(ctrllog.Log.WithName("scheduler"))
	})

	Context("Single NodeMaintenance request", func() {
		BeforeEach(func() {
			// 5 nodes, no nm in progress, single nm pending
			nodes := testutils.GetTestNodes("node", 5, false)
			nmInProgress := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-in-prog-0", "node-0", "test.nvidia.com", maintenancev1.ConditionReasonScheduled),
			}
			nmPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("test-nm", "node-1", "test.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			candidateNodes := sets.New("node-1")

			schedulerCtx = &scheduler.SchedulerContext{
				AvailableSlots:       1,
				CanBecomeUnavailable: 1,
				CandidateNodes:       candidateNodes,
				CandidateMaintenance: nmPending,
			}
			clusterState = scheduler.NewClusterState(nodes, append(nmInProgress, nmPending...))
		})

		It("Schedules successfully", func() {
			recommend := sched.Schedule(clusterState, schedulerCtx)
			mustHave := sets.New("test-nm")
			validateRecommentation(recommend, 1, schedulerCtx.CandidateNodes, &mustHave)
		})

		It("does not recomment node maintenance for scheduling if AvailableSlots is 0", func() {
			schedulerCtx.AvailableSlots = 0
			res := sched.Schedule(clusterState, schedulerCtx)
			Expect(res).To(HaveLen(0))
		})
	})

	Context("multiple NodeMaintenance - basic", func() {
		BeforeEach(func() {
			// 5 nodes, 2 nm in progress, 5 nm pending from one requestor, 1 nm pending from another requestor
			nodes := testutils.GetTestNodes("node", 5, false)
			nmInProgress := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-in-prog-0", "node-0", "test.nvidia.com", maintenancev1.ConditionReasonScheduled),
				testutils.GetTestNodeMaintenance("nm-in-prog-1", "node-1", "test.nvidia.com", maintenancev1.ConditionReasonScheduled),
			}
			nmPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("test-nm-1", "node-1", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("test-nm-2", "node-2", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("other-nm", "node-2", "other-test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("test-nm-3", "node-3", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("test-nm-4", "node-4", "test.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			nmCandidate := nmPending[2:]
			candidateNodes := sets.New("node-2", "node-3", "node-4")

			schedulerCtx = &scheduler.SchedulerContext{
				AvailableSlots:       2,
				CanBecomeUnavailable: 2,
				CandidateNodes:       candidateNodes,
				CandidateMaintenance: nmCandidate,
			}
			clusterState = scheduler.NewClusterState(nodes, append(nmInProgress, nmPending...))
		})

		It("Schedules successfully without unavailable constrains", func() {
			recommend := sched.Schedule(clusterState, schedulerCtx)
			validateRecommentation(recommend, 2, schedulerCtx.CandidateNodes, nil)
		})

		It("Schedules successfully with unavailable constrains", func() {
			schedulerCtx.CanBecomeUnavailable = 1
			recommend := sched.Schedule(clusterState, schedulerCtx)
			validateRecommentation(recommend, 1, schedulerCtx.CandidateNodes, nil)
		})

		It("Schedules successfully with unavailable constrains with one candidate node already unavailable", func() {
			schedulerCtx.CanBecomeUnavailable = 1
			clusterState.Nodes["node-3"].Spec.Unschedulable = true
			recommend := sched.Schedule(clusterState, schedulerCtx)
			By("expecting to have 2 node maintenance recommended")
			mustHave := sets.New("test-nm-3")
			validateRecommentation(recommend, 2, schedulerCtx.CandidateNodes, &mustHave)
		})
	})
})

func validateRecommentation(recommend []*maintenancev1.NodeMaintenance, expectedLen int, candidateNodes sets.Set[string], mustHaveNm *sets.Set[string]) {
	By("check expected NM len")
	ExpectWithOffset(-1, recommend).To(HaveLen(expectedLen), "recommended NodeMaintenance does not have expected len")

	if expectedLen == 0 {
		return
	}

	nmNodesSet := sets.New[string]()
	nmSet := sets.New[string]()
	for _, nm := range recommend {
		nmNodesSet.Insert(nm.Spec.NodeName)
		nmSet.Insert(nm.Name)
	}

	By("check unique nodes")
	ExpectWithOffset(-1, nmNodesSet).To(HaveLen(expectedLen), "NodeMaintenance recommended dont have unique nodes")

	By("check for candidateNodes subset")
	ExpectWithOffset(-1, candidateNodes.IsSuperset(nmNodesSet)).To(BeTrue(), "recommended NodeMaintenance is not a subset of candidateNodes")

	if mustHaveNm != nil {
		By("checking for must have NodeMaintenance in recommendation")
		ExpectWithOffset(-1, nmSet.IsSuperset(*mustHaveNm)).To(BeTrue(),
			"recommended NodeMaintenance does contain must have NodeMaintenance. current: %v mustHave: %v",
			nmSet.UnsortedList(), mustHaveNm.UnsortedList())
	}
}
