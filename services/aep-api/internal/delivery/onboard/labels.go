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
	"regexp"
	"strings"
)

var labelUnsafeRE = regexp.MustCompile(`[^a-z0-9._-]+`)

func depSlug(name string) string {
	slug := labelUnsafeRE.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	return strings.Trim(slug, "-")
}

// onboardBranch returns the vendor branch for a component. It deliberately
// does NOT match ^aep/m\d+ so resolvePRRun never treats it as a cycle branch.
func onboardBranch(component string) string {
	slug := depSlug(component)
	if slug == "" {
		slug = "component"
	}
	return "aep/onboard/" + slug
}
