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

package spec

import (
	"context"
	"encoding/json"
	"fmt"
)

// ParityDiffPath is the criterion-by-criterion legacy vs modernize report the
// validation runner commits. Cutover reads `matched` and nothing else.
const ParityDiffPath = "tests/validation/parity-diff.json"

// ModernizePair is one legacy/modernize sibling pair in a design.
type ModernizePair struct {
	Legacy    string
	Modernize string
}

// ModernizePairs returns every modernize component and the importAsIs sibling
// it names. Order follows the design. Duplicates are not collapsed — the build
// gate already refuses two modernize components claiming the same legacy.
func ModernizePairs(components []DesignComponent) []ModernizePair {
	var out []ModernizePair
	for _, c := range components {
		if !c.IsModernize() || c.Modernizes == "" {
			continue
		}
		out = append(out, ModernizePair{Legacy: c.Modernizes, Modernize: c.Name})
	}
	return out
}

// RewriteCutoverEdges rewrites every sibling-component dependency named `from`
// so it names `to`, and restamps endpoint wiring for the new name. Returns the
// number of dependency entries rewritten. Mutates components in place.
//
// This is a mechanical rename, not an architectural judgment — the same reason
// deriveDependencyWiring is safe to overwrite on every save.
func RewriteCutoverEdges(components []DesignComponent, from, to, projectID string) int {
	if from == "" || to == "" || from == to {
		return 0
	}
	endpointNames := make(map[string]string, len(components))
	for _, c := range components {
		endpointNames[c.Name] = c.EndpointName()
	}
	n := 0
	for i := range components {
		for j := range components[i].Dependencies {
			d := &components[i].Dependencies[j]
			if d.Kind != DependencyKindComponent || d.Name != from {
				continue
			}
			d.Name = to
			d.Wiring = siblingEndpointWiring(to, projectID, endpointNames)
			n++
		}
	}
	return n
}

// ParityDiffMatched reports whether a parity-diff.json payload says the two
// reports agreed. ok is false when the payload is missing, unparseable, or
// lacks the matched field — cutover treats that as "do not cut over".
func ParityDiffMatched(raw []byte) (matched, ok bool) {
	if len(raw) == 0 || !json.Valid(raw) {
		return false, false
	}
	var doc struct {
		Matched *bool `json:"matched"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Matched == nil {
		return false, false
	}
	return *doc.Matched, true
}

// RewriteCutoverEdgesAtHead applies RewriteCutoverEdges to the design at HEAD
// and commits only the components whose dependencies actually moved. A no-op
// when from is absent or nothing points at it.
func (s *designService) RewriteCutoverEdgesAtHead(ctx context.Context, orgID, projectID, from, to string) (int, error) {
	if s.fileCommitter == nil {
		return 0, fmt.Errorf("cutover: design file committer not configured")
	}
	designFile, err := s.store.ReadDesign(ctx, orgID, projectID)
	if err != nil {
		return 0, fmt.Errorf("cutover: read design: %w", err)
	}
	if designFile == nil {
		return 0, nil
	}
	n := RewriteCutoverEdges(designFile.Components, from, to, projectID)
	if n == 0 {
		return 0, nil
	}
	var writes []DesignFileWrite
	for _, comp := range designFile.Components {
		touched := false
		for _, d := range comp.Dependencies {
			if d.Kind == DependencyKindComponent && d.Name == to {
				touched = true
				break
			}
		}
		if !touched {
			continue
		}
		rendered, rerr := SplitDesign(&DesignFile{Components: []DesignComponent{comp}})
		if rerr != nil {
			return 0, fmt.Errorf("cutover: render %q: %w", comp.Name, rerr)
		}
		designSub := componentDirPrefix + comp.Name + "/design.json"
		content, ok := rendered[designSub]
		if !ok {
			return 0, fmt.Errorf("cutover: render %q: %q missing from split", comp.Name, designSub)
		}
		designFull := DesignDir + "/" + designSub
		_, sha, exists, rerr := s.fileCommitter.ReadFile(ctx, orgID, projectID, designFull)
		if rerr != nil {
			return 0, fmt.Errorf("cutover: read %q: %w", designFull, rerr)
		}
		if !exists {
			return 0, fmt.Errorf("cutover: %q missing on disk", designFull)
		}
		writes = append(writes, DesignFileWrite{Path: designFull, Content: content, BaseSHA: sha})
	}
	if len(writes) == 0 {
		return n, nil
	}
	msg := fmt.Sprintf("Cut over dependencies from %s to %s", from, to)
	if err := s.fileCommitter.Commit(ctx, orgID, projectID, writes, msg); err != nil {
		return 0, fmt.Errorf("cutover: commit design.json: %w", err)
	}
	return n, nil
}
