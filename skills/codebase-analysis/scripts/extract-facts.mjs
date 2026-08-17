#!/usr/bin/env node
/**
 * Deterministic extraction for onboarding analysis facts.
 * Facts only — no component decomposition.
 */
import { execSync } from "node:child_process";
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

if (fs.existsSync(path.join(root, "go.mod"))) {
  facts.languages.push("Go");
  facts.modules.push({ path: ".", language: "Go", name: readGoModule() });
  facts.entryPoints.push(...findGoEntrypoints());
  facts.importGraph.push(...scanGoImports());
  facts.routes.push(...scanGoRoutes());
  facts.externalCalls.push(...scanGoModRequires());
}

if (fs.existsSync(path.join(root, "package.json"))) {
  if (!facts.languages.includes("TypeScript")) facts.languages.push("TypeScript");
  const pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
  facts.modules.push({ path: ".", language: "TypeScript", name: pkg.name ?? "." });
  if (pkg.main) facts.entryPoints.push({ path: pkg.main, kind: "node" });
  if (pkg.scripts?.start) facts.entryPoints.push({ path: "package.json#start", kind: "http" });
}

if (fs.existsSync(path.join(root, "Ballerina.toml"))) {
  facts.languages.push("Ballerina");
  facts.modules.push({ path: ".", language: "Ballerina", name: "ballerina" });
  facts.gaps.push("Ballerina import graph not extracted in v1");
}

if (facts.languages.length === 0) {
  facts.gaps.push("no recognized manifest (go.mod, package.json, Ballerina.toml)");
}

const body = JSON.stringify(facts, null, 2) + "\n";
if (out === "-") process.stdout.write(body);
else fs.writeFileSync(out, body);

function resolveRef(r) {
  if (r !== "HEAD") return r;
  try {
    return execSync("git rev-parse HEAD", { encoding: "utf8" }).trim();
  } catch {
    return "HEAD";
  }
}

function readGoModule() {
  const mod = fs.readFileSync(path.join(root, "go.mod"), "utf8");
  const m = mod.match(/^module\s+(\S+)/m);
  return m?.[1] ?? ".";
}

function findGoEntrypoints() {
  const out = [];
  for (const dir of ["cmd", "."]) {
    walk(dir, (file) => {
      if (!file.endsWith(".go") || file.endsWith("_test.go")) return;
      const src = fs.readFileSync(path.join(root, file), "utf8");
      if (/func\s+main\s*\(/.test(src)) {
        out.push({ path: file, kind: file.includes("cmd/") ? "http" : "cli" });
      }
    });
  }
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

function scanGoModRequires() {
  const mod = fs.readFileSync(path.join(root, "go.mod"), "utf8");
  const out = [];
  for (const m of mod.matchAll(/^\s+(\S+)\s+v/gim)) {
    const pkg = m[1];
    if (!pkg.startsWith("github.com/wso2/")) out.push({ package: pkg, kind: "sdk" });
  }
  return out.slice(0, 100);
}

function walk(dir, fn) {
  if (!fs.existsSync(path.join(root, dir))) return;
  for (const ent of fs.readdirSync(path.join(root, dir), { withFileTypes: true })) {
    if (ent.name === "node_modules" || ent.name === ".git" || ent.name === "vendor") continue;
    const rel = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(rel, fn);
    else fn(rel);
  }
}
