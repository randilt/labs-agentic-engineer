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

	"github.com/wso2/aep/aep-api/internal/delivery"
	"github.com/wso2/aep/aep-api/internal/platform/secrets"
	"github.com/wso2/aep/aep-api/internal/sourcecontrol"
	"github.com/wso2/aep/aep-api/internal/spec"
)

// IssueClient is the GitHub issue + pull-request surface the onboard executor
// drives. sourcecontrol.IssueService satisfies it.
type IssueClient interface {
	ListIssues(ctx context.Context, orgID, projectID string, labels []string) ([]sourcecontrol.IssueInfo, error)
	CreateIssue(ctx context.Context, orgID, projectID string, req sourcecontrol.CreateIssueRequest) (*sourcecontrol.IssueResult, error)
	CommentIssue(ctx context.Context, orgID, projectID string, number int, body string) error
	CreatePullRequest(ctx context.Context, orgID, projectID string, req sourcecontrol.CreatePullRequestRequest) (*sourcecontrol.PullRequestResult, error)
}

// ExecutionStore is the executions-rows slice the onboard lifecycle drives.
// delivery.ExecutionRepository satisfies it.
type ExecutionStore interface {
	TryAdmit(ctx context.Context, e *delivery.Execution) (admitted bool, row *delivery.Execution, err error)
	StartWithRun(ctx context.Context, id, runName string) (*delivery.Execution, error)
	Finish(ctx context.Context, id, status, reason string) (*delivery.Execution, error)
	LatestPerKindScoped(ctx context.Context, orgID, repo string, issueNumber int) (map[string]*delivery.Execution, error)
}

// DesignReader reads authored design components at HEAD.
type DesignReader interface {
	ReadDesignComponents(ctx context.Context, orgID, projectID string) ([]spec.DesignComponent, error)
}

// RepoLocator resolves org+project to its GitHub repo full name and default branch.
type RepoLocator interface {
	RepoFullName(ctx context.Context, orgID, projectID string) (string, error)
	RepoRecord(ctx context.Context, orgID, projectID string) (*sourcecontrol.GitRepository, error)
}

// GitCommitter writes vendored trees onto a named branch of the project repo.
type GitCommitter interface {
	WorkspaceRef(ctx context.Context, orgID, projectID string) (sourcecontrol.RepoRef, error)
	MutateBranch(ctx context.Context, ref sourcecontrol.RepoRef, branch string, fn func(sourcecontrol.Tx) error, message string) (sourcecontrol.CommitResult, error)
}

// CredentialResolver mints the org git credential used to clone foreign sources.
type CredentialResolver interface {
	Resolve(ctx context.Context, orgID string) (secrets.Credential, error)
}

// ComponentDeleter removes an OpenChoreo Component on cutover teardown.
type ComponentDeleter interface {
	DeleteComponentCascade(ctx context.Context, orgID, projectID, componentName string) error
}
