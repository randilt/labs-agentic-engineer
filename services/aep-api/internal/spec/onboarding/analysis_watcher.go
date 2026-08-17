// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package onboarding

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/wso2/aep/aep-api/internal/clients/openchoreo"
	"github.com/wso2/aep/aep-api/internal/contracts/taskmeta"
	"github.com/wso2/aep/aep-api/internal/delivery"
	"github.com/wso2/aep/aep-api/internal/delivery/codingagent"
)

const (
	reasonExitedWithoutFacts = "analysis pod exited without posting facts"
	reasonJobGoneBeforeFacts = "analysis job disappeared before facts were posted"
)

// RuntimePodReader is the narrow OpenChoreo runtime port for pod truth.
type RuntimePodReader interface {
	ReleaseBindingName(ctx context.Context, orgName, projectName, componentName, environment string) (string, error)
	PodSnapshot(ctx context.Context, orgName, bindingName string) (openchoreo.RuntimePod, error)
	PodEvents(ctx context.Context, orgName, bindingName, podName string) ([]openchoreo.RuntimeEvent, error)
}

// AnalysisWatcher reconciles running analysis executions against pod truth.
type AnalysisWatcher struct {
	runtime   RuntimePodReader
	execs     ExecutionStore
	onFail    *Service
	asService func(ctx context.Context) context.Context
	tick      time.Duration
	startup   time.Duration
	missing   map[string]int
	exited    map[string]int
}

// NewAnalysisWatcher wires the analysis execution watcher.
func NewAnalysisWatcher(runtime RuntimePodReader, execs ExecutionStore, onFail *Service, asService func(ctx context.Context) context.Context, tick time.Duration) *AnalysisWatcher {
	if tick <= 0 {
		tick = 10 * time.Second
	}
	return &AnalysisWatcher{
		runtime:   runtime,
		execs:     execs,
		onFail:    onFail,
		asService: asService,
		tick:      tick,
		startup:   15 * time.Minute,
		missing:   map[string]int{},
		exited:    map[string]int{},
	}
}

// Run sweeps until ctx is canceled.
func (w *AnalysisWatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Sweep(ctx)
		}
	}
}

// Sweep runs one reconciliation pass (exported for tests).
func (w *AnalysisWatcher) Sweep(ctx context.Context) {
	if w.asService != nil {
		ctx = w.asService(ctx)
	}
	active, err := w.execs.ListActive(ctx)
	if err != nil {
		slog.WarnContext(ctx, "onboarding watcher: list active failed", "error", err)
		return
	}
	live := map[string]bool{}
	for i := range active {
		row := &active[i]
		if row.Kind != string(taskmeta.KindAnalysis) || row.Status != string(taskmeta.ExecRunning) || row.RunName == "" {
			continue
		}
		live[row.ID] = true
		w.check(ctx, row)
	}
	for id := range w.missing {
		if !live[id] {
			delete(w.missing, id)
		}
	}
	for id := range w.exited {
		if !live[id] {
			delete(w.exited, id)
		}
	}
}

func (w *AnalysisWatcher) check(ctx context.Context, row *delivery.Execution) {
	binding, err := w.runtime.ReleaseBindingName(ctx, row.OrgID, row.ProjectID, row.RunName, openchoreo.DevEnvironmentName)
	if err != nil {
		w.noteReadFailure(ctx, row, err)
		return
	}
	pod, err := w.runtime.PodSnapshot(ctx, row.OrgID, binding)
	if err != nil {
		w.noteReadFailure(ctx, row, err)
		return
	}
	delete(w.missing, row.ID)

	switch codingagent.ClassifyPod(pod) {
	case codingagent.OutcomeFailed:
		if w.onFail != nil {
			w.onFail.FailExecution(ctx, row.ID, codingagent.FailureReason(pod))
		}
	case codingagent.OutcomePending:
		if row.StartedAt != nil && time.Since(*row.StartedAt) > w.startup {
			var events []openchoreo.RuntimeEvent
			if pod.Found {
				events, _ = w.runtime.PodEvents(ctx, row.OrgID, binding, pod.Name)
			}
			if w.onFail != nil {
				w.onFail.FailExecution(ctx, row.ID, codingagent.StartupFailureReason(pod, events))
			}
		}
	case codingagent.OutcomeSucceeded:
		// Terminal success is the facts callback. A clean pod exit only means
		// the agent process ended — wait a few ticks for the S2S POST, then
		// fail so the console is not stuck on "analyzing" forever.
		w.exited[row.ID]++
		if w.exited[row.ID] >= 3 && w.onFail != nil {
			w.onFail.FailExecution(ctx, row.ID, reasonExitedWithoutFacts)
		}
	case codingagent.OutcomeRunning:
		delete(w.exited, row.ID)
	}
}

func (w *AnalysisWatcher) noteReadFailure(ctx context.Context, row *delivery.Execution, err error) {
	if !errors.Is(err, openchoreo.ErrNotFound) {
		slog.WarnContext(ctx, "onboarding watcher: runtime read failed", "execution", row.ID, "error", err)
		delete(w.missing, row.ID)
		return
	}
	w.missing[row.ID]++
	if w.missing[row.ID] < 3 {
		return
	}
	if w.onFail != nil {
		w.onFail.FailExecution(ctx, row.ID, reasonJobGoneBeforeFacts)
	}
}
