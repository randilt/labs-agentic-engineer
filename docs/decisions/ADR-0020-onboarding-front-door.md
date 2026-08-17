# ADR-0020 — Onboarding is a second front door into the same spec pipeline

**Status:** Accepted

## Context

AEP's first front door is `/design`: a human describes a product, the design
agent produces `requirements.md`, `design.cell`, per-component `design.json` and
`validation-criteria.json`, and the milestone loop builds, deploys and validates
as if the code had never existed. That path cannot take an already-running
codebase as input.

A parallel "import then rewrite" product was tempting: a second deploy path, a
second runner image, a second validation class. All three would fork the loop
ADR-0011 and ADR-0017 made one thing.

## Decision

**Onboarding is a front door, not a pipeline.** `POST /projects/{name}/onboarding`
starts an `analysis` execution against a foreign `sourceRepoRef`. The runner
extracts facts, the platform commits `specs/onboarding/analysis.json`, and
`/onboard` walks the **same** cell-design / architecture / validation-criteria
lineup `/design` already uses.

Per-component origin is recorded on `design.json`:

- **Absence of `sourceMode`** is platform-generated. Not a default value — every
  existing design.json stays valid.
- `importAsIs` vendors the source tree byte-identically at `appPath` (missing
  Dockerfile is a blocker, never an invented file) via the ops executor, which
  opens an auto-merging PR. Build and deploy are inherited from merge →
  `fanOutBuilds` → ADR-0019 waves. The executor never calls `EnsureComponent` or
  `TriggerBuild` itself.
- `modernize` is planned and coded like any generated component. `modernizes`
  names the `importAsIs` sibling it will replace. Task planning emits **no**
  Task for an `importAsIs` component.

The ops-executor issue carries `aep:onboard` and holds dispatch the same way
`aep:provision` does. Flip-to-modernize is `/onboard <component>` from the
console, not a new API.

## Consequences

- Analysis facts are committed, not inlined into a turn prompt.
- `task ⊥ run` stays an import ban; onboard adapters live in `internal/app`.
- Import means byte-identical vendored code.

## Rejected

- A second deploy path inside the ops executor (forks ADR-0019 wave order).
- Defaulting `sourceMode` to `"generated"` (would invalidate every existing
  design.json).
- Parsing issue bodies for onboard routing (ADR-0011: every routable fact is a
  label).
