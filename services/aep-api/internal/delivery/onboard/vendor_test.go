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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepo builds a source repo with two commits on the default branch and a
// tag on the first, and returns its path plus the first commit's SHA.
//
// allowAnySHA1InWant mirrors GitHub, which serves `git fetch origin <sha>` for
// any reachable commit — the capability checkoutRef's fallback depends on, and
// the same one runners/remote-worker's checkoutSourceRef relies on for analysis.
func fixtureRepo(t *testing.T) (dir, firstSHA string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	git("init", "--initial-branch=main")
	git("config", "uploadpack.allowAnySHA1InWant", "true")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("Dockerfile", "FROM scratch\n")
	write("marker.txt", "first")
	git("add", ".")
	git("commit", "-m", "first")
	firstSHA = git("rev-parse", "HEAD")
	git("tag", "v1.0.0")

	write("marker.txt", "second")
	git("add", ".")
	git("commit", "-m", "second")
	return dir, firstSHA
}

// A commit SHA is a legal `owner/name@ref` — the onboarding analysis half
// accepts one and pins the clone to it, so vendoring must resolve the same ref
// to the same tree. `git clone --branch <sha>` cannot: it takes branch and tag
// names only, which used to fail the vendor step with "Remote branch not found"
// for exactly the refs analysis had just succeeded on.
func TestCloneForeignRepo_PinsBranchTagAndCommitSHA(t *testing.T) {
	src, firstSHA := fixtureRepo(t)

	for _, tc := range []struct {
		name string
		ref  string
		want string
	}{
		{"no ref stays on the default branch", "", "second"},
		{"branch name", "main", "second"},
		{"tag", "v1.0.0", "first"},
		{"commit SHA", firstSHA, "first"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, cleanup, err := cloneForeignRepo(context.Background(), src, tc.ref, nil)
			if err != nil {
				t.Fatalf("clone %q: %v", tc.ref, err)
			}
			defer cleanup()

			got, err := os.ReadFile(filepath.Join(dir, "marker.txt"))
			if err != nil {
				t.Fatalf("read marker: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("marker = %q, want %q — clone did not pin to ref %q", got, tc.want, tc.ref)
			}
		})
	}
}

// A ref that exists nowhere must fail the clone rather than silently vendoring
// the default branch: the design pinned a specific tree and the human needs to
// hear that it is gone.
func TestCloneForeignRepo_UnknownRefFails(t *testing.T) {
	src, _ := fixtureRepo(t)

	dir, cleanup, err := cloneForeignRepo(context.Background(), src, "no-such-ref", nil)
	if err == nil {
		cleanup()
		t.Fatalf("unknown ref cloned into %s, want an error", dir)
	}
	if !strings.Contains(err.Error(), "no-such-ref") {
		t.Errorf("error does not name the missing ref: %v", err)
	}
}

func TestDockerfilePresent(t *testing.T) {
	dir := t.TempDir()
	if dockerfilePresent(dir) {
		t.Fatal("empty dir reported a Dockerfile")
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !dockerfilePresent(dir) {
		t.Error("Dockerfile at root not detected")
	}
}

func TestDefaultImportAsIsDockerfile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		language, componentType, want string
	}{
		{"Ballerina", "service", "ballerina/ballerina:"},
		{"Go", "service", "golang:1.25-alpine"},
		{"TypeScript", "web-application", "npm run build"},
		{"JavaScript", "service", `"npm", "start"`},
	} {
		t.Run(tc.language+"/"+tc.componentType, func(t *testing.T) {
			body, err := defaultImportAsIsDockerfile(tc.language, tc.componentType)
			if err != nil {
				t.Fatalf("synthesize: %v", err)
			}
			got := string(body)
			if !strings.Contains(got, tc.want) {
				t.Errorf("Dockerfile missing %q:\n%s", tc.want, got)
			}
			if !strings.Contains(got, "EXPOSE 9090") {
				t.Error("Dockerfile does not EXPOSE 9090")
			}
		})
	}

	if _, err := defaultImportAsIsDockerfile("Python", "service"); err == nil {
		t.Fatal("expected error for unsupported language")
	}
}
