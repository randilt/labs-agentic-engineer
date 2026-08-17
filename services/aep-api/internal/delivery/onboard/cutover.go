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

	"github.com/wso2/aep/aep-api/internal/delivery"
	"github.com/wso2/aep/aep-api/internal/spec"
)

// CutoverInput is the event plane's hand-off after a human merged a parity PR.
type CutoverInput struct {
	OrgID, ProjectID, MergeSHA, CycleID string
	IssueNumber                         int
}

// OnParityMerged is the cutover decision: read the merged commit's parity
// diff, and only if the reports matched rewrite dependency edges and tear down
// each legacy Component. A mismatch is recorded and does not cut over.
func (s *Service) OnParityMerged(ctx context.Context, in CutoverInput) error {
	if in.OrgID == "" || in.ProjectID == "" || in.MergeSHA == "" {
		return nil
	}
	matched, ok := s.parityMatched(ctx, in)
	if !ok || !matched {
		s.recordCutover(ctx, in.CycleID, delivery.CycleCutoverMismatch)
		slog.InfoContext(ctx, "onboard: parity mismatch merged; cutover skipped",
			"project", in.ProjectID, "sha", delivery.ShortSHA(in.MergeSHA), "readable", ok)
		return nil
	}
	if s.design == nil {
		return fmt.Errorf("onboard: design reader not configured")
	}
	comps, err := s.design.ReadDesignComponents(ctx, in.OrgID, in.ProjectID)
	if err != nil {
		return fmt.Errorf("onboard: read design: %w", err)
	}
	pairs := spec.ModernizePairs(comps)
	if len(pairs) == 0 {
		s.recordCutover(ctx, in.CycleID, delivery.CycleCutoverComplete)
		return nil
	}
	for _, pair := range pairs {
		if s.edges != nil {
			if _, rerr := s.edges.RewriteCutoverEdges(ctx, in.OrgID, in.ProjectID, pair.Legacy, pair.Modernize); rerr != nil {
				return fmt.Errorf("onboard: rewrite %s → %s: %w", pair.Legacy, pair.Modernize, rerr)
			}
		}
		if err := s.TeardownComponent(ctx, in.OrgID, in.ProjectID, pair.Legacy, in.IssueNumber); err != nil {
			return fmt.Errorf("onboard: teardown %s: %w", pair.Legacy, err)
		}
	}
	s.recordCutover(ctx, in.CycleID, delivery.CycleCutoverComplete)
	slog.InfoContext(ctx, "onboard: cutover complete",
		"project", in.ProjectID, "pairs", len(pairs), "sha", delivery.ShortSHA(in.MergeSHA))
	return nil
}

func (s *Service) parityMatched(ctx context.Context, in CutoverInput) (matched, ok bool) {
	if s.reports == nil {
		return false, false
	}
	raw, err := s.reports.ReadAt(ctx, in.OrgID, in.ProjectID, spec.ParityDiffPath, in.MergeSHA)
	if err != nil {
		slog.WarnContext(ctx, "onboard: read parity-diff.json failed", "error", err, "sha", delivery.ShortSHA(in.MergeSHA))
		return false, false
	}
	return spec.ParityDiffMatched([]byte(raw))
}

func (s *Service) recordCutover(ctx context.Context, cycleID, verdict string) {
	if s.cycles == nil || cycleID == "" {
		return
	}
	if err := s.cycles.SetCutoverVerdict(ctx, cycleID, verdict); err != nil {
		slog.WarnContext(ctx, "onboard: record cutover verdict failed", "cycle", cycleID, "verdict", verdict, "error", err)
	}
}
