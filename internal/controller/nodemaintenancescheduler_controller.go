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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

const (
	schedulerSyncEventName = "node-maintenance-scheduler-sync-event"
	schedulerSyncTime      = 10 * time.Second
)

// NodeMaintenanceSchedulerReconciler reconciles a NodeMaintenance object
type NodeMaintenanceSchedulerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeMaintenance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *NodeMaintenanceSchedulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	reqLog.Info("got request", "name", req.NamespacedName)

	return ctrl.Result{RequeueAfter: schedulerSyncTime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeMaintenanceSchedulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	qHandler := func(q workqueue.RateLimitingInterface) {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      schedulerSyncEventName,
		}})
	}

	eventHandler := handler.Funcs{
		GenericFunc: func(ctx context.Context, e event.GenericEvent, q workqueue.RateLimitingInterface) {
			log.Log.WithName("NodeMaintenanceScheduler").
				Info("Enqueuing sync for generic event", "resource", e.Object.GetName())
			qHandler(q)
		},
	}

	// send initial sync event to trigger reconcile when controller is started
	eventChan := make(chan event.GenericEvent, 1)
	eventChan <- event.GenericEvent{Object: &maintenancev1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{Name: schedulerSyncEventName, Namespace: ""},
	}}
	close(eventChan)

	return ctrl.NewControllerManagedBy(mgr).
		Named("nodemaintenancescheduler").
		WatchesRawSource(source.Channel(eventChan, eventHandler)).
		Complete(r)
}
