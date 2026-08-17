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

package app

import (
	"context"

	"github.com/wso2/aep/aep-api/internal/delivery/onboard"
	"github.com/wso2/aep/aep-api/internal/platform/secrets"
	"github.com/wso2/aep/aep-api/internal/projects"
	"github.com/wso2/aep/aep-api/internal/sourcecontrol"
)

type runOnboarder struct {
	svc *onboard.Service
}

func (r runOnboarder) OnboardForMilestone(ctx context.Context, orgID, projectID, tag string, milestoneNumber int) error {
	return r.svc.OnboardForMilestone(ctx, orgID, projectID, tag, milestoneNumber)
}

type onboardRepoLocator struct {
	repos sourcecontrol.RepoRepository
}

func (o onboardRepoLocator) RepoFullName(ctx context.Context, orgID, projectID string) (string, error) {
	return repoFullNameLookup{repos: o.repos}.RepoFullName(ctx, orgID, projectID)
}

func (o onboardRepoLocator) RepoRecord(ctx context.Context, orgID, projectID string) (*sourcecontrol.GitRepository, error) {
	return o.repos.GetByOrgAndProjectID(ctx, orgID, projectID)
}

type onboardGitCommitter struct {
	git   sourcecontrol.GitOpsService
	repos sourcecontrol.RepoRepository
}

func (g onboardGitCommitter) WorkspaceRef(ctx context.Context, orgID, projectID string) (sourcecontrol.RepoRef, error) {
	row, err := g.repos.GetByOrgAndProjectID(ctx, orgID, projectID)
	if err != nil {
		return sourcecontrol.RepoRef{}, err
	}
	if row == nil {
		return sourcecontrol.RepoRef{}, sourcecontrol.ErrRepoNotFound
	}
	return sourcecontrol.ResolveWorkspaceRef(ctx, g.git.Resolver(), orgID, row)
}

func (g onboardGitCommitter) MutateBranch(ctx context.Context, ref sourcecontrol.RepoRef, branch string, fn func(sourcecontrol.Tx) error, message string) (sourcecontrol.CommitResult, error) {
	return g.git.Workspace().Mutate(ctx, ref, fn, sourcecontrol.CommitOpts{
		Message: message,
		Branch:  branch,
	})
}

type onboardCredResolver struct {
	resolver secrets.Resolver
}

func (c onboardCredResolver) Resolve(ctx context.Context, orgID string) (secrets.Credential, error) {
	return c.resolver.Resolve(ctx, orgID)
}

type onboardComponentDeleter struct {
	dep *projects.DeploymentService
}

func (d onboardComponentDeleter) DeleteComponentCascade(ctx context.Context, orgID, projectID, componentName string) error {
	return d.dep.DeleteComponentCascade(ctx, orgID, projectID, componentName)
}
