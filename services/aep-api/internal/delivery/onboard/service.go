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
	"fmt"
	"log/slog"

	"github.com/wso2/aep/aep-api/internal/contracts/taskmeta"
)

// Service coordinates import-as-is vendoring for a milestone.
type Service struct {
	issues  IssueClient
	execs   ExecutionStore
	design  DesignReader
	repos   RepoLocator
	git     GitCommitter
	creds   CredentialResolver
	deleter ComponentDeleter
	edges   EdgeRewriter
	reports ReportReader
	cycles  CycleRecorder
}

// Deps is the onboard service's collaborator set.
type Deps struct {
	Issues  IssueClient
	Execs   ExecutionStore
	Design  DesignReader
	Repos   RepoLocator
	Git     GitCommitter
	Creds   CredentialResolver
	Deleter ComponentDeleter
	Edges   EdgeRewriter
	Reports ReportReader
	Cycles  CycleRecorder
}

// NewService wires the onboard executor.
func NewService(d Deps) *Service {
	return &Service{
		issues:  d.Issues,
		execs:   d.Execs,
		design:  d.Design,
		repos:   d.Repos,
		git:     d.Git,
		creds:   d.Creds,
		deleter: d.Deleter,
		edges:   d.Edges,
		reports: d.Reports,
		cycles:  d.Cycles,
	}
}

// OnboardForMilestone mints onboard issues and vendors every import-as-is
// component that has not yet opened its pull request.
func (s *Service) OnboardForMilestone(ctx context.Context, orgID, projectID, designTag string, milestoneNumber int) error {
	issueByComponent, err := s.EnsureOnboardIssues(ctx, orgID, projectID, designTag, milestoneNumber)
	if err != nil {
		return err
	}
	comps, err := s.design.ReadDesignComponents(ctx, orgID, projectID)
	if err != nil {
		return fmt.Errorf("onboard: read design: %w", err)
	}
	targets := importAsIsComponents(comps)
	if len(targets) == 0 {
		return nil
	}
	repo, err := s.repos.RepoFullName(ctx, orgID, projectID)
	if err != nil {
		return fmt.Errorf("onboard: resolve repo: %w", err)
	}
	for _, comp := range targets {
		slug := depSlug(comp.Name)
		if slug == "" {
			continue
		}
		issueNum := issueByComponent[slug]
		if issueNum == 0 {
			continue
		}
		if s.alreadyVendored(ctx, orgID, repo, issueNum) {
			continue
		}
		if err := s.vendorComponent(ctx, orgID, projectID, repo, comp, issueNum); err != nil {
			slog.WarnContext(ctx, "onboard: vendor failed", "component", comp.Name, "error", err)
		}
	}
	return nil
}

func (s *Service) alreadyVendored(ctx context.Context, orgID, repo string, issueNumber int) bool {
	if s.execs == nil {
		return false
	}
	rows, err := s.execs.LatestPerKindScoped(ctx, orgID, repo, issueNumber)
	if err != nil {
		return false
	}
	ex := rows[string(taskmeta.KindOps)]
	return ex != nil && ex.Status == string(taskmeta.ExecSucceeded)
}
