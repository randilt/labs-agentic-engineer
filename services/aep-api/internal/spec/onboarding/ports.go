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

	"github.com/wso2/aep/aep-api/internal/delivery"
)

// ExecutionStore is the admission + lifecycle half of an analysis run.
type ExecutionStore interface {
	TryAdmit(ctx context.Context, e *delivery.Execution) (admitted bool, row *delivery.Execution, err error)
	StartWithRun(ctx context.Context, id, runName string) (*delivery.Execution, error)
	Finish(ctx context.Context, id, status, reason string) (*delivery.Execution, error)
	GetByIDScoped(ctx context.Context, orgID, id string) (*delivery.Execution, error)
	LatestPerKindScoped(ctx context.Context, orgID, repo string, issueNumber int) (map[string]*delivery.Execution, error)
	ListActive(ctx context.Context) ([]delivery.Execution, error)
}

// RepoLocator resolves the project's GitHub full name for execution rows.
type RepoLocator interface {
	RepoFullName(ctx context.Context, orgID, projectID string) (string, error)
}

// ProjectRepo validates project ownership and reads clone metadata.
type ProjectRepo interface {
	GetRepo(ctx context.Context, orgID, projectID string) (repoURL string, ok bool, err error)
}

// AnalysisDispatcher launches the analysis runner pod.
type AnalysisDispatcher interface {
	DispatchAnalysis(ctx context.Context, in AnalysisDispatchInput) (runName string, err error)
}

// AnalysisDispatchInput is the foreign-repo analysis launch payload.
type AnalysisDispatchInput struct {
	OrgID, ProjectID, ExecutionID, SourceRepoRef string
}

// FilesCommitter commits onboarding facts into the project spec bundle.
type FilesCommitter interface {
	CommitFile(ctx context.Context, orgID, projectID, path, content string) error
}
