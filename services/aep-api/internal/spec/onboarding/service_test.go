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
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2/aep/aep-api/internal/contracts/taskmeta"
	"github.com/wso2/aep/aep-api/internal/delivery"
)

type fakeRepos struct {
	url string
	ok  bool
}

func (f fakeRepos) GetRepo(context.Context, string, string) (string, bool, error) {
	return f.url, f.ok, nil
}

type fakeLocator struct{ name string }

func (f fakeLocator) RepoFullName(context.Context, string, string) (string, error) {
	return f.name, nil
}

type fakeExecs struct {
	row      *delivery.Execution
	admitted bool
}

func (f *fakeExecs) TryAdmit(context.Context, *delivery.Execution) (bool, *delivery.Execution, error) {
	if !f.admitted {
		return false, nil, nil
	}
	return true, f.row, nil
}
func (f *fakeExecs) StartWithRun(context.Context, string, string) (*delivery.Execution, error) {
	return f.row, nil
}
func (f *fakeExecs) Finish(context.Context, string, string, string) (*delivery.Execution, error) {
	return f.row, nil
}
func (f *fakeExecs) GetByIDScoped(context.Context, string, string) (*delivery.Execution, error) {
	return f.row, nil
}
func (f *fakeExecs) LatestPerKindScoped(context.Context, string, string, int) (map[string]*delivery.Execution, error) {
	if f.row == nil {
		return map[string]*delivery.Execution{}, nil
	}
	return map[string]*delivery.Execution{f.row.Kind: f.row}, nil
}
func (f *fakeExecs) ListActive(context.Context) ([]delivery.Execution, error) {
	if f.row == nil {
		return nil, nil
	}
	return []delivery.Execution{*f.row}, nil
}

type fakeDispatcher struct{ run string }

func (f fakeDispatcher) DispatchAnalysis(context.Context, AnalysisDispatchInput) (string, error) {
	return f.run, nil
}

type fakeFiles struct{ path string }

func (f *fakeFiles) CommitFile(_ context.Context, _, _, path, _ string) error {
	f.path = path
	return nil
}

func TestGetStatus_IdleWhenNone(t *testing.T) {
	svc := NewService(Deps{
		Repos:    fakeRepos{url: "https://github.com/acme/p", ok: true},
		RepoName: fakeLocator{name: "acme/p"},
		Execs:    &fakeExecs{},
	})
	status, id, reason, err := svc.GetStatus(context.Background(), "org", "proj")
	if err != nil {
		t.Fatal(err)
	}
	if status != "idle" || id != "" || reason != "" {
		t.Fatalf("got %q %q %q", status, id, reason)
	}
}

func TestGetStatus_Running(t *testing.T) {
	svc := NewService(Deps{
		Repos:    fakeRepos{url: "https://github.com/acme/p", ok: true},
		RepoName: fakeLocator{name: "acme/p"},
		Execs: &fakeExecs{row: &delivery.Execution{
			ID: "e1", Kind: string(taskmeta.KindAnalysis), Status: string(taskmeta.ExecRunning),
		}},
	})
	status, id, reason, err := svc.GetStatus(context.Background(), "org", "proj")
	if err != nil {
		t.Fatal(err)
	}
	if status != "running" || id != "e1" || reason != "" {
		t.Fatalf("got %q %q %q", status, id, reason)
	}
}

func TestGetStatus_FailedReason(t *testing.T) {
	svc := NewService(Deps{
		Repos:    fakeRepos{url: "https://github.com/acme/p", ok: true},
		RepoName: fakeLocator{name: "acme/p"},
		Execs: &fakeExecs{row: &delivery.Execution{
			ID: "e2", Kind: string(taskmeta.KindAnalysis), Status: string(taskmeta.ExecFailed), Reason: "pod crashed",
		}},
	})
	status, id, reason, err := svc.GetStatus(context.Background(), "org", "proj")
	if err != nil {
		t.Fatal(err)
	}
	if status != "failed" || id != "e2" || reason != "pod crashed" {
		t.Fatalf("got %q %q %q", status, id, reason)
	}
}

func TestGetStatus_ProjectMissing(t *testing.T) {
	svc := NewService(Deps{Repos: fakeRepos{}})
	_, _, _, err := svc.GetStatus(context.Background(), "org", "proj")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestReceiveFacts_CommitsAnalysisJSON(t *testing.T) {
	files := &fakeFiles{}
	row := &delivery.Execution{
		ID: "e3", OrgID: "org", ProjectID: "proj",
		Kind: string(taskmeta.KindAnalysis), Component: AnalysisComponentSentinel,
		Status: string(taskmeta.ExecRunning),
	}
	svc := NewService(Deps{
		Execs: &fakeExecs{row: row},
		Files: files,
	})
	if err := svc.ReceiveFacts(context.Background(), "org", "e3", json.RawMessage(`{"sourceRepo":"a/b","ref":"h"}`)); err != nil {
		t.Fatal(err)
	}
	if files.path != AnalysisFactsPath {
		t.Fatalf("committed %q", files.path)
	}
}
