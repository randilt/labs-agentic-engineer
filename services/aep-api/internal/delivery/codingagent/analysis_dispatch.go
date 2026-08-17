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

package codingagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/aep/aep-api/internal/platform/auth"
	"github.com/wso2/aep/aep-api/internal/sourcecontrol"
)

// analysisComponentSentinel is the AEP_COMPONENT_NAME for onboarding analysis
// pods — a label value only, not a design.json component path.
const analysisComponentSentinel = "aep-onboarding-analysis"

// analysisTaskKind selects the runner's analysis branch (AEP_TASK_KIND).
const analysisTaskKind = "analysis"

// analysisDeadlineSeconds bounds reverse-engineering a foreign repo (2h).
const analysisDeadlineSeconds int64 = 7200

// AnalysisDispatchInput launches one foreign-repo analysis execution.
type AnalysisDispatchInput struct {
	OrgID, ProjectID, ExecutionID, SourceRepoRef string
}

// DispatchAnalysis launches a runner pod that clones SourceRepoRef and posts
// structured facts to the internal onboarding callback. It writes no execution
// row — the caller admits and StartWithRun's.
func (e *CodingExecutor) DispatchAnalysis(ctx context.Context, in AnalysisDispatchInput) (string, error) {
	if in.OrgID == "" || in.ProjectID == "" || in.ExecutionID == "" {
		return "", fmt.Errorf("analysis dispatch: org, project, and execution id are required")
	}
	sourceRepoRef := strings.TrimSpace(in.SourceRepoRef)
	if sourceRepoRef == "" {
		return "", fmt.Errorf("analysis dispatch: sourceRepoRef is required")
	}
	parsed, err := parseAnalysisSourceRef(sourceRepoRef)
	if err != nil {
		return "", err
	}

	if e.skillMirror != nil {
		_ = e.skillMirror.SyncProjectSkills(ctx, in.OrgID, in.ProjectID)
	}

	platform := strings.TrimRight(e.platformURL, "/")
	prompt := buildAnalysisPrompt(platform, in.ExecutionID, sourceRepoRef, parsed.Ref)
	shape := dispatchShape{
		prompt:        prompt,
		componentName: analysisComponentSentinel,
		taskKind:      analysisTaskKind,
		deadline:      analysisDeadlineSeconds,
	}

	sourceRepo := &sourcecontrol.GitRepository{RepoURL: parsed.CloneURL}
	return e.launchAnalysisAgent(ctx, agentLaunch{
		orgID:         in.OrgID,
		projectID:     in.ProjectID,
		correlationID: in.ExecutionID,
		shape:         shape,
		repo:          sourceRepo,
	}, sourceRepoRef, parsed.Ref)
}

type parsedAnalysisSource struct {
	CloneURL string
	Ref      string
}

func parseAnalysisSourceRef(ref string) (parsedAnalysisSource, error) {
	ref = strings.TrimSpace(ref)
	ownerName := ref
	pin := ""
	if at := strings.LastIndex(ref, "@"); at > 0 {
		ownerName = ref[:at]
		pin = strings.TrimSpace(ref[at+1:])
	}
	parts := strings.Split(ownerName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return parsedAnalysisSource{}, fmt.Errorf("analysis dispatch: expected owner/name or owner/name@ref, got %q", ref)
	}
	return parsedAnalysisSource{
		CloneURL: "https://github.com/" + ownerName,
		Ref:      pin,
	}, nil
}

func buildAnalysisPrompt(platformURL, executionID, sourceRepoRef, ref string) string {
	callback := strings.TrimRight(platformURL, "/") + "/internal/v1/onboarding/" + executionID + "/facts"
	var b strings.Builder
	b.WriteString("Run the codebase-analysis skill on the cloned repository.\n")
	b.WriteString("Source: ")
	b.WriteString(sourceRepoRef)
	b.WriteString("\n")
	if ref != "" {
		b.WriteString("Checkout ref: ")
		b.WriteString(ref)
		b.WriteString("\n")
	}
	b.WriteString("POST the structured facts JSON to ")
	b.WriteString(callback)
	b.WriteString(" using the runner bearer before exiting.\n")
	b.WriteString("Do not open a pull request. Do not modify the source repository.\n")
	return b.String()
}

func (e *CodingExecutor) launchAnalysisAgent(ctx context.Context, in agentLaunch, sourceRepoRef, ref string) (string, error) {
	repo := in.repo
	if repo == nil {
		return "", fmt.Errorf("analysis dispatch: source repo required")
	}
	if e.ocJobs == nil {
		return "", fmt.Errorf("no coding-agent dispatch path configured: set AGENT_RUNNER_IMAGE")
	}
	name, email, login, err := e.identities.IdentityFor(ctx, in.orgID)
	if err != nil {
		return "", fmt.Errorf("resolve git identity: %w", err)
	}
	bearer, err := e.tokens.Issue(in.correlationID, in.orgID, in.projectID)
	if err != nil {
		return "", fmt.Errorf("mint runner bearer: %w", err)
	}
	mcpToken, err := e.tokens.IssueServiceToken(auth.AudienceMCP, in.orgID, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("mint MCP token: %w", err)
	}
	// The org PAT (githubSR) authenticates the foreign clone the same way a
	// coding Job clones the project repo. StageSourceSecret provisions an
	// OpenChoreo GitSecret for build WorkflowRuns; this Job is a coding-agent
	// pod and already mounts GITHUB_TOKEN via SecretEnvRef.
	anthropicSR, githubSR, err := e.resolveRunnerSecretRefs(ctx, in.orgID)
	if err != nil {
		return "", err
	}
	disp := in.shape
	platform := strings.TrimRight(e.platformURL, "/")
	env := map[string]string{
		"AEP_TASK_ID":           in.correlationID,
		"AEP_ORG_ID":            in.orgID,
		"AEP_PROJECT_ID":        in.projectID,
		"AEP_COMPONENT_NAME":    disp.componentName,
		"AEP_REPO_URL":          repo.RepoURL,
		"AEP_PROMPT":            disp.prompt,
		"AEP_GIT_SERVICE_URL":   e.gitServiceURL,
		"AEP_PLATFORM_URL":      e.platformURL,
		"AEP_MCP_URL":           platform + "/internal/v1/mcp",
		"AEP_IDENTITY_NAME":     name,
		"AEP_IDENTITY_EMAIL":    email,
		"AEP_IDENTITY_LOGIN":    login,
		"AEP_CORRELATION_ID":    in.correlationID,
		"AEP_TASK_KIND":         taskKindOrDefault(disp.taskKind),
		"AEP_SOURCE_REPO_REF":   sourceRepoRef,
		"AEP_ANALYSIS_CALLBACK": platform + "/internal/v1/onboarding/" + in.correlationID + "/facts",
		"WORKSPACE_BASE_PATH":   codingAgentWorkspacePath,
	}
	if ref != "" {
		env["AEP_SOURCE_REF"] = ref
	}
	if bearer != "" {
		env["AEP_BEARER"] = bearer
	}
	if mcpToken != "" {
		env["AEP_MCP_TOKEN"] = mcpToken
	}
	return e.ocJobs.Dispatch(ctx, OCDispatchInputs{
		OrgID:                 in.orgID,
		ProjectID:             in.projectID,
		CycleID:               in.correlationID,
		Kind:                  disp.taskKind,
		RunName:               codingAgentRunNameFor(in.projectID, in.correlationID),
		ActiveDeadlineSeconds: int(disp.deadline),
		Env:                   env,
		SecretEnv: []SecretEnvRef{
			{Key: anthropicEnvVarOrDefault(anthropicSR.EnvVar), SecretName: anthropicSR.SecretRefName, SecretKey: anthropicSR.Property},
			{Key: envGitHubToken, SecretName: githubSR.SecretRefName, SecretKey: githubSR.Property},
		},
	})
}
