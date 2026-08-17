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
	"fmt"

	"github.com/wso2/aep/aep-api/internal/delivery/codingagent"
	"github.com/wso2/aep/aep-api/internal/sourcecontrol"
	"github.com/wso2/aep/aep-api/internal/spec"
	"github.com/wso2/aep/aep-api/internal/spec/onboarding"
)

type onboardingProjectRepo struct {
	repos sourcecontrol.RepoRepository
}

func (p onboardingProjectRepo) GetRepo(ctx context.Context, orgID, projectID string) (string, bool, error) {
	row, err := p.repos.GetByOrgAndProjectID(ctx, orgID, projectID)
	if err != nil {
		return "", false, err
	}
	if row == nil {
		return "", false, nil
	}
	return row.RepoURL, true, nil
}

type onboardingFiles struct {
	files spec.FilesService
}

func (f onboardingFiles) CommitFile(ctx context.Context, orgID, projectID, path, content string) error {
	_, conflicts, err := f.files.Apply(ctx, orgID, projectID, spec.ApplyRequest{
		Writes: []spec.WriteOp{{Path: path, Content: content}},
	})
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("apply conflict on %s", path)
	}
	return nil
}

type analysisDispatcher struct {
	exec *codingagent.CodingExecutor
}

func (a analysisDispatcher) DispatchAnalysis(ctx context.Context, in onboarding.AnalysisDispatchInput) (string, error) {
	return a.exec.DispatchAnalysis(ctx, codingagent.AnalysisDispatchInput{
		OrgID:         in.OrgID,
		ProjectID:     in.ProjectID,
		ExecutionID:   in.ExecutionID,
		SourceRepoRef: in.SourceRepoRef,
	})
}
