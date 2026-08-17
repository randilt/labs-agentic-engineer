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
	"testing"

	"github.com/wso2/aep/aep-api/internal/clients/openchoreo"
	"github.com/wso2/aep/aep-api/internal/contracts/taskmeta"
	"github.com/wso2/aep/aep-api/internal/delivery"
)

type watcherRuntime struct {
	pod    openchoreo.RuntimePod
	podErr error
}

func (f *watcherRuntime) ReleaseBindingName(context.Context, string, string, string, string) (string, error) {
	if f.podErr != nil {
		return "", f.podErr
	}
	return "rb-dev", nil
}
func (f *watcherRuntime) PodSnapshot(context.Context, string, string) (openchoreo.RuntimePod, error) {
	return f.pod, f.podErr
}
func (f *watcherRuntime) PodEvents(context.Context, string, string, string) ([]openchoreo.RuntimeEvent, error) {
	return nil, nil
}

type recordingExecs struct {
	row     *delivery.Execution
	reasons []string
}

func (f *recordingExecs) TryAdmit(context.Context, *delivery.Execution) (bool, *delivery.Execution, error) {
	return false, nil, nil
}
func (f *recordingExecs) StartWithRun(context.Context, string, string) (*delivery.Execution, error) {
	return f.row, nil
}
func (f *recordingExecs) Finish(_ context.Context, _, status, reason string) (*delivery.Execution, error) {
	f.reasons = append(f.reasons, reason)
	if f.row != nil {
		f.row.Status = status
		f.row.Reason = reason
	}
	return f.row, nil
}
func (f *recordingExecs) GetByIDScoped(context.Context, string, string) (*delivery.Execution, error) {
	return f.row, nil
}
func (f *recordingExecs) LatestPerKindScoped(context.Context, string, string, int) (map[string]*delivery.Execution, error) {
	return nil, nil
}
func (f *recordingExecs) ListActive(context.Context) ([]delivery.Execution, error) {
	if f.row == nil {
		return nil, nil
	}
	return []delivery.Execution{*f.row}, nil
}

func runningAnalysis() *delivery.Execution {
	return &delivery.Execution{
		ID:        "e-analysis",
		OrgID:     "org",
		ProjectID: "proj",
		Kind:      string(taskmeta.KindAnalysis),
		Status:    string(taskmeta.ExecRunning),
		RunName:   "run-1",
		Component: AnalysisComponentSentinel,
	}
}

func TestAnalysisWatcher_ExitedWithoutFactsFailsAfterGrace(t *testing.T) {
	execs := &recordingExecs{row: runningAnalysis()}
	svc := NewService(Deps{Execs: execs})
	rt := &watcherRuntime{pod: openchoreo.RuntimePod{Found: true, Name: "p1", Phase: "Succeeded"}}
	w := NewAnalysisWatcher(rt, execs, svc, nil, 0)

	w.Sweep(context.Background())
	w.Sweep(context.Background())
	if len(execs.reasons) != 0 {
		t.Fatalf("failed too early: %v", execs.reasons)
	}
	w.Sweep(context.Background())
	if len(execs.reasons) != 1 || execs.reasons[0] != reasonExitedWithoutFacts {
		t.Fatalf("reasons = %v, want %q", execs.reasons, reasonExitedWithoutFacts)
	}
}

func TestAnalysisWatcher_JobGoneFailsWithFactsReason(t *testing.T) {
	execs := &recordingExecs{row: runningAnalysis()}
	svc := NewService(Deps{Execs: execs})
	rt := &watcherRuntime{podErr: openchoreo.ErrNotFound}
	w := NewAnalysisWatcher(rt, execs, svc, nil, 0)

	w.Sweep(context.Background())
	w.Sweep(context.Background())
	if len(execs.reasons) != 0 {
		t.Fatalf("failed too early: %v", execs.reasons)
	}
	w.Sweep(context.Background())
	if len(execs.reasons) != 1 || execs.reasons[0] != reasonJobGoneBeforeFacts {
		t.Fatalf("reasons = %v, want %q", execs.reasons, reasonJobGoneBeforeFacts)
	}
}
