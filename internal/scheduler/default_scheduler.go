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
	"cmp"
	"slices"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
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
	// Naive Implementation to get things working

	// sort NodeMaintenance by NodeName and RequestorID
	slices.SortFunc(schedulerCtx.CandidateMaintenance, func(a *maintenancev1.NodeMaintenance, b *maintenancev1.NodeMaintenance) int {
		if a.Spec.NodeName != b.Spec.NodeName {
			// different nodes, sort by NodeName
			return cmp.Compare(a.Spec.NodeName, b.Spec.NodeName)
		}

		if a.Spec.RequestorID != b.Spec.RequestorID {
			// different requestors, sort by requestorID
			return cmp.Compare(a.Spec.RequestorID, b.Spec.RequestorID)
		}

		// same node and requestor, compare by Namespaced Name
		return cmp.Compare(types.NamespacedName{Namespace: a.Namespace, Name: a.Name}.String(),
			types.NamespacedName{Namespace: b.Namespace, Name: b.Name}.String())
	})

	// compact entries pointing to the same Node, keeping the first (highest priority) NodeMainenance from the list.
	// this is done as we can only recommend a single NodeMaintenance per node for scheduling.
	// NOTE(adrianc): for this to work we rely on the fact that entries with same node are adjacent (assured by the sort function above)
	compacted := slices.CompactFunc(schedulerCtx.CandidateMaintenance, func(a *maintenancev1.NodeMaintenance, b *maintenancev1.NodeMaintenance) bool {
		return a.Spec.NodeName == b.Spec.NodeName
	})

	// Return up to AvailableSlots NodeMaintenance without exceeding NodesCanBecomeUnavailable
	var recommended []*maintenancev1.NodeMaintenance
	canBecomeUnavail := schedulerCtx.CanBecomeUnavailable

	for i := range utils.MinInt(schedulerCtx.AvailableSlots, len(compacted)) {
		node := clusterState.Nodes[compacted[i].Spec.NodeName]
		nodeAvailable := !k8sutils.IsNodeUnschedulable(node) && k8sutils.IsNodeReady(node)

		if nodeAvailable {
			if canBecomeUnavail == 0 {
				// try next nodeMaintenance
				continue
			}
			canBecomeUnavail--
			recommended = append(recommended, compacted[i])
		} else {
			// node is already in unavailable state so scheduling nodeMaintenance for it will not affect the total number
			// of unavailable nodes
			recommended = append(recommended, compacted[i])
		}
	}

	ds.log.Info("Schedule", "recommends", utils.CanonicalStringsFromListP(recommended))
	return recommended
}
