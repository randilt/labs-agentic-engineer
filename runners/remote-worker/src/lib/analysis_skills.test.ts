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
import {
  ANALYSIS_WORKFLOW_SKILL,
  MissingAnalysisSkillError,
  seedAnalysisSkills,
} from "./analysis_skills.js";
import { requireWorkflowBodies } from "./skills_presence.js";

function tmpDir(prefix: string): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

test("seedAnalysisSkills copies SKILL.md and scripts, skips overlays", async () => {
  const library = tmpDir("aep-lib-");
  const workspace = tmpDir("aep-ws-");
  const skill = path.join(library, ANALYSIS_WORKFLOW_SKILL);
  fs.mkdirSync(path.join(skill, "scripts"), { recursive: true });
  fs.mkdirSync(path.join(skill, "overlays"), { recursive: true });
  fs.writeFileSync(path.join(skill, "SKILL.md"), "---\nname: codebase-analysis\n---\n\nCODEWORD-ANALYSIS\n");
  fs.writeFileSync(path.join(skill, "scripts", "extract-facts.mjs"), "export default 1;\n");
  fs.writeFileSync(path.join(skill, "overlays", "local.md"), "should-not-copy\n");
  try {
    await seedAnalysisSkills(workspace, { AEP_SKILLS_LIBRARY: library });
    const dest = path.join(workspace, ".claude", "skills", ANALYSIS_WORKFLOW_SKILL);
    assert.equal(
      fs.readFileSync(path.join(dest, "SKILL.md"), "utf8").includes("CODEWORD-ANALYSIS"),
      true,
    );
    assert.ok(fs.existsSync(path.join(dest, "scripts", "extract-facts.mjs")));
    assert.equal(fs.existsSync(path.join(dest, "overlays")), false);
    const body = requireWorkflowBodies(workspace, [ANALYSIS_WORKFLOW_SKILL]);
    assert.match(body, /CODEWORD-ANALYSIS/);
  } finally {
    fs.rmSync(library, { recursive: true, force: true });
    fs.rmSync(workspace, { recursive: true, force: true });
  }
});

test("seedAnalysisSkills is fatal when the library has no analysis skill", async () => {
  const library = tmpDir("aep-lib-empty-");
  const workspace = tmpDir("aep-ws-empty-");
  try {
    await assert.rejects(
      () => seedAnalysisSkills(workspace, { AEP_SKILLS_LIBRARY: library }),
      (err: unknown) => err instanceof MissingAnalysisSkillError,
    );
  } finally {
    fs.rmSync(library, { recursive: true, force: true });
    fs.rmSync(workspace, { recursive: true, force: true });
  }
});
