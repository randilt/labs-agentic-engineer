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
