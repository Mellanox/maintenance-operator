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

	"k8s.io/apimachinery/pkg/types"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

const (
	// InProgressRankerWeight is the weight of the InProgressRanker
	InProgressRankerWeight = 1000
	// LeastPendingRankerMaxWeight is the weight of the LeastPendingRanker
	LeastPendingRankerMaxWeight = 100
)

// RankedNodeMaintenance is a wrapper for NodeMaintenance object with a rank
type RankedNodeMaintenance struct {
	maintenancev1.NodeMaintenance
	Rank int
}

// CompareRanked compares between two RankedNodeMaintenance objects
func CompareRanked(a *RankedNodeMaintenance, b *RankedNodeMaintenance) int {
	return cmp.Or(
		cmp.Compare(b.Rank, a.Rank),                           // compare by rank (higher rank first)
		a.CreationTimestamp.Compare(b.CreationTimestamp.Time), // compare by creation time
		cmp.Compare(a.Spec.NodeName, b.Spec.NodeName),         // compare by NodeName
		cmp.Compare(a.Spec.RequestorID, b.Spec.RequestorID),   // compare by RequestorID
		cmp.Compare(types.NamespacedName{Namespace: a.Namespace, Name: a.Name}.String(),
			types.NamespacedName{Namespace: b.Namespace, Name: b.Name}.String()), // compare by Namespaced Name
	)
}

// RankSlice ranks the given maintenances objects using the given Rankers and returns a slice of RankedNodeMaintenance objects
// if no rankers were given, it converts maintenances to RankedNodeMaintenance objects
func RankSlice(maintenances []*maintenancev1.NodeMaintenance, rankers ...Ranker) []*RankedNodeMaintenance {
	// convert to slice of RankedNodeMaintenance
	rankedMaintenances := rankedNodeMaintenances(maintenances)

	for _, r := range rankers {
		for _, rnm := range rankedMaintenances {
			r.Rank(rnm)
		}
	}

	return rankedMaintenances
}

// rankedNodeMaintenances creates a slice of RankedNodeMaintenance objects from a slice of NodeMaintenance objects
func rankedNodeMaintenances(maintenances []*maintenancev1.NodeMaintenance) []*RankedNodeMaintenance {
	rankedMaintenances := make([]*RankedNodeMaintenance, len(maintenances))
	for i, maintenance := range maintenances {
		rankedMaintenances[i] = &RankedNodeMaintenance{
			NodeMaintenance: *maintenance.DeepCopy(),
			Rank:            0,
		}
	}
	return rankedMaintenances
}

// CompactIdenticalNodes compacts the list of RankedNodeMaintenance objects by keeping only the first NodeMaintenance object for each node name
func CompactIdenticalNodes(rankedMaintenances []*RankedNodeMaintenance) []*RankedNodeMaintenance {
	compacted := make([]*RankedNodeMaintenance, 0, len(rankedMaintenances))
	seen := make(map[string]struct{})
	for _, rnm := range rankedMaintenances {
		if _, ok := seen[rnm.Spec.NodeName]; !ok {
			compacted = append(compacted, rnm)
			seen[rnm.Spec.NodeName] = struct{}{}
		}
	}
	return compacted
}

// Ranker is an interface for ranking NodeMaintenance objects
type Ranker interface {
	// Rank	ranks the given RankedNodeMaintenance object
	Rank(rnm *RankedNodeMaintenance)
}

// RankerBuilder is a builder for Ranker objects
type RankerBuilder struct {
	cs      *ClusterState
	rankers []Ranker
}

// NewRankerBuilder creates a new RankerBuilder
func NewRankerBuilder(cs *ClusterState) *RankerBuilder {
	return &RankerBuilder{cs: cs}
}

// Build builds the Ranker objects
func (rb *RankerBuilder) Build() []Ranker {
	return rb.rankers
}

// WithInProgressRanker adds the InProgressRanker to the RankerBuilder
func (rb *RankerBuilder) WithInProgressRanker() *RankerBuilder {
	rb.rankers = append(rb.rankers, NewInProgressRanker(rb.cs))
	return rb
}

// WithLeastPendingRanker adds the LeastPendingRanker to the RankerBuilder
func (rb *RankerBuilder) WithLeastPendingRanker() *RankerBuilder {
	rb.rankers = append(rb.rankers, NewLeastPendingRanker(rb.cs))
	return rb
}

// InProgressRanker ranks NodeMaintenance objects that are currently in progress
type inProgressRanker struct {
	cs *ClusterState
}

// Rank ranks the given RankedNodeMaintenance object
func (r *inProgressRanker) Rank(rnm *RankedNodeMaintenance) {
	for _, nm := range r.cs.MaintenanceInProgress {
		if rnm.Spec.RequestorID == nm.Spec.RequestorID {
			rnm.Rank += InProgressRankerWeight
			break
		}
	}
}

// NewInProgressRanker creates a new InProgressRanker
func NewInProgressRanker(cs *ClusterState) Ranker {
	return &inProgressRanker{cs: cs}
}

// LeastPendingRanker ranks NodeMaintenance objects based on the number of pending NodeMaintenance objects for the same requestor
type leastPendingRanker struct {
	cs *ClusterState
}

// Rank ranks the given RankedNodeMaintenance object
func (r *leastPendingRanker) Rank(rnm *RankedNodeMaintenance) {
	var totalPending int
	for _, nm := range r.cs.MaintenancePending {
		if nm.Spec.RequestorID == rnm.Spec.RequestorID {
			totalPending++
		}
	}
	// cap the rank to LeastPendingRankerWeight
	if totalPending > LeastPendingRankerMaxWeight {
		totalPending = LeastPendingRankerMaxWeight
	}

	rnm.Rank += LeastPendingRankerMaxWeight - totalPending
}

// NewLeastPendingRanker creates a new LeastPendingRanker
func NewLeastPendingRanker(cs *ClusterState) Ranker {
	return &leastPendingRanker{cs: cs}
}
