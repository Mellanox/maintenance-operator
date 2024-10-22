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

package scheduler

import (
	"slices"

	"github.com/go-logr/logr"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
	"github.com/Mellanox/maintenance-operator/internal/utils"
)

// NewDefaultScheduler creates a new default scheduler
func NewDefaultScheduler(log logr.Logger) *defaultScheduler {
	return &defaultScheduler{log: log}
}

// defaultScheduler is a default implementation of Scheduler Interface
type defaultScheduler struct {
	log logr.Logger
}

// Schedule implements Scheduler interface
func (ds *defaultScheduler) Schedule(clusterState *ClusterState, schedulerCtx *SchedulerContext) []*maintenancev1.NodeMaintenance {
	// rank CandidateMaintenance
	ranked := RankSlice(schedulerCtx.CandidateMaintenance,
		NewRankerBuilder(clusterState).WithInProgressRanker().WithLeastPendingRanker().Build()...)
	// sort
	slices.SortFunc(ranked, CompareRanked)

	for _, r := range ranked {
		ds.log.V(operatorlog.DebugLevel).Info("ranked maintenance", "maintenance", r.CanonicalString(), "rank", r.Rank)
	}

	// compact entries pointing to the same Node, keeping the first (highest priority) NodeMainenance from the list.
	compacted := CompactIdenticalNodes(ranked)

	// Return up to AvailableSlots NodeMaintenance without exceeding NodesCanBecomeUnavailable
	var recommended []*maintenancev1.NodeMaintenance
	canBecomeUnavail := schedulerCtx.CanBecomeUnavailable

	for i := range utils.Min(schedulerCtx.AvailableSlots, len(compacted)) {
		node := clusterState.Nodes[compacted[i].Spec.NodeName]
		nodeAvailable := !k8sutils.IsNodeUnschedulable(node) && k8sutils.IsNodeReady(node)

		if nodeAvailable {
			if canBecomeUnavail == 0 {
				// try next nodeMaintenance
				continue
			}
			canBecomeUnavail--
			recommended = append(recommended, &compacted[i].NodeMaintenance)
		} else {
			// node is already in unavailable state so scheduling nodeMaintenance for it will not affect the total number
			// of unavailable nodes
			recommended = append(recommended, &compacted[i].NodeMaintenance)
		}
	}

	ds.log.Info("Schedule", "recommends", utils.CanonicalStringsFromListP(recommended))
	return recommended
}
