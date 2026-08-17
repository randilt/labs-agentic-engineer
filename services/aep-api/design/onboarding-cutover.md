# Onboarding cutover

Final shape after P9. The event plane detects a merged parity-hold PR
(including after the run has settled); `delivery/onboard.OnParityMerged`
decides. Ordinary validation merge never reaches it — the signal is
`RunCycle.MergeVerdict == parity-hold`, not "this looks like validation".

On `"matched": true` in `tests/validation/parity-diff.json` at the merge SHA:
rewrite `dependencies[]` (and endpoint wiring) from the legacy name onto the
modernize name, then `DeleteComponentCascade` the legacy sibling. On mismatch
or an unreadable diff: record `CutoverVerdict = mismatch` and stop.

Wiring is composition-root only (`internal/app` adapters). `task ⊥ run`
stays an import ban. See [ADR-0020](../../../docs/decisions/ADR-0020-onboarding-front-door.md)
and [ADR-0021](../../../docs/decisions/ADR-0021-parity-cutover.md).

## Two rules the platform enforces rather than delegates

**`owner/name@ref` means the same thing on both halves.** Analysis pins the
clone through `checkoutSourceRef` (runners/remote-worker), which accepts a
branch, a tag, or a commit SHA. Vendoring resolves the ref the same way —
shallow clone, then checkout, falling back to `fetch origin <ref>` + detach.
`clone --branch` is not usable here: it rejects commit SHAs, so the two halves
would disagree about which refs a design may pin.

**An `importAsIs` component never receives a coding Task.** `skills/task-planning`
says so, but the plan tap (`delivery/task/plan_tap.go`) refuses the write too,
reading the vendored set off the design. A planner that slipped would otherwise
put an agent to work editing imported source — the one thing import-as-is
exists to prevent. A refused plan is a skip, not a write failure.
