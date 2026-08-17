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

type fakeIssues struct {
	created []sourcecontrol.CreateIssueRequest
}

func (f *fakeIssues) ListIssues(context.Context, string, string, []string) ([]sourcecontrol.IssueInfo, error) {
	return nil, nil
}
func (f *fakeIssues) CreateIssue(_ context.Context, _, _ string, req sourcecontrol.CreateIssueRequest) (*sourcecontrol.IssueResult, error) {
	f.created = append(f.created, req)
	return &sourcecontrol.IssueResult{Number: 42}, nil
}
func (f *fakeIssues) CommentIssue(context.Context, string, string, int, string) error { return nil }
func (f *fakeIssues) CreatePullRequest(context.Context, string, string, sourcecontrol.CreatePullRequestRequest) (*sourcecontrol.PullRequestResult, error) {
	return &sourcecontrol.PullRequestResult{Number: 1, URL: "https://github.com/o/r/pull/1"}, nil
}

type fakeDesign struct{ comps []spec.DesignComponent }

func (f fakeDesign) ReadDesignComponents(context.Context, string, string) ([]spec.DesignComponent, error) {
	return f.comps, nil
}

func TestEnsureOnboardIssues_NeverStampsAepLabel(t *testing.T) {
	issues := &fakeIssues{}
	svc := NewService(Deps{Issues: issues, Design: fakeDesign{comps: []spec.DesignComponent{{
		Name:       "legacy-api",
		SourceMode: spec.SourceModeImportAsIs,
		Source:     &spec.ComponentSource{Repo: "org/legacy"},
		AppPath:    "legacy-api",
	}}}})
	if _, err := svc.EnsureOnboardIssues(context.Background(), "org", "proj", "v1", 3); err != nil {
		t.Fatalf("EnsureOnboardIssues: %v", err)
	}
	if len(issues.created) != 1 {
		t.Fatalf("created %d issues, want 1", len(issues.created))
	}
	labels := issues.created[0].Labels
	if delivery.HasLabel(labels, delivery.LabelAgentWork) {
		t.Fatalf("onboard issue carries aep working-set label: %v", labels)
	}
	if !delivery.HasLabel(labels, delivery.LabelOnboard) {
		t.Fatalf("onboard issue missing aep:onboard: %v", labels)
	}
}

func TestOnboardBranch_NotCycleBranch(t *testing.T) {
	b := onboardBranch("my-service")
	if stringsHasPrefixMilestone(b) {
		t.Fatalf("onboard branch %q matches cycle branch pattern", b)
	}
}

func stringsHasPrefixMilestone(s string) bool {
	return len(s) > 6 && s[:6] == "aep/m"
}
