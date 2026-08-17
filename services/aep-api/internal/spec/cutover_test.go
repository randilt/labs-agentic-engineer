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

import "testing"

func TestRewriteCutoverEdges_RenamesComponentDepsAndWiring(t *testing.T) {
	comps := []DesignComponent{
		{
			Name: "web",
			Dependencies: []Dependency{
				{Kind: DependencyKindComponent, Name: "legacy-api"},
				{Kind: DependencyKindPlatformResource, Name: "db"},
			},
		},
		{
			Name:         "legacy-api",
			SourceMode:   SourceModeImportAsIs,
			Endpoint:     &ComponentEndpoint{Name: "http"},
			Dependencies: []Dependency{},
		},
		{
			Name:         "new-api",
			SourceMode:   SourceModeModernize,
			Modernizes:   "legacy-api",
			Endpoint:     &ComponentEndpoint{Name: "http"},
			Dependencies: []Dependency{},
		},
	}
	n := RewriteCutoverEdges(comps, "legacy-api", "new-api", "shop")
	if n != 1 {
		t.Fatalf("rewrote %d, want 1", n)
	}
	if comps[0].Dependencies[0].Name != "new-api" {
		t.Fatalf("web dep = %q, want new-api", comps[0].Dependencies[0].Name)
	}
	if comps[0].Dependencies[1].Name != "db" {
		t.Fatalf("platform-resource dep must be untouched, got %q", comps[0].Dependencies[1].Name)
	}
	w := comps[0].Dependencies[0].Wiring
	if w == nil || w.Endpoint == nil {
		t.Fatal("rewritten dep must carry restamped endpoint wiring")
	}
	if want := "shop-new-api"; w.Endpoint.Component != want {
		t.Fatalf("wiring.component = %q, want %q", w.Endpoint.Component, want)
	}
}

func TestRewriteCutoverEdges_NoopWhenNothingPointsAtLegacy(t *testing.T) {
	comps := []DesignComponent{{
		Name:         "solo",
		Dependencies: []Dependency{{Kind: DependencyKindComponent, Name: "other"}},
	}}
	if n := RewriteCutoverEdges(comps, "legacy-api", "new-api", "shop"); n != 0 {
		t.Fatalf("rewrote %d, want 0", n)
	}
	if comps[0].Dependencies[0].Name != "other" {
		t.Fatalf("untouched dep renamed to %q", comps[0].Dependencies[0].Name)
	}
}

func TestParityDiffMatched(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantMatched bool
		wantOK      bool
	}{
		{"matched", `{"schemaVersion":1,"matched":true}`, true, true},
		{"mismatch", `{"schemaVersion":1,"matched":false}`, false, true},
		{"empty", "", false, false},
		{"garbage", "{not json", false, false},
		{"missing field", `{"schemaVersion":1}`, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			matched, ok := ParityDiffMatched([]byte(c.raw))
			if matched != c.wantMatched || ok != c.wantOK {
				t.Fatalf("got (%v, %v), want (%v, %v)", matched, ok, c.wantMatched, c.wantOK)
			}
		})
	}
}

func TestModernizePairs(t *testing.T) {
	got := ModernizePairs([]DesignComponent{
		{Name: "legacy-api", SourceMode: SourceModeImportAsIs},
		{Name: "new-api", SourceMode: SourceModeModernize, Modernizes: "legacy-api"},
		{Name: "generated"},
	})
	if len(got) != 1 || got[0].Legacy != "legacy-api" || got[0].Modernize != "new-api" {
		t.Fatalf("got %+v", got)
	}
}
