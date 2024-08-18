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

package drain

import (
	"errors"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

// DrainState is the state of the drain request
type DrainState string

const (
	// DrainStateNotStarted means drain request has not been started
	DrainStateNotStarted DrainState = "NotStarted"
	// DrainStateInProgress means drain request is in progress
	DrainStateInProgress DrainState = "InProgress"
	// DrainStateCanceled means drain request got canceled
	DrainStateCanceled DrainState = "Caneceled"
	// DrainStateSuccess means drain request completed successfully
	DrainStateSuccess DrainState = "Success"
	// DrainStateSuccess means drain request completed with error
	DrainStateError DrainState = "Error"

	// DrainDeleted is drain Deleted string
	DrainDeleted = "Deleted"
	// DrainEvicted is drain Evicted string
	DrainEvicted = "Evicted"
)

// ErrDrainRequestExists error means drain request exists
var ErrDrainRequestExists = errors.New("drain request exists")

// DrainStatus is the status of drain
type DrainStatus struct {
	// State is the state of the DrainRequest
	State DrainState
	// PodsToDelete are the list of namespace/name pods that are pending deletion/eviction
	PodsToDelete []string
}

// DrainRequestSpec is the DrainRequest spec
type DrainRequestSpec struct {
	// NodeName is the name of the node for drain
	NodeName string
	// Spec is the DrainSpec as defined in NodeMaintenance CRD
	Spec maintenancev1.DrainSpec
}

// DrainRequest is an interface to perform drain for a node
//
//go:generate mockery --name DrainRequest
type DrainRequest interface {
	// UID returns DrainRequest UID.
	UID() string
	// StartDrain starts drain. drain will be performed to completion or until canceled.
	// a DrainRequest can StartDrain only once. this is a non-blocking operations.
	StartDrain()
	// CancelDrain cancels a started DrainRequest. Blocks until DrainRequest in canceled.
	CancelDrain()
	// Status returns current DrainStatus of a DrainRequest.
	Status() (DrainStatus, error)
	// State returns current DrainState of a DrainRequest.
	State() DrainState
	// Spec returns the request's DrainRequestSpec.
	Spec() DrainRequestSpec
	// LastError returns the last error that occurred during Drain operation of DrainRequest
	// it returns error if drain request completed with error. returns nil otherwise.
	LastError() error
}

// Manager is an interface to Manage DrainRequests
//
//go:generate mockery --name Manager
type Manager interface {
	// NewDrainRequest creates a new DrainRequest for NodeMaintenance
	NewDrainRequest(*maintenancev1.NodeMaintenance) DrainRequest
	// AddRequest adds req to Manager. returns error if request with same UID already exists. This operation also starts the request.
	AddRequest(req DrainRequest) error
	// GetRequest returns DrainRequest with provided UID or nil if request was not found.
	GetRequest(uid string) DrainRequest
	// RemoveRequest removes request with given UID from manager. The operation also cancels the drain request before removal.
	// Operation is blocking until DrainRequest is canceled and drain operation exits.
	RemoveRequest(uid string)
	// ListRequests lists current DrainRequest in manager.
	ListRequests() []DrainRequest
}
