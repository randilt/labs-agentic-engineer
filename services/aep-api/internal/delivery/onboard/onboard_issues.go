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
	"strings"

	"github.com/wso2/aep/aep-api/internal/delivery"
	"github.com/wso2/aep/aep-api/internal/sourcecontrol"
	"github.com/wso2/aep/aep-api/internal/spec"
)

const componentLabelPrefix = "aep:component/"

// EnsureOnboardIssues mints one aep:onboard issue per import-as-is component in
// the approved design. Issues deliberately omit the aep working-set label — they
// hold dispatch via aep:onboard the same way gates hold via aep:provision.
func (s *Service) EnsureOnboardIssues(ctx context.Context, orgID, projectID, designTag string, milestoneNumber int) (map[string]int, error) {
	comps, err := s.design.ReadDesignComponents(ctx, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("onboard: read design: %w", err)
	}
	targets := importAsIsComponents(comps)
	if len(targets) == 0 {
		return nil, nil
	}

	existing, err := s.openOnboardComponents(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}

	issueByComponent := make(map[string]int, len(targets))
	for key, num := range existing {
		issueByComponent[key] = num
	}

	var created int
	for _, comp := range targets {
		key := depSlug(comp.Name)
		if key == "" {
			continue
		}
		if issueByComponent[key] > 0 {
			continue
		}
		labels := []string{delivery.LabelOnboard}
		if l := componentLabel(comp.Name); l != "" {
			labels = append(labels, l)
		}
		req := sourcecontrol.CreateIssueRequest{
			Title:     onboardIssueTitle(comp.Name),
			Body:      onboardIssueBody(comp),
			Labels:    labels,
			DedupeKey: "onboard:" + projectID + ":" + designTag + ":" + key,
		}
		if milestoneNumber > 0 {
			n := milestoneNumber
			req.Milestone = &n
		}
		res, cerr := s.issues.CreateIssue(ctx, orgID, projectID, req)
		if cerr != nil {
			slog.WarnContext(ctx, "onboard: create issue failed", "component", comp.Name, "error", cerr)
			continue
		}
		if res != nil {
			issueByComponent[key] = res.Number
		}
		created++
	}
	if created > 0 {
		slog.InfoContext(ctx, "onboard: minted issues", "project", projectID, "count", created)
	}
	return issueByComponent, nil
}

func importAsIsComponents(comps []spec.DesignComponent) []spec.DesignComponent {
	var out []spec.DesignComponent
	for _, c := range comps {
		if c.IsImportAsIs() {
			out = append(out, c)
		}
	}
	return out
}

func openOnboardComponents(ctx context.Context, orgID, projectID string, issues IssueClient) (map[string]int, error) {
	list, err := issues.ListIssues(ctx, orgID, projectID, []string{delivery.LabelOnboard})
	if err != nil {
		return nil, fmt.Errorf("onboard: list issues: %w", err)
	}
	out := map[string]int{}
	for _, iss := range list {
		if !strings.EqualFold(iss.State, "open") {
			continue
		}
		if comp := componentFromLabels(iss.Labels); comp != "" {
			out[comp] = iss.Number
		}
	}
	return out, nil
}

func (s *Service) openOnboardComponents(ctx context.Context, orgID, projectID string) (map[string]int, error) {
	return openOnboardComponents(ctx, orgID, projectID, s.issues)
}

func componentLabel(name string) string {
	slug := depSlug(name)
	if slug == "" {
		return ""
	}
	return componentLabelPrefix + slug
}

func componentFromLabels(labels []string) string {
	for _, l := range labels {
		if rest, ok := strings.CutPrefix(strings.ToLower(l), componentLabelPrefix); ok {
			return rest
		}
	}
	return ""
}

func onboardIssueTitle(component string) string {
	return "Onboard component: " + component
}

func onboardIssueBody(comp spec.DesignComponent) string {
	src := "unknown"
	if comp.Source != nil {
		src = comp.Source.Repo
		if comp.Source.Ref != "" {
			src += "@" + comp.Source.Ref
		}
		if comp.Source.Subpath != "" {
			src += " (" + comp.Source.Subpath + ")"
		}
	}
	return fmt.Sprintf("Import `%s` unmodified from `%s` into `%s`.\n\n"+
		"The platform vendors the source, opens a pull request, and closes this issue when it merges. "+
		"Build and deploy follow automatically.", comp.Name, src, comp.AppPath)
}
