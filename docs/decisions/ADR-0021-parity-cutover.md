# ADR-0021 — Parity validation is held for a human; mismatch never auto-cuts over

**Status:** Accepted · **Refines:** [ADR-0020](ADR-0020-onboarding-front-door.md)

## Context

A modernize component ships beside its `importAsIs` sibling. Validation must
assert the same e2e suite against both live endpoints and produce a diff. If
that parity PR is excluded from auto-merge, the supervisor's `readVerdict`
would wait on a merge that may never come. If cutover fired on "this looks
like a validation PR", an ordinary validation merge would tear down a
component.

`decideAutoMerge` sees no PR labels — only resolves-refs plus the milestone
issues' labels. The distinguishing signal therefore has to ride a label on
the **issue**.

## Decision

1. **Parity is an issue label, not a PR shape.** The validation issue is minted
   with `aep:parity`. `decideAutoMerge` declines when a resolved issue carries
   it (`mergeVerdict = parity-hold`). An ordinary validation merge never
   records that verdict, so it never reaches cutover.

2. **The run settles.** `awaiting-parity-review` is a deploy.validation
   lifecycle value. Cutover is a webhook-driven action decoupled from the run
   loop. The cycle stays open (`MergeSHA` empty) until the human merges; the
   merge webhook stamps the SHA.

3. **Cutover fires only on a matching diff at the merge SHA.**
   `tests/validation/parity-diff.json` `{"matched": true}` → rewrite every
   sibling `dependencies[]` naming the legacy component onto the modernize
   name, restamp endpoint wiring, tear down the legacy Component via
   `DeleteComponentCascade`, record `CutoverVerdict = complete`
   (`deploy.validation = cutover-complete`).

4. **A mismatched merge is an explicit non-event.** Unreadable or
   `"matched": false` records `CutoverVerdict = mismatch`
   (`deploy.validation = parity-mismatch-merged`) and does **not** rewrite or
   tear down. Overriding is a separate later action. The verdict lives on
   `RunCycle.CutoverVerdict` (write-once, including closed cycles) because
   `SetValidationVerdict` on the run is `updateNonTerminal` and cannot write
   after settle.

## Consequences

- Promotion is blocked by `awaiting-parity-review` and
  `parity-mismatch-merged`, not by `cutover-complete`.
- Ordinary validation merge tests must assert zero cutover calls. The
  signal is the recorded `parity-hold`, never "this is a validation PR".

## Rejected

- Waiting in the run loop for a human merge (the run would never settle).
- Inferring cutover from the PR being a validation PR.
- Auto-cutover on a mismatched merge the human landed anyway.
