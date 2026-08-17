# `specs/onboarding/analysis.json`

Facts the analysis job committed before this turn. Never a proposed component
list — `architecture` decides independently-deployable units from these.

| Field | Meaning |
|---|---|
| `sourceRepo` | Foreign repo as `owner/name`. Becomes every component's `source.repo`. |
| `ref` | Commit/ref the facts were taken at. Becomes every component's `source.ref`. |
| `languages` | Languages observed. Evidence for `language`, not a stack decision. |
| `entryPoints` | `{path, kind}` — `http`, `cli`, `worker`, … Evidence of surfaces. |
| `modules` | `{path, language, name}` — module roots. Evidence for `source.subpath`, never 1:1 with components. |
| `importGraph` | `{from, to}` package edges. Coupling evidence for the independently-deployable-unit test. |
| `routes` | `{method, path, package}` HTTP surfaces. Evidence of actors and API shape. |
| `externalCalls` | `{package, kind}` outbound SDKs/HTTP. Evidence for `external` dependencies. |
| `gaps` | What the analyzer did not understand. Leave unresolved; do not invent to fill them. |

`source.subpath` is the path of the independently-deployable unit architecture
chose, not a legacy folder copied as a component. `"."` when that unit is the
repo.
