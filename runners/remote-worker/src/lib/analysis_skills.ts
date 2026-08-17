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

// Analysis pods clone a FOREIGN source repo, which has no `.claude/skills/`
// mirror. ADR-0005 forbids an image-library fallback on an AEP project clone
// (org edits would be silently discarded). An analysis workspace is not that
// clone — the only skill it needs is the platform `codebase-analysis` body
// (and its extractor script), which never rides an org library edit.
//
// Copy from the image library (`/app/skills`, overridable via AEP_SKILLS_LIBRARY
// for tests and the playground bind-mount) into the workspace after clone.
// `overlays/` is withheld, same as every other mirror writer.

import fs from "node:fs";
import path from "node:path";
import { SKILLS_MIRROR_DIR } from "./skills_presence.js";

export const ANALYSIS_WORKFLOW_SKILL = "codebase-analysis";

const OVERLAYS_DIR = "overlays";

export function analysisSkillsLibraryDir(env: NodeJS.ProcessEnv = process.env): string {
  const override = env.AEP_SKILLS_LIBRARY?.trim();
  return override && override !== "" ? override : "/app/skills";
}

export class MissingAnalysisSkillError extends Error {
  constructor(libraryDir: string, skill: string) {
    super(
      `analysis skill ${skill} is missing from ${libraryDir} — the runner image did not receive the skills library. Refusing to start.`,
    );
    this.name = "MissingAnalysisSkillError";
  }
}

/**
 * Materialise `codebase-analysis` (SKILL.md + scripts/) into the foreign clone
 * so requireWorkflowBodies and the extractor path both resolve.
 */
export async function seedAnalysisSkills(
  workspace: string,
  env: NodeJS.ProcessEnv = process.env,
): Promise<void> {
  const library = analysisSkillsLibraryDir(env);
  const src = path.join(library, ANALYSIS_WORKFLOW_SKILL);
  const skillMd = path.join(src, "SKILL.md");
  try {
    await fs.promises.access(skillMd, fs.constants.R_OK);
  } catch {
    throw new MissingAnalysisSkillError(library, ANALYSIS_WORKFLOW_SKILL);
  }
  const dest = path.join(workspace, SKILLS_MIRROR_DIR, ANALYSIS_WORKFLOW_SKILL);
  await fs.promises.cp(src, dest, {
    recursive: true,
    filter: (p) => path.basename(p) !== OVERLAYS_DIR,
  });
}
