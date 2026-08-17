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

// TeardownComponent deletes the legacy OpenChoreo Component after a successful
// parity cutover. It admits and finishes its own KindOps execution row.
func (s *Service) TeardownComponent(ctx context.Context, orgID, projectID, componentName string, issueNumber int) error {
	if s.deleter == nil {
		return fmt.Errorf("onboard: component deleter not configured")
	}
	repo, err := s.repos.RepoFullName(ctx, orgID, projectID)
	if err != nil {
		return fmt.Errorf("onboard: resolve repo: %w", err)
	}
	row, admitted, err := s.admitOnboardRow(ctx, orgID, projectID, repo, componentName, issueNumber)
	if err != nil {
		return err
	}
	if !admitted {
		return nil
	}
	execID := row.ID
	if err := s.deleter.DeleteComponentCascade(ctx, orgID, projectID, componentName); err != nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, componentName, execID, "teardown: "+err.Error())
		return err
	}
	if _, serr := s.execs.StartWithRun(ctx, execID, componentName); serr != nil {
		slog.WarnContext(ctx, "onboard: start teardown execution failed", "execution", execID, "error", serr)
	}
	reason := fmt.Sprintf("Tore down legacy component `%s`.", componentName)
	if _, ferr := s.execs.Finish(ctx, execID, string(taskmeta.ExecSucceeded), reason); ferr != nil {
		slog.WarnContext(ctx, "onboard: finish teardown execution failed", "execution", execID, "error", ferr)
	}
	if issueNumber > 0 && s.issues != nil {
		_ = s.issues.CommentIssue(ctx, orgID, projectID, issueNumber, "✅ "+reason)
	}
	return nil
}
