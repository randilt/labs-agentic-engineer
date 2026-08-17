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

package onboard

import (
	"context"
	"testing"

	"github.com/wso2/aep/aep-api/internal/delivery"
	"github.com/wso2/aep/aep-api/internal/sourcecontrol"
	"github.com/wso2/aep/aep-api/internal/spec"
)

type fakeReports struct{ body string }

func (f fakeReports) ReadAt(context.Context, string, string, string, string) (string, error) {
	return f.body, nil
}

type fakeEdges struct{ calls [][2]string }

func (f *fakeEdges) RewriteCutoverEdges(_ context.Context, _, _, from, to string) (int, error) {
	f.calls = append(f.calls, [2]string{from, to})
	return 1, nil
}

type fakeDeleter struct{ deleted []string }

func (f *fakeDeleter) DeleteComponentCascade(_ context.Context, _, _, name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

type fakeCycleRec struct{ verdict string }

func (f *fakeCycleRec) SetCutoverVerdict(_ context.Context, _, verdict string) error {
	f.verdict = verdict
	return nil
}

type fakeExecs struct{}

func (fakeExecs) TryAdmit(_ context.Context, e *delivery.Execution) (bool, *delivery.Execution, error) {
	return true, e, nil
}
func (fakeExecs) StartWithRun(context.Context, string, string) (*delivery.Execution, error) {
	return nil, nil
}
func (fakeExecs) Finish(context.Context, string, string, string) (*delivery.Execution, error) {
	return nil, nil
}
func (fakeExecs) LatestPerKindScoped(context.Context, string, string, int) (map[string]*delivery.Execution, error) {
	return nil, nil
}

type fakeRepoLoc struct{}

func (fakeRepoLoc) RepoFullName(context.Context, string, string) (string, error) {
	return "acme/app", nil
}
func (fakeRepoLoc) RepoRecord(context.Context, string, string) (*sourcecontrol.GitRepository, error) {
	return nil, nil
}

func TestOnParityMerged_MatchedRewritesAndTearsDown(t *testing.T) {
	edges := &fakeEdges{}
	deleter := &fakeDeleter{}
	cycles := &fakeCycleRec{}
	svc := NewService(Deps{
		Design: fakeDesign{comps: []spec.DesignComponent{
			{Name: "legacy-api", SourceMode: spec.SourceModeImportAsIs},
			{Name: "new-api", SourceMode: spec.SourceModeModernize, Modernizes: "legacy-api"},
			{Name: "web", Dependencies: []spec.Dependency{{Kind: spec.DependencyKindComponent, Name: "legacy-api"}}},
		}},
		Edges:   edges,
		Reports: fakeReports{body: `{"schemaVersion":1,"matched":true}`},
		Deleter: deleter,
		Cycles:  cycles,
		Execs:   fakeExecs{},
		Repos:   fakeRepoLoc{},
	})
	if err := svc.OnParityMerged(context.Background(), CutoverInput{
		OrgID: "org", ProjectID: "proj", MergeSHA: "abc123def456", CycleID: "cycle-1", IssueNumber: 16,
	}); err != nil {
		t.Fatalf("OnParityMerged: %v", err)
	}
	if len(edges.calls) != 1 || edges.calls[0] != [2]string{"legacy-api", "new-api"} {
		t.Fatalf("rewrite calls = %v", edges.calls)
	}
	if len(deleter.deleted) != 1 || deleter.deleted[0] != "legacy-api" {
		t.Fatalf("deleted = %v, want [legacy-api]", deleter.deleted)
	}
	if cycles.verdict != delivery.CycleCutoverComplete {
		t.Fatalf("verdict = %q, want %q", cycles.verdict, delivery.CycleCutoverComplete)
	}
}

func TestOnParityMerged_MismatchNeverCutsOver(t *testing.T) {
	edges := &fakeEdges{}
	deleter := &fakeDeleter{}
	cycles := &fakeCycleRec{}
	svc := NewService(Deps{
		Design: fakeDesign{comps: []spec.DesignComponent{
			{Name: "legacy-api", SourceMode: spec.SourceModeImportAsIs},
			{Name: "new-api", SourceMode: spec.SourceModeModernize, Modernizes: "legacy-api"},
		}},
		Edges:   edges,
		Reports: fakeReports{body: `{"schemaVersion":1,"matched":false}`},
		Deleter: deleter,
		Cycles:  cycles,
	})
	if err := svc.OnParityMerged(context.Background(), CutoverInput{
		OrgID: "org", ProjectID: "proj", MergeSHA: "abc123def456", CycleID: "cycle-1",
	}); err != nil {
		t.Fatalf("OnParityMerged: %v", err)
	}
	if len(edges.calls) != 0 {
		t.Fatalf("mismatch must not rewrite, got %v", edges.calls)
	}
	if len(deleter.deleted) != 0 {
		t.Fatalf("mismatch must not teardown, got %v", deleter.deleted)
	}
	if cycles.verdict != delivery.CycleCutoverMismatch {
		t.Fatalf("verdict = %q, want %q", cycles.verdict, delivery.CycleCutoverMismatch)
	}
}
