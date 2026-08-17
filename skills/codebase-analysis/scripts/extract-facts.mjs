#!/usr/bin/env node
/**
 * Deterministic extraction for onboarding analysis facts.
 * Facts only — no component decomposition. Walks the tree for manifests so a
 * monorepo is not reduced to whatever sits at the repo root.
 */
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

function arg(name, fallback = "") {
  const i = process.argv.indexOf(name);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1] : fallback;
}

const sourceRepo = arg("--source-repo");
const ref = arg("--ref", "HEAD");
const out = arg("--out", "-");
const root = process.cwd();

if (!sourceRepo) {
  console.error("usage: extract-facts.mjs --source-repo owner/name [--ref sha] [--out file]");
  process.exit(2);
}

const SKIP_DIRS = new Set([
  "node_modules",
  ".git",
  "vendor",
  "dist",
  "build",
  "target",
  ".next",
  "coverage",
  "testdata",
  "__pycache__",
  ".venv",
]);

const facts = {
  sourceRepo,
  ref: resolveRef(ref),
  languages: [],
  entryPoints: [],
  modules: [],
  importGraph: [],
  routes: [],
  externalCalls: [],
  gaps: [],
};

const manifests = collectManifests();

if (manifests.goMods.length > 0) {
  facts.languages.push("Go");
  for (const file of manifests.goMods) {
    const dir = path.dirname(file);
    facts.modules.push({
      path: dir === "." ? "." : dir,
      language: "Go",
      name: readGoModule(file),
    });
    facts.externalCalls.push(...scanGoModRequires(file));
  }
  facts.entryPoints.push(...findGoEntrypoints());
  facts.importGraph.push(...scanGoImports());
  facts.routes.push(...scanGoRoutes());
}

if (manifests.packages.length > 0) {
  if (!facts.languages.includes("TypeScript")) facts.languages.push("TypeScript");
  for (const file of manifests.packages) {
    const dir = path.dirname(file);
    let pkg = {};
    try {
      pkg = JSON.parse(fs.readFileSync(path.join(root, file), "utf8"));
    } catch {
      facts.gaps.push(`unreadable package.json at ${file}`);
      continue;
    }
    facts.modules.push({
      path: dir === "." ? "." : dir,
      language: "TypeScript",
      name: pkg.name ?? (dir === "." ? "." : dir),
    });
    if (pkg.main) {
      facts.entryPoints.push({
        path: dir === "." ? pkg.main : path.join(dir, pkg.main),
        kind: "node",
      });
    }
    if (pkg.scripts?.start) {
      facts.entryPoints.push({ path: `${file}#start`, kind: "http" });
    }
  }
}

if (manifests.ballerinas.length > 0) {
  facts.languages.push("Ballerina");
  for (const file of manifests.ballerinas) {
    const dir = path.dirname(file);
    facts.modules.push({
      path: dir === "." ? "." : dir,
      language: "Ballerina",
      name: "ballerina",
    });
  }
  facts.gaps.push("Ballerina import graph not extracted in v1");
}

if (facts.languages.length === 0) {
  facts.gaps.push("no recognized manifest (go.mod, package.json, Ballerina.toml)");
}

facts.externalCalls = dedupeBy(facts.externalCalls, (c) => c.package).slice(0, 100);

const body = JSON.stringify(facts, null, 2) + "\n";
if (out === "-") process.stdout.write(body);
else fs.writeFileSync(out, body);

function collectManifests() {
  const goMods = [];
  const packages = [];
  const ballerinas = [];
  walk(".", (file) => {
    const base = path.basename(file);
    if (base === "go.mod") goMods.push(file);
    else if (base === "package.json") packages.push(file);
    else if (base === "Ballerina.toml") ballerinas.push(file);
  });
  return { goMods, packages, ballerinas };
}

function resolveRef(r) {
  try {
    return execFileSync("git", ["rev-parse", "--verify", r], {
      cwd: root,
      encoding: "utf8",
    }).trim();
  } catch {
    return r;
  }
}

function readGoModule(file) {
  const mod = fs.readFileSync(path.join(root, file), "utf8");
  const m = mod.match(/^module\s+(\S+)/m);
  return m?.[1] ?? ".";
}

function findGoEntrypoints() {
  const out = [];
  walk(".", (file) => {
    if (!file.endsWith(".go") || file.endsWith("_test.go")) return;
    const src = fs.readFileSync(path.join(root, file), "utf8");
    if (/func\s+main\s*\(/.test(src)) {
      out.push({ path: file, kind: file.includes(`${path.sep}cmd${path.sep}`) || file.startsWith("cmd/") ? "http" : "cli" });
    }
  });
  return out;
}

function scanGoImports() {
  const edges = [];
  walk(".", (file) => {
    if (!file.endsWith(".go") || file.endsWith("_test.go")) return;
    const pkg = path.dirname(file);
    const src = fs.readFileSync(path.join(root, file), "utf8");
    for (const m of src.matchAll(/^\s*"([^"]+)"/gm)) {
      const imp = m[1];
      if (imp.startsWith("github.com/") || imp.includes("/internal/")) {
        edges.push({ from: pkg === "." ? "." : pkg, to: imp });
      }
    }
  });
  return edges.slice(0, 200);
}

function scanGoRoutes() {
  const routes = [];
  walk(".", (file) => {
    if (!file.endsWith(".go")) return;
    const pkg = path.dirname(file);
    const src = fs.readFileSync(path.join(root, file), "utf8");
    for (const m of src.matchAll(/\.(?:Get|Post|Put|Delete|Patch)\(\s*"([^"]+)"/g)) {
      routes.push({ method: m[0].split(".")[1].toUpperCase().replace("(", ""), path: m[1], package: pkg });
    }
    for (const m of src.matchAll(/HandleFunc\(\s*"([^"]+)"/g)) {
      routes.push({ method: "GET", path: m[1], package: pkg });
    }
  });
  return routes.slice(0, 200);
}

function scanGoModRequires(file) {
  const mod = fs.readFileSync(path.join(root, file), "utf8");
  const out = [];
  for (const m of mod.matchAll(/^\s+(\S+)\s+v/gim)) {
    const pkg = m[1];
    if (!pkg.startsWith("github.com/wso2/")) out.push({ package: pkg, kind: "sdk" });
  }
  return out;
}

function dedupeBy(items, key) {
  const seen = new Set();
  const out = [];
  for (const item of items) {
    const k = key(item);
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(item);
  }
  return out;
}

function walk(dir, fn) {
  if (!fs.existsSync(path.join(root, dir))) return;
  for (const ent of fs.readdirSync(path.join(root, dir), { withFileTypes: true })) {
    if (SKIP_DIRS.has(ent.name) || ent.name.startsWith(".")) continue;
    const rel = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(rel, fn);
    else fn(rel);
  }
}
