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
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wso2/aep/aep-api/internal/contracts/taskmeta"
	"github.com/wso2/aep/aep-api/internal/delivery"
	"github.com/wso2/aep/aep-api/internal/platform/secrets"
	"github.com/wso2/aep/aep-api/internal/sourcecontrol"
	"github.com/wso2/aep/aep-api/internal/spec"
	"github.com/wso2/aep/aep-api/internal/spec/onboarding"
)

// vendorComponent clones the component's foreign source, commits it unmodified
// onto aep/onboard/<component>, and opens a pull request that resolves the
// onboard issue.
func (s *Service) vendorComponent(ctx context.Context, orgID, projectID, repo string, comp spec.DesignComponent, issueNumber int) error {
	if comp.Source == nil || strings.TrimSpace(comp.Source.Repo) == "" {
		return s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, "", "component source pointer is missing")
	}
	parsed, err := onboarding.ParseSourceRepoRef(comp.Source.Repo)
	if err != nil {
		return s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, "", "invalid source repo: "+err.Error())
	}
	if comp.Source.Ref != "" {
		parsed.Ref = comp.Source.Ref
	}

	row, admitted, err := s.admitOnboardRow(ctx, orgID, projectID, repo, comp.Name, issueNumber)
	if err != nil {
		return err
	}
	if !admitted {
		return nil
	}
	execID := row.ID

	cred, err := s.creds.Resolve(ctx, orgID)
	if err != nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "resolve git credential: "+err.Error())
		return nil
	}

	cloneDir, cleanup, err := cloneForeignRepo(ctx, parsed.CloneURL, parsed.Ref, cred)
	if err != nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "clone source: "+err.Error())
		return nil
	}
	defer cleanup()

	root := cloneDir
	if sub := strings.Trim(comp.Source.Subpath, "/"); sub != "" {
		root = filepath.Join(cloneDir, sub)
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "source subpath not found: "+comp.Source.Subpath)
		return nil
	}

	if !dockerfilePresent(root) {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID,
			"no Dockerfile at vendored root — add one in the source repo before onboarding")
		return nil
	}

	files, err := collectTree(root)
	if err != nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "read source tree: "+err.Error())
		return nil
	}

	appPath := strings.Trim(comp.AppPath, "/")
	if appPath == "" {
		appPath = depSlug(comp.Name)
	}

	ref, err := s.git.WorkspaceRef(ctx, orgID, projectID)
	if err != nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "resolve project repo: "+err.Error())
		return nil
	}
	branch := onboardBranch(comp.Name)
	msg := fmt.Sprintf("onboard(%s): vendor import-as-is from %s", comp.Name, parsed.OwnerName)
	res, err := s.git.MutateBranch(ctx, ref, branch, func(tx sourcecontrol.Tx) error {
		prefix := appPath
		if prefix != "" {
			prefix += "/"
		}
		for rel, content := range files {
			path := prefix + filepath.ToSlash(rel)
			tx.Write(path, content)
		}
		return nil
	}, msg)
	if err != nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "commit vendored tree: "+err.Error())
		return nil
	}
	if !res.Changed {
		slog.InfoContext(ctx, "onboard: vendored tree unchanged", "component", comp.Name)
	}

	repoRecord, err := s.repos.RepoRecord(ctx, orgID, projectID)
	if err != nil || repoRecord == nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "read project repo record")
		return nil
	}
	base := repoRecord.DefaultBranch
	if base == "" {
		base = "main"
	}

	prBody := fmt.Sprintf("Resolves #%d\n\nVendored unmodified from `%s`.", issueNumber, parsed.OwnerName)
	pr, err := s.issues.CreatePullRequest(ctx, orgID, projectID, sourcecontrol.CreatePullRequestRequest{
		Title: fmt.Sprintf("Onboard %s (import as-is)", comp.Name),
		Body:  prBody,
		Head:  branch,
		Base:  base,
	})
	if err != nil {
		s.failOnboard(ctx, orgID, projectID, issueNumber, comp.Name, execID, "open pull request: "+err.Error())
		return nil
	}

	if _, serr := s.execs.StartWithRun(ctx, execID, branch); serr != nil {
		slog.WarnContext(ctx, "onboard: start execution failed", "execution", execID, "error", serr)
	}
	reason := fmt.Sprintf("Opened pull request #%d vendoring `%s`. Build and deploy follow on merge.", pr.Number, comp.Name)
	if _, ferr := s.execs.Finish(ctx, execID, string(taskmeta.ExecSucceeded), reason); ferr != nil {
		slog.WarnContext(ctx, "onboard: finish execution failed", "execution", execID, "error", ferr)
	}
	if err := s.issues.CommentIssue(ctx, orgID, projectID, issueNumber,
		fmt.Sprintf("✅ %s\n\nPull request: %s", reason, pr.URL)); err != nil {
		slog.WarnContext(ctx, "onboard: comment issue failed", "issue", issueNumber, "error", err)
	}
	return nil
}

func dockerfilePresent(root string) bool {
	for _, name := range []string{"Dockerfile", "dockerfile", "Dockerfile.dockerfile"} {
		if st, err := os.Stat(filepath.Join(root, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func collectTree(root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	return out, err
}

func cloneForeignRepo(ctx context.Context, cloneURL, ref string, cred secrets.Credential) (dir string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "aep-onboard-clone-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }

	env := append(os.Environ(), gitCredentialEnv(ctx, cred)...)
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("clone", "--depth", "1", cloneURL, tmp); err != nil {
		cleanup()
		return "", nil, err
	}
	if ref = strings.TrimSpace(ref); ref != "" {
		if err := checkoutRef(tmp, ref, run); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return tmp, cleanup, nil
}

// checkoutRef pins the clone to ref — a branch, tag, or commit SHA. The shallow
// clone above carries only the default branch tip, so anything else needs an
// explicit fetch. `clone --branch` cannot be used here: it rejects commit SHAs,
// and the analysis half of onboarding (runners/remote-worker workspace.ts
// checkoutSourceRef) accepts them, so both halves must read owner/name@ref the
// same way.
func checkoutRef(dir, ref string, run func(args ...string) error) error {
	in := func(args ...string) error {
		return run(append([]string{"-C", dir}, args...)...)
	}
	if err := in("checkout", "--detach", ref); err == nil {
		return nil
	}
	if err := in("fetch", "--depth", "1", "origin", ref); err != nil {
		return fmt.Errorf("fetch ref %q: %w", ref, err)
	}
	return in("checkout", "--detach", "FETCH_HEAD")
}

func gitCredentialEnv(ctx context.Context, cred secrets.Credential) []string {
	if cred == nil {
		return nil
	}
	token, _, err := cred.Token(ctx)
	if err != nil || token == "" {
		return nil
	}
	user := gitHTTPSUsername(cred)
	// Inline askpass script — one-shot env for foreign clone only.
	script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\n*Username*) echo %q ;;\n*Password*) echo %q ;;\nesac\n", user, token)
	path := filepath.Join(os.TempDir(), "aep-onboard-askpass.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return nil
	}
	return []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=" + path,
	}
}

func gitHTTPSUsername(cred secrets.Credential) string {
	if cred == nil {
		return "x-access-token"
	}
	id := cred.Identity()
	if id.Login != "" {
		return id.Login
	}
	return "x-access-token"
}

func (s *Service) admitOnboardRow(ctx context.Context, orgID, projectID, repo, component string, issueNumber int) (*delivery.Execution, bool, error) {
	admitted, row, err := s.execs.TryAdmit(ctx, &delivery.Execution{
		OrgID:       orgID,
		ProjectID:   projectID,
		Repo:        repo,
		IssueNumber: issueNumber,
		Kind:        string(taskmeta.KindOps),
		Status:      string(taskmeta.ExecQueued),
		Component:   component,
	})
	return row, admitted, err
}

func (s *Service) failOnboard(ctx context.Context, orgID, projectID string, issueNumber int, component, execID, reason string) error {
	if execID != "" {
		if _, err := s.execs.Finish(ctx, execID, string(taskmeta.ExecFailed), reason); err != nil {
			slog.WarnContext(ctx, "onboard: finish failed execution", "execution", execID, "error", err)
		}
	}
	if issueNumber > 0 {
		body := fmt.Sprintf("⚠️ Onboard `%s` blocked: %s", component, reason)
		if err := s.issues.CommentIssue(ctx, orgID, projectID, issueNumber, body); err != nil {
			slog.WarnContext(ctx, "onboard: comment failure", "issue", issueNumber, "error", err)
		}
	}
	return fmt.Errorf("onboard %s: %s", component, reason)
}
