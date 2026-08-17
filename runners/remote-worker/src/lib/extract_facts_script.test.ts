/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const SCRIPT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../../skills/codebase-analysis/scripts/extract-facts.mjs",
);

test("extract-facts.mjs records nested go.mod and package.json modules", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "aep-facts-"));
  try {
    fs.mkdirSync(path.join(root, "services", "orders"), { recursive: true });
    fs.mkdirSync(path.join(root, "apps", "web"), { recursive: true });
    fs.writeFileSync(path.join(root, "services", "orders", "go.mod"), "module example.com/orders\n\nrequire github.com/stripe/stripe-go v1.0.0\n");
    fs.writeFileSync(
      path.join(root, "services", "orders", "main.go"),
      'package main\nfunc main() {}\n',
    );
    fs.writeFileSync(
      path.join(root, "apps", "web", "package.json"),
      JSON.stringify({ name: "web", main: "index.js", scripts: { start: "node index.js" } }),
    );
    const stdout = execFileSync(process.execPath, [SCRIPT, "--source-repo", "acme/shop", "--ref", "abc"], {
      cwd: root,
      encoding: "utf8",
    });
    const facts = JSON.parse(stdout);
    assert.deepEqual(facts.languages.sort(), ["Go", "TypeScript"]);
    const mods = facts.modules.map((m) => `${m.language}:${m.path}:${m.name}`).sort();
    assert.ok(mods.includes("Go:services/orders:example.com/orders"));
    assert.ok(mods.includes("TypeScript:apps/web:web"));
    assert.equal(facts.sourceRepo, "acme/shop");
    assert.equal(facts.ref, "abc");
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("extract-facts.mjs resolves a git ref to a SHA", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "aep-facts-git-"));
  try {
    execFileSync("git", ["init"], { cwd: root });
    execFileSync("git", ["config", "user.email", "test@example.com"], { cwd: root });
    execFileSync("git", ["config", "user.name", "test"], { cwd: root });
    fs.writeFileSync(path.join(root, "go.mod"), "module example.com/app\n");
    execFileSync("git", ["add", "go.mod"], { cwd: root });
    execFileSync("git", ["commit", "-m", "init"], { cwd: root });
    const sha = execFileSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" }).trim();
    const stdout = execFileSync(process.execPath, [SCRIPT, "--source-repo", "acme/app", "--ref", "HEAD"], {
      cwd: root,
      encoding: "utf8",
    });
    assert.equal(JSON.parse(stdout).ref, sha);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
