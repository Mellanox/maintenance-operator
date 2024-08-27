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

package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/log"
)

var (
	defaultMaxNodeMaintenanceTime         = 1600 * time.Second
	defaultGarbageCollectionReconcileTime = 5 * time.Minute
	garbageCollectionReconcileTime        = defaultGarbageCollectionReconcileTime
)

// GarbageCollectIgnoreAnnotation garbage collector will skip NodeMaintenance with this annotation.
const GarbageCollectIgnoreAnnotation = "maintenance.nvidia.com/garbage-collector.ignore"

// NewGarbageCollectorOptions creates new *GarbageCollectorOptions
func NewGarbageCollectorOptions() *GarbageCollectorOptions {
	return &GarbageCollectorOptions{
		pendingMaxNodeMaintenanceTime: defaultMaxNodeMaintenanceTime,
		maxNodeMaintenanceTime:        defaultMaxNodeMaintenanceTime,
	}
}

// GarbageCollectorOptions are options for GarbageCollector where values
// are stored by external entity and read by GarbageCollector.
type GarbageCollectorOptions struct {
	sync.Mutex

	pendingMaxNodeMaintenanceTime time.Duration
	maxNodeMaintenanceTime        time.Duration
}

// Store maxNodeMaintenanceTime
func (gco *GarbageCollectorOptions) Store(maxNodeMaintenanceTime time.Duration) {
	gco.Lock()
	defer gco.Unlock()

	gco.pendingMaxNodeMaintenanceTime = maxNodeMaintenanceTime
}

// Load loads the last Stored options
func (gco *GarbageCollectorOptions) Load() {
	gco.Lock()
	defer gco.Unlock()

	gco.maxNodeMaintenanceTime = gco.pendingMaxNodeMaintenanceTime
}

// MaxNodeMaintenanceTime returns the last loaded MaxUnavailable option
func (gco *GarbageCollectorOptions) MaxNodeMaintenanceTime() time.Duration {
	return gco.maxNodeMaintenanceTime
}

// NewNodeMaintenanceGarbageCollector creates a new NodeMaintenanceGarbageCollector
func NewNodeMaintenanceGarbageCollector(kClient client.Client, options *GarbageCollectorOptions, log logr.Logger) *NodeMaintenanceGarbageCollector {
	return &NodeMaintenanceGarbageCollector{
		Client:  kClient,
		options: options,
		log:     log,
	}
}

// NodeMaintenanceGarbageCollector performs garbage collection for NodeMaintennace
type NodeMaintenanceGarbageCollector struct {
	client.Client

	options *GarbageCollectorOptions
	log     logr.Logger
}

// SetupWithManager sets up NodeMaintenanceGarbageCollector with controller manager
func (r *NodeMaintenanceGarbageCollector) SetupWithManager(mgr ctrl.Manager) error {
	return mgr.Add(r)
}

// Reconcile collects garabage once
func (r *NodeMaintenanceGarbageCollector) Reconcile(ctx context.Context) error {
	r.log.Info("periodic reconcile start")
	r.options.Load()
	r.log.V(log.DebugLevel).Info("loaded options", "maxNodeMaintenanceTime", r.options.MaxNodeMaintenanceTime())

	mnl := &maintenancev1.NodeMaintenanceList{}
	err := r.List(ctx, mnl)
	if err != nil {
		return errors.Wrap(err, "failed to list NodeMaintenance")
	}

	timeNow := time.Now()
	for _, nm := range mnl.Items {
		// skip NodeMaintenance with
		nmLog := r.log.WithValues("namespace", nm.Namespace, "name", nm.Name)

		if nm.Annotations[GarbageCollectIgnoreAnnotation] == "true" {
			nmLog.Info("skipping NodeMaintenance due to ignore annotation")
			continue
		}

		if nm.Annotations[ReadyTimeAnnotation] != "" {
			readyTime, err := time.Parse(time.RFC3339, nm.Annotations[ReadyTimeAnnotation])
			if err != nil {
				nmLog.Error(err, "failed to parse ready-time annotation for NodeMaintenenace")
				continue
			}
			if timeNow.After(readyTime.Add(r.options.MaxNodeMaintenanceTime())) {
				nmLog.Info("NodeMaintenance is due for garbage collection")
				if nm.GetDeletionTimestamp().IsZero() {
					nmLog.Info("deleting NodeMaintenance")
					if err = r.Delete(ctx, &nm); err != nil {
						nmLog.Error(err, "failed to delete NodeMaintenance")
					}
				} else {
					r.log.V(log.DebugLevel).Info("deletion timestamp already set for NodeMaintenance")
				}
			}
		}
	}

	r.log.Info("periodic reconcile end")
	return nil
}

// Start NodeMaintenanceGarbageCollector
func (r *NodeMaintenanceGarbageCollector) Start(ctx context.Context) error {
	r.log.Info("NodeMaintenanceGarbageCollector Start")

	t := time.NewTicker(garbageCollectionReconcileTime)
	defer t.Stop()

OUTER:
	for {
		select {
		case <-ctx.Done():
			break OUTER
		case <-t.C:
			err := r.Reconcile(ctx)
			if err != nil {
				r.log.Error(err, "failed to run reconcile")
			}
		}
	}

	return nil
}
