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

import "testing"

func TestParseSourceRepoRef(t *testing.T) {
	got, err := ParseSourceRepoRef("wso2/example")
	if err != nil {
		t.Fatal(err)
	}
	if got.OwnerName != "wso2/example" || got.CloneURL != "https://github.com/wso2/example" || got.Ref != "" {
		t.Fatalf("got %+v", got)
	}
	got, err = ParseSourceRepoRef("acme/app@deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ref != "deadbeef" {
		t.Fatalf("ref = %q", got.Ref)
	}
	if _, err := ParseSourceRepoRef("bad"); err == nil {
		t.Fatal("expected error for bad ref")
	}
}
