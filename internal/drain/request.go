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
	"io"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/log"
)

// DrainRequestUIDFromNodeMaintenance returns DrainRequest UID of given NodeMaintenance object.
func DrainRequestUIDFromNodeMaintenance(nm *maintenancev1.NodeMaintenance) string {
	return fmt.Sprintf("%s/%s:%s", nm.Namespace, nm.Name, nm.UID)
}

// NewDrainRequest creates a new DrainRequest.
func NewDrainRequest(ctx context.Context, log logr.Logger, k8sInterface kubernetes.Interface, nm *maintenancev1.NodeMaintenance, disableEviction bool) DrainRequest {
	newCtx, cfunc := context.WithCancel(ctx)
	uid := DrainRequestUIDFromNodeMaintenance(nm)
	dri := &drainRequestImpl{
		mu:           sync.Mutex{},
		log:          log,
		ctx:          newCtx,
		cfunc:        cfunc,
		uid:          uid,
		wg:           sync.WaitGroup{},
		k8sInterface: k8sInterface,
		drainSpec: DrainRequestSpec{
			NodeName: nm.Spec.NodeName,
			Spec:     *nm.Spec.DrainSpec.DeepCopy(),
		},
	}
	dri.drainHelper = dri.createDrainHelper()
	dri.drainHelper.DisableEviction = disableEviction
	return dri
}

// drainRequestImpl implements DrainRequest interface.
//
//nolint:containedctx
type drainRequestImpl struct {
	mu           sync.Mutex
	log          logr.Logger
	ctx          context.Context
	cfunc        context.CancelFunc
	uid          string
	started      bool
	finished     bool
	canceled     bool
	lastErr      error
	wg           sync.WaitGroup
	drainSpec    DrainRequestSpec
	k8sInterface kubernetes.Interface
	drainHelper  *drain.Helper
}

// UID implements DrainRequest interface
func (dr *drainRequestImpl) UID() string {
	return dr.uid
}

// StartDrain implements DrainRequest interface
func (dr *drainRequestImpl) StartDrain() {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	if dr.canceled || dr.started {
		return
	}
	dr.started = true

	dr.log.V(log.DebugLevel).Info("Starting Drain")
	dr.wg.Add(1)
	go func() {
		defer dr.wg.Done()
		dr.doDrain()
	}()
}

// CancelDrain implements DrainRequest interface
func (dr *drainRequestImpl) CancelDrain() {
	dr.mu.Lock()
	dr.canceled = true
	dr.mu.Unlock()

	dr.cfunc()
	dr.wg.Wait()
}

// Status implements DrainRequest interface
func (dr *drainRequestImpl) Status() (DrainStatus, error) {
	ds := DrainStatus{}
	ds.State = dr.State()

	pdl, errs := dr.drainHelper.GetPodsForDeletion(dr.drainSpec.NodeName)
	if errs != nil {
		return ds, fmt.Errorf("failed to get pods for deletion. %w", k8serror.NewAggregate(errs))
	}
	pl := pdl.Pods()

	ds.PodsToDelete = make([]string, 0, len(pl))
	for _, p := range pl {
		ds.PodsToDelete = append(ds.PodsToDelete, fmt.Sprintf("%s/%s", p.Namespace, p.Name))
	}

	return ds, nil
}

// State implements DrainRequest interface.
func (dr *drainRequestImpl) State() DrainState {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	state := DrainStateNotStarted

	switch {
	case dr.finished:
		if dr.lastErr != nil {
			// distinguish between canceled rquest and drain error
			select {
			case _, ok := <-dr.ctx.Done():
				if !ok {
					state = DrainStateCanceled
				}
			default:
				state = DrainStateError
			}
		} else {
			state = DrainStateSuccess
		}
	case dr.canceled:
		state = DrainStateCanceled
	case dr.started:
		state = DrainStateInProgress
	}

	return state
}

// LastError implements DrainRequest interface.
func (dr *drainRequestImpl) LastError() error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	return dr.lastErr
}

// Spec implements DrainRequest interface.
func (dr *drainRequestImpl) Spec() DrainRequestSpec {
	return dr.drainSpec
}

// doDrain performs node drain
func (dr *drainRequestImpl) doDrain() {
	err := drain.RunNodeDrain(dr.drainHelper, dr.drainSpec.NodeName)

	dr.mu.Lock()
	dr.lastErr = err
	dr.finished = true
	dr.mu.Unlock()

	dr.log.Info("doDrain() finished")
	if dr.lastErr != nil {
		dr.log.Error(dr.lastErr, "doDrain() finished with error")
	}
}

// createDrainHelper creates a drain helper.
func (dr *drainRequestImpl) createDrainHelper() *drain.Helper {
	drainer := &drain.Helper{
		Ctx:                 dr.ctx,
		Client:              dr.k8sInterface,
		Force:               dr.drainSpec.Spec.Force,
		GracePeriodSeconds:  -1,
		IgnoreAllDaemonSets: true,
		Timeout:             time.Duration(dr.drainSpec.Spec.TimeoutSecond) * time.Second,
		DeleteEmptyDirData:  dr.drainSpec.Spec.DeleteEmptyDir,
		PodSelector:         dr.drainSpec.Spec.PodSelector,

		// print args
		Out:    newLogInfoWriter(dr.log.Info),
		ErrOut: newLogErrWriter(dr.log.Error),
		OnPodDeletionOrEvictionStarted: func(pod *corev1.Pod, usingEviction bool) {
			verbStr := DrainDeleted
			if usingEviction {
				verbStr = DrainEvicted
			}
			dr.log.Info(fmt.Sprintf("%s pod from Node %s started: %s/%s",
				verbStr, dr.drainSpec.NodeName, pod.Namespace, pod.Name))
		},

		OnPodDeletionOrEvictionFinished: func(pod *corev1.Pod, usingEviction bool, err error) {
			verbStr := DrainDeleted
			if usingEviction {
				verbStr = DrainEvicted
			}
			if err != nil {
				dr.log.Error(err, fmt.Sprintf("%s pod from Node %s finished with failure: %s/%s",
					verbStr, dr.drainSpec.NodeName, pod.Namespace, pod.Name))
			} else {
				dr.log.Info(fmt.Sprintf("%s pod from Node %s finished: %s/%s",
					verbStr, dr.drainSpec.NodeName, pod.Namespace, pod.Name))
			}
		},
	}

	if len(dr.drainSpec.Spec.PodEvictionFilters) > 0 {
		drainer.AdditionalFilters = []drain.PodFilter{drainFilterFromPodEvictionFilters(dr.drainSpec.Spec.PodEvictionFilters)}
	}

	return drainer
}

// newLogInfoWriter creates a new logWriter from log.Info func
func newLogInfoWriter(logFunc func(msg string, keysAndValues ...interface{})) io.Writer {
	return &logWriter{logFunc: logFunc}
}

// newLogErrWriter creates a new logWriter from log.Error func
func newLogErrWriter(errLogFunc func(err error, msg string, keysAndValues ...interface{})) io.Writer {
	logFunc := func(msg string, keysAndValues ...interface{}) {
		errLogFunc(nil, msg, keysAndValues...)
	}
	return &logWriter{logFunc: logFunc}
}

// logWriter implements io.Writer interface as a pass-through for log print func.
type logWriter struct {
	logFunc func(msg string, keysAndValues ...interface{})
}

// Write passes string(p) into writer's logFunc and always returns len(p)
func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logFunc(string(p))
	return len(p), nil
}

// drainFilterFromPodEvictionFilters constructs drain.PodFilter from PodEvictionFiterEntries
func drainFilterFromPodEvictionFilters(f []maintenancev1.PodEvictionFiterEntry) drain.PodFilter {
	return func(pod corev1.Pod) drain.PodDeleteStatus {
		var toDelete bool

		for _, e := range f {
			toDelete = toDelete || e.Match(&pod)
		}

		return drain.PodDeleteStatus{
			Delete:  toDelete,
			Reason:  "PodMatchPodEvictionFiters",
			Message: "",
		}
	}
}
