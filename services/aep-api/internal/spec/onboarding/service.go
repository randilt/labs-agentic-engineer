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
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/aep/aep-api/internal/contracts/taskmeta"
	"github.com/wso2/aep/aep-api/internal/delivery"
)

const (
	// AnalysisComponentSentinel is the execution row's component field and the
	// runner's AEP_COMPONENT_NAME — a label value only, not a design component.
	AnalysisComponentSentinel = "aep-onboarding-analysis"
	// AnalysisFactsPath is committed when the analysis callback succeeds.
	AnalysisFactsPath = "specs/onboarding/analysis.json"
)

var (
	ErrProjectNotFound    = errors.New("onboarding: project not found")
	ErrAnalysisInFlight   = errors.New("onboarding: analysis already in flight")
	ErrExecutionNotFound  = errors.New("onboarding: execution not found")
	ErrExecutionNotActive = errors.New("onboarding: execution not active")
	ErrInvalidSourceRef   = errors.New("onboarding: invalid sourceRepoRef")
)

// Service coordinates onboarding analysis: admit → dispatch → facts callback.
type Service struct {
	repos      ProjectRepo
	repoName   RepoLocator
	execs      ExecutionStore
	dispatcher AnalysisDispatcher
	files      FilesCommitter
}

// Deps is the onboarding service's collaborator set.
type Deps struct {
	Repos      ProjectRepo
	RepoName   RepoLocator
	Execs      ExecutionStore
	Dispatcher AnalysisDispatcher
	Files      FilesCommitter
}

// NewService wires the onboarding service.
func NewService(d Deps) *Service {
	return &Service{
		repos:      d.Repos,
		repoName:   d.RepoName,
		execs:      d.Execs,
		dispatcher: d.Dispatcher,
		files:      d.Files,
	}
}

// StartAnalysis admits one analysis execution and dispatches the runner pod.
func (s *Service) StartAnalysis(ctx context.Context, orgID, projectID, sourceRepoRef string) (string, error) {
	sourceRepoRef = strings.TrimSpace(sourceRepoRef)
	if sourceRepoRef == "" {
		return "", fmt.Errorf("%w: sourceRepoRef is required", ErrInvalidSourceRef)
	}
	if _, err := ParseSourceRepoRef(sourceRepoRef); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidSourceRef, err)
	}
	if s.repos == nil {
		return "", fmt.Errorf("onboarding: project repo lookup not configured")
	}
	repoURL, ok, err := s.repos.GetRepo(ctx, orgID, projectID)
	if err != nil {
		return "", fmt.Errorf("onboarding: lookup project repo: %w", err)
	}
	if !ok || repoURL == "" {
		return "", ErrProjectNotFound
	}
	if s.repoName == nil || s.execs == nil || s.dispatcher == nil {
		return "", fmt.Errorf("onboarding: service not fully configured")
	}
	repoFull, err := s.repoName.RepoFullName(ctx, orgID, projectID)
	if err != nil {
		return "", fmt.Errorf("onboarding: resolve repo full name: %w", err)
	}

	admitted, row, err := s.execs.TryAdmit(ctx, &delivery.Execution{
		OrgID:       orgID,
		ProjectID:   projectID,
		Repo:        repoFull,
		IssueNumber: 0,
		Kind:        string(taskmeta.KindAnalysis),
		Status:      string(taskmeta.ExecQueued),
		Component:   AnalysisComponentSentinel,
	})
	if err != nil {
		return "", fmt.Errorf("onboarding: admit execution: %w", err)
	}
	if !admitted || row == nil {
		return "", ErrAnalysisInFlight
	}

	runName, err := s.dispatcher.DispatchAnalysis(ctx, AnalysisDispatchInput{
		OrgID:         orgID,
		ProjectID:     projectID,
		ExecutionID:   row.ID,
		SourceRepoRef: sourceRepoRef,
	})
	if err != nil {
		if _, fErr := s.execs.Finish(ctx, row.ID, string(taskmeta.ExecFailed), err.Error()); fErr != nil {
			slog.WarnContext(ctx, "onboarding: finish failed dispatch", "execution", row.ID, "error", fErr)
		}
		return "", fmt.Errorf("onboarding: dispatch analysis: %w", err)
	}
	if _, err := s.execs.StartWithRun(ctx, row.ID, runName); err != nil {
		return "", fmt.Errorf("onboarding: start execution: %w", err)
	}
	return row.ID, nil
}

// GetStatus returns the latest analysis execution for the project (issue 0).
func (s *Service) GetStatus(ctx context.Context, orgID, projectID string) (status, executionID, reason string, err error) {
	if s.repos == nil {
		return "", "", "", fmt.Errorf("onboarding: project repo lookup not configured")
	}
	repoURL, ok, err := s.repos.GetRepo(ctx, orgID, projectID)
	if err != nil {
		return "", "", "", fmt.Errorf("onboarding: lookup project repo: %w", err)
	}
	if !ok || repoURL == "" {
		return "", "", "", ErrProjectNotFound
	}
	if s.repoName == nil || s.execs == nil {
		return "", "", "", fmt.Errorf("onboarding: service not fully configured")
	}
	repoFull, err := s.repoName.RepoFullName(ctx, orgID, projectID)
	if err != nil {
		return "", "", "", fmt.Errorf("onboarding: resolve repo full name: %w", err)
	}
	byKind, err := s.execs.LatestPerKindScoped(ctx, orgID, repoFull, 0)
	if err != nil {
		return "", "", "", fmt.Errorf("onboarding: lookup analysis execution: %w", err)
	}
	row := byKind[string(taskmeta.KindAnalysis)]
	if row == nil {
		return "idle", "", "", nil
	}
	return row.Status, row.ID, row.Reason, nil
}

// ReceiveFacts validates the callback, commits analysis.json, and finishes the row.
func (s *Service) ReceiveFacts(ctx context.Context, orgID, executionID string, facts json.RawMessage) error {
	if s.execs == nil || s.files == nil {
		return fmt.Errorf("onboarding: facts handler not configured")
	}
	row, err := s.execs.GetByIDScoped(ctx, orgID, executionID)
	if err != nil {
		return fmt.Errorf("onboarding: lookup execution: %w", err)
	}
	if row == nil {
		return ErrExecutionNotFound
	}
	if row.Kind != string(taskmeta.KindAnalysis) || row.Component != AnalysisComponentSentinel {
		return ErrExecutionNotFound
	}
	if row.Status != string(taskmeta.ExecQueued) && row.Status != string(taskmeta.ExecRunning) {
		return ErrExecutionNotActive
	}
	if len(facts) == 0 || !json.Valid(facts) {
		return fmt.Errorf("onboarding: facts must be valid JSON")
	}
	pretty, err := json.MarshalIndent(json.RawMessage(facts), "", "  ")
	if err != nil {
		return fmt.Errorf("onboarding: marshal facts: %w", err)
	}
	pretty = append(pretty, '\n')

	if err := s.files.CommitFile(ctx, row.OrgID, row.ProjectID, AnalysisFactsPath, string(pretty)); err != nil {
		return fmt.Errorf("onboarding: commit analysis.json: %w", err)
	}

	exec, err := s.execs.Finish(ctx, executionID, string(taskmeta.ExecSucceeded), "")
	if err != nil {
		return fmt.Errorf("onboarding: finish execution: %w", err)
	}
	if exec == nil {
		return ErrExecutionNotActive
	}
	return nil
}

// FailExecution marks a running analysis execution failed (pod watcher path).
func (s *Service) FailExecution(ctx context.Context, executionID, reason string) {
	if s.execs == nil {
		return
	}
	exec, err := s.execs.Finish(ctx, executionID, string(taskmeta.ExecFailed), reason)
	if err != nil {
		slog.WarnContext(ctx, "onboarding: finish failed analysis", "execution", executionID, "error", err)
		return
	}
	if exec == nil {
		return
	}
	slog.InfoContext(ctx, "onboarding: analysis execution failed", "execution", executionID, "reason", reason)
}
