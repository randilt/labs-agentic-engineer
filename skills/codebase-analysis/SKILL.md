---
name: codebase-analysis
description: Use when reverse-engineering a foreign source repository into structured onboarding facts for specs/onboarding/analysis.json.
metadata:
  aep:
    kind: platform
    audience: [coding]
---

# Codebase analysis

Turn the cloned foreign repository into **facts only** — never a proposed
component list. The `architecture` skill downstream decides independently
deployable units from these facts.

## Run deterministic extraction first

From the workspace root, run:

```bash
node "$AEP_SKILLS_DIR/codebase-analysis/scripts/extract-facts.mjs" \
  --source-repo "$AEP_SOURCE_REPO_REF" \
  --ref "${AEP_SOURCE_REF:-HEAD}" \
  --out /tmp/analysis-facts.json
```

The script records manifests, entry points, modules, import edges, HTTP routes,
external calls, and `gaps` for anything it could not classify. Read its output;
fill nothing the script already captured.

## Shape (committed as analysis.json)

```json
{
  "sourceRepo": "owner/name",
  "ref": "sha-or-ref",
  "languages": ["Go"],
  "entryPoints": [{"path": "cmd/server/main.go", "kind": "http"}],
  "modules": [{"path": ".", "language": "Go", "name": "example.com/app"}],
  "importGraph": [{"from": "cmd/server", "to": "internal/http"}],
  "routes": [{"method": "GET", "path": "/health", "package": "internal/http"}],
  "externalCalls": [{"package": "github.com/stripe/stripe-go", "kind": "sdk"}],
  "gaps": ["no Ballerina sources found"]
}
```

`sourceRepo` and `ref` come from `AEP_SOURCE_REPO_REF` and the checkout ref
(`AEP_SOURCE_REF` when set, else the clone's HEAD SHA). Never invent routes or
modules the extractor did not observe.

## POST facts, then exit

POST the final JSON to `AEP_ANALYSIS_CALLBACK` (or the URL in the dispatch
prompt) with the runner bearer:

```bash
curl -fsS -X POST "$AEP_ANALYSIS_CALLBACK" \
  -H "Authorization: Bearer $AEP_BEARER" \
  -H "Content-Type: application/json" \
  --data-binary @/tmp/analysis-facts.json
```

A 204 means the platform committed `specs/onboarding/analysis.json`. Fail the
run if the callback is missing, unreachable, or returns an error — do not exit
success without a committed facts file.

Do not open a pull request. Do not push. Do not edit files outside reporting
gaps in the JSON.
