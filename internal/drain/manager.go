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
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

// NewManager creates a new Manager
func NewManager(log logr.Logger, ctx context.Context, kInterface kubernetes.Interface) Manager {
	return &managerImpl{
		mu:            sync.RWMutex{},
		drainRequests: make(map[string]DrainRequest),
		log:           log,
		ctx:           ctx,
		k8sInterface:  kInterface,
	}
}

// managerImpl implements Manager interface.
//
//nolint:containedctx
type managerImpl struct {
	mu            sync.RWMutex
	drainRequests map[string]DrainRequest
	log           logr.Logger
	ctx           context.Context
	k8sInterface  kubernetes.Interface
}

// NewDrainRequest implements Manager interface.
func (m *managerImpl) NewDrainRequest(nm *maintenancev1.NodeMaintenance) DrainRequest {
	uid := DrainRequestUIDFromNodeMaintenance(nm)
	return NewDrainRequest(m.ctx, m.log.WithName(fmt.Sprintf("DrainRequest-%s", uid)), m.k8sInterface, nm, false)
}

// AddRequest implements Manager interface.
func (m *managerImpl) AddRequest(req DrainRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.drainRequests[req.UID()]; exists {
		return ErrDrainRequestExists
	}
	m.drainRequests[req.UID()] = req
	req.StartDrain()
	return nil
}

// GetRequest implements Manager interface.
func (m *managerImpl) GetRequest(uid string) DrainRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, exists := m.drainRequests[uid]
	if !exists {
		return nil
	}
	return req
}

// RemoveRequest implements Manager interface.
func (m *managerImpl) RemoveRequest(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.drainRequests[uid]
	if exists {
		req.CancelDrain()
		delete(m.drainRequests, uid)
	}
}

// ListRequests implements Manager interface.
func (m *managerImpl) ListRequests() []DrainRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reqs := make([]DrainRequest, 0, len(m.drainRequests))
	for i := range m.drainRequests {
		reqs = append(reqs, m.drainRequests[i])
	}
	return reqs
}
