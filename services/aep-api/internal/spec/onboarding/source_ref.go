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

package onboarding

import (
	"fmt"
	"strings"
)

// ParsedSourceRepo is a foreign repo reference for analysis dispatch.
type ParsedSourceRepo struct {
	OwnerName string // owner/repo
	Ref       string // optional; empty → default branch at clone time
	CloneURL  string // https://github.com/owner/repo
}

// ParseSourceRepoRef parses `owner/name` or `owner/name@ref`.
func ParseSourceRepoRef(ref string) (ParsedSourceRepo, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ParsedSourceRepo{}, fmt.Errorf("empty sourceRepoRef")
	}
	ownerName := ref
	var pin string
	if at := strings.LastIndex(ref, "@"); at > 0 {
		ownerName = ref[:at]
		pin = strings.TrimSpace(ref[at+1:])
	}
	parts := strings.Split(ownerName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ParsedSourceRepo{}, fmt.Errorf("expected owner/name or owner/name@ref, got %q", ref)
	}
	if strings.ContainsAny(parts[0]+parts[1], " \t") {
		return ParsedSourceRepo{}, fmt.Errorf("owner/name must not contain whitespace")
	}
	return ParsedSourceRepo{
		OwnerName: ownerName,
		Ref:       pin,
		CloneURL:  "https://github.com/" + ownerName,
	}, nil
}
