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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/scheduler"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("Ranker tests", func() {
	Context("CompactIdenticalNodes tests", func() {
		It("should return empty list for empty input", func() {
			Expect(scheduler.CompactIdenticalNodes(nil)).To(BeEmpty())
		})

		It("should return the same list for unique nodes", func() {
			rnms := scheduler.RankSlice([]*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-1", "node-1", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-2", "node-2", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-3", "node-3", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-4", "node-4", "test.nvidia.com", maintenancev1.ConditionReasonPending),
			})
			Expect(scheduler.CompactIdenticalNodes(rnms)).To(Equal(rnms))
		})

		It("should return a compacted list for identical nodes", func() {
			rnms := scheduler.RankSlice([]*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-1", "node-1", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-2", "node-0", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-3", "node-2", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-4", "node-1", "test.nvidia.com", maintenancev1.ConditionReasonPending),
			})
			expected := scheduler.RankSlice([]*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-1", "node-1", "test.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-3", "node-2", "test.nvidia.com", maintenancev1.ConditionReasonPending),
			})
			Expect(scheduler.CompactIdenticalNodes(rnms)).To(Equal(expected))
		})
	})

	Context("Rank tests", func() {
		It("should rank properly", func() {
			nodes := testutils.GetTestNodes("node", 5, false)
			maintenancesInProgress := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "one.nvidia.com", maintenancev1.ConditionReasonScheduled),
				testutils.GetTestNodeMaintenance("nm-1", "node-1", "two.nvidia.com", maintenancev1.ConditionReasonScheduled),
			}
			maintenancesPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-2", "node-1", "one.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-3", "node-2", "one.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-4", "node-3", "two.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-5", "node-4", "three.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			var allMaintenances []*maintenancev1.NodeMaintenance
			allMaintenances = append(allMaintenances, maintenancesInProgress...)
			allMaintenances = append(allMaintenances, maintenancesPending...)
			cs := scheduler.NewClusterState(nodes, allMaintenances)

			ranked := scheduler.RankSlice(maintenancesPending, scheduler.NewInProgressRanker(cs), scheduler.NewLeastPendingRanker(cs))

			rankedMap := make(map[string]*scheduler.RankedNodeMaintenance)
			for i := range ranked {
				rankedMap[ranked[i].Name] = ranked[i]
			}
			Expect(rankedMap["nm-2"].Rank).To(Equal(scheduler.InProgressRankerWeight + scheduler.LeastPendingRankerMaxWeight - 2))
			Expect(rankedMap["nm-3"].Rank).To(Equal(scheduler.InProgressRankerWeight + scheduler.LeastPendingRankerMaxWeight - 2))
			Expect(rankedMap["nm-4"].Rank).To(Equal(scheduler.InProgressRankerWeight + scheduler.LeastPendingRankerMaxWeight - 1))
			Expect(rankedMap["nm-5"].Rank).To(Equal(scheduler.LeastPendingRankerMaxWeight - 1))
		})
	})

	Context("LeastPendingRanker tests", func() {
		It("several pending maintenances", func() {
			nodes := testutils.GetTestNodes("node", 3, false)
			maintenancesPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "one.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-1", "node-1", "two.nvidia.com", maintenancev1.ConditionReasonPending),
				testutils.GetTestNodeMaintenance("nm-2", "node-2", "one.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			cs := scheduler.NewClusterState(nodes, maintenancesPending)

			ranked := scheduler.RankSlice(maintenancesPending, scheduler.NewLeastPendingRanker(cs))

			Expect(ranked[0].Rank).To(Equal(scheduler.LeastPendingRankerMaxWeight - 2))
			Expect(ranked[1].Rank).To(Equal(scheduler.LeastPendingRankerMaxWeight - 1))
			Expect(ranked[2].Rank).To(Equal(scheduler.LeastPendingRankerMaxWeight - 2))

		})

		It("edgecase: single pending maintenance", func() {
			nodes := testutils.GetTestNodes("node", 3, false)
			maintenancesPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "one.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			cs := scheduler.NewClusterState(nodes, maintenancesPending)

			ranked := scheduler.RankSlice(maintenancesPending, scheduler.NewLeastPendingRanker(cs))

			Expect(ranked[0].Rank).To(Equal(scheduler.LeastPendingRankerMaxWeight - 1))
		})

		It("edgecase: more than LeastPendingRankerMaxWeight pending maintenances", func() {
			var maintenancesPending []*maintenancev1.NodeMaintenance
			for i := 0; i < scheduler.LeastPendingRankerMaxWeight+10; i++ {
				maintenancesPending = append(maintenancesPending, testutils.GetTestNodeMaintenance(
					fmt.Sprintf("nm-%d", i), fmt.Sprintf("node-%d", i), "one.nvidia.com", maintenancev1.ConditionReasonPending))
			}
			nodes := testutils.GetTestNodes("node", 1, false)
			cs := scheduler.NewClusterState(nodes, maintenancesPending)

			ranked := scheduler.RankSlice(maintenancesPending, scheduler.NewLeastPendingRanker(cs))

			for _, r := range ranked {
				Expect(r.Rank).To(Equal(0))
			}
		})
	})

	Context("InProgressRanker tests", func() {
		It("has in progress maintenance", func() {
			nodes := testutils.GetTestNodes("node", 2, false)
			maintenancesPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "one.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			maintenancesInProgress := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-1", "node-1", "one.nvidia.com", maintenancev1.ConditionReasonScheduled),
			}
			var totalMaintenances []*maintenancev1.NodeMaintenance
			totalMaintenances = append(totalMaintenances, maintenancesPending...)
			totalMaintenances = append(totalMaintenances, maintenancesInProgress...)
			cs := scheduler.NewClusterState(nodes, totalMaintenances)

			ranked := scheduler.RankSlice(maintenancesPending, scheduler.NewInProgressRanker(cs))

			Expect(ranked[0].Rank).To(Equal(scheduler.InProgressRankerWeight))
		})

		It("no in progress maintenance", func() {
			nodes := testutils.GetTestNodes("node", 1, false)
			maintenancesPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "one.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			cs := scheduler.NewClusterState(nodes, maintenancesPending)

			ranked := scheduler.RankSlice(maintenancesPending, scheduler.NewInProgressRanker(cs))

			Expect(ranked[0].Rank).To(Equal(0))
		})

		It("no in progress maintenance from same requestor", func() {
			nodes := testutils.GetTestNodes("node", 2, false)
			maintenancesPending := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-0", "node-0", "one.nvidia.com", maintenancev1.ConditionReasonPending),
			}
			maintenancesInProgress := []*maintenancev1.NodeMaintenance{
				testutils.GetTestNodeMaintenance("nm-1", "node-1", "two.nvidia.com", maintenancev1.ConditionReasonScheduled),
			}
			var totalMaintenances []*maintenancev1.NodeMaintenance
			totalMaintenances = append(totalMaintenances, maintenancesPending...)
			totalMaintenances = append(totalMaintenances, maintenancesInProgress...)
			cs := scheduler.NewClusterState(nodes, totalMaintenances)

			ranked := scheduler.RankSlice(maintenancesPending, scheduler.NewInProgressRanker(cs))

			Expect(ranked[0].Rank).To(Equal(0))
		})
	})
})
