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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
)

// NewClusterState returns a new ClusterState
func NewClusterState(nodes []*corev1.Node, nodeMaintenances []*maintenancev1.NodeMaintenance) *ClusterState {
	n := make(map[string]*corev1.Node)
	for i := range nodes {
		n[nodes[i].Name] = nodes[i]
	}

	var maintenanceInProgress []*maintenancev1.NodeMaintenance
	var maintenancePending []*maintenancev1.NodeMaintenance
	for i := range nodeMaintenances {
		if k8sutils.IsUnderMaintenance(nodeMaintenances[i]) {
			maintenanceInProgress = append(maintenanceInProgress, nodeMaintenances[i])
			continue
		}

		if k8sutils.GetReadyConditionReason(nodeMaintenances[i]) == maintenancev1.ConditionReasonPending {
			maintenancePending = append(maintenancePending, nodeMaintenances[i])
		}
	}

	return &ClusterState{
		Nodes:                 n,
		MaintenanceInProgress: maintenanceInProgress,
		MaintenancePending:    maintenancePending,
	}
}

// ClusterState represent the state of the cluster for Scheduler
type ClusterState struct {
	// Nodes is a map between node name and k8s Node obj
	Nodes map[string]*corev1.Node
	// MaintenanceInProgress hold all NodeMaintenance objects that are currently in progress
	MaintenanceInProgress []*maintenancev1.NodeMaintenance
	// MaintenancePending holds all NodeMaintenance that are pending to be scheduled
	MaintenancePending []*maintenancev1.NodeMaintenance
}

// SchedulerContext contains pre-scheduling information
type SchedulerContext struct {
	// AvailableSlots is the number of Maintenance slots available
	AvailableSlots int
	// CanBecomeUnavailable is the number of nodes that can become unavailable during scheduling
	CanBecomeUnavailable int
	// CandidateNodes is the Set of nodes that must be considered for maintenance
	CandidateNodes sets.Set[string]
	// CandidateMaintenance is a list of NodeMaintenance that must be considered for scheduling
	CandidateMaintenance []*maintenancev1.NodeMaintenance
}

// Scheduler defines an interface for scheduling NodeMaintenance requests
type Scheduler interface {
	// Schedule returns the next set of NodeMaintenance objects to be scheduled.
	Schedule(clusterState *ClusterState, schedulerCtx *SchedulerContext) []*maintenancev1.NodeMaintenance
}
