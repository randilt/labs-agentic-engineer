---
name: onboard
description: Use when onboarding an existing codebase into AEP from reverse-engineered facts, or re-entering to modernize one already-onboarded component.
metadata:
  aep:
    kind: platform
    audience: [design]
---

# Onboard

The onboarding step: derive a cell-first design from reverse-engineered facts
in `specs/onboarding/analysis.json`, then mint the validation criteria. Same
downstream lineup as `/design`. The build gate checks the result mechanically,
so the way to a clean Build is to follow the order below.

## The facts are the brief

Read `specs/onboarding/analysis.json`. Design FROM those facts. Do not interview
the user for the codebase shape and do not invent what the file does not record.
A missing or empty analysis means the user needs to submit the source repo first
(the analysis job) — stop and say so.

Facts are evidence, not a decomposition. Apply the `architecture` skill's
independently-deployable-unit test; do NOT reproduce the legacy folder structure
(domain/layer splits).

The file is facts only — never a proposed component list. Schema (field notes
in this skill's `references/analysis.md`):

```json
{
  "sourceRepo": "owner/name",
  "ref": "sha",
  "languages": ["Go"],
  "entryPoints": [{"path": "cmd/server/main.go", "kind": "http"}],
  "modules": [{"path": ".", "language": "Go", "name": "example.com/app"}],
  "importGraph": [{"from": "cmd/server", "to": "internal/http"}],
  "routes": [{"method": "GET", "path": "/health", "package": "internal/http"}],
  "externalCalls": [{"package": "github.com/stripe/stripe-go", "kind": "sdk"}],
  "gaps": ["no Ballerina sources found"]
}
```

`gaps` are reported unknowns — leave them unresolved rather than filling them.

When no PRD exists, these facts are the requirement the lineup below reads:
actors, routes, and external systems come from the facts, never from invention.

## The lineup

Each step names the skill that governs it. Those bodies are inlined for this
turn — apply them directly, and load one only if you find you do not have it.

1. **design.cell** (`cell-design`) — emit the cell FIRST: every component,
   boundaries and edges. The console streams it into the live diagram, and
   the platform scaffolds a design.json skeleton per deployable component
   when it lands.
2. **Component enrichment** (`architecture`) — fill each
   component's design.json: language (org Tech stack default first), the PRD
   `stories` it serves (every story the PRD defines must be claimed by some
   component — the build gate checks coverage), dependencies (discover before
   you invent), description, pinned skills.
3. **design.md** — a DIAGRAM document, mermaid throughout: one Overview
   paragraph, then `## Context (C1)` (a mermaid graph: the PRD's actors, the
   system, external systems), `## Domain model (ER)` (a mermaid erDiagram:
   entities, key fields, relations — these become the API schemas), and
   `## Key flows` (one mermaid sequenceDiagram per core workflow). No
   Components or Interactions prose — the cell owns C2.
4. **security.md** (`security-design`) — when the design has sign-in or roles.
5. **Per-component artifacts** — every `service` gets `openapi.yaml`
   (`openapi-conventions`); every `web-application` gets `wireframes.dsl`
   (`wireframes`).
6. **Validation criteria** (`validation-criteria`) — mint
   `specs/validation/validation-criteria.json` LAST. A design without its
   acceptance oracle is unfinished — never skip this.

Order binds only where a step reads an earlier one's result: the cell before
enrichment (the platform scaffolds each design.json from it), and design.md's
ER model before `openapi.yaml` (those entities become the API schemas).
Everything else is independent — emit independent artifacts as parallel calls
in ONE step, not a step each.

`importAsIs` components skip generated `openapi.yaml` / `wireframes.dsl` — the
vendored code brings its own. A `modernize` component takes the full artifact
path. Sign-in/roles for `security-design` are read off the facts (auth routes,
IdP calls) or the design already in the snapshot.

## Source mode

Default every component `sourceMode: "importAsIs"` with `source: { repo, ref,
subpath }` taken from the facts: `repo` is `sourceRepo`, `ref` is `ref`,
`subpath` is the path of that independently-deployable unit in the source repo
(`"."` when the unit is the repo). Authored once; never recomputed.

## Modernize pair

When the user/instruction explicitly names a component for modernization, emit
TWO design components for that one piece of functionality — never a single
component that tries to be both.

**Naming:** keep the architecture-chosen kebab-case id on the legacy component.
Name the modernize sibling `<id>-next`. Do not rename the legacy component with
a `-legacy` suffix. Do not reuse the legacy id for the modernize sibling.

- **legacy** — original id. `sourceMode: "importAsIs"`, original source pointer.
- **modernize** — `<id>-next`. `sourceMode: "modernize"`, same source pointer,
  `modernizes: <legacy-id>`.

`modernizes` is a `design.json` fact, not a cell edge — both components appear
in `design.cell`. A legacy component may have only one modernize sibling.

## Re-entry — modernizing a named component on request

A later turn may name one already-onboarded component — "modernize
expense-api". It carries no extra JSON: read that component's `design.json`
from the snapshot and act on its current state.

- **Currently `importAsIs`, no sibling yet claims it via `modernizes`.** Keep
  the existing importAsIs component as it is. Mint the `<id>-next` sibling as
  above. Update `design.cell` (the new component exists), enrich only the new
  sibling, emit its artifacts, remint validation criteria. Do not rewrite the
  legacy component in place.
- **Already paired, or already `modernize`.** Stop and say so — do not mint a
  second modernize sibling.
- **No such component.** Stop and say so.

Edit only what this scoped change requires. Other components stay as they were.

A design already exists and this is not a scoped modernize → CONVERGE it to
the current facts: update what drifted, keep what holds.

## Where this stops

`/onboard` ends at the design and its validation criteria — no task planning,
no application code. Never touch git outside the spec bundle. Never call
OpenChoreo. Close with three parts and nothing more: one line per component
(name, type, one-clause role, sourceMode); a **"Needs your input"** block
listing only the dependencies still ambiguous or unresolved; and a one-line
pointer to `specs/design/`. The dependency narration during the turn (the
`architecture` skill owns its format) already carried the play-by-play.
