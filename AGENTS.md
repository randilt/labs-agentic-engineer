# AGENTS.md — AEP (root)

Agentic Engineer Platform: a polyglot (Go + TypeScript) monorepo. Spec-driven
SDLC platform built on OpenChoreo. 

## Uniform commands (single entry point: the root `Makefile`)

| Verb | Command | Does |
|---|---|---|
| install | `make install` | pnpm install + `go work sync` |
| build | `make build` | turbo build (TS) + `go build` (go.work) — runs `gen` first |
| dev | `make dev` | start dev servers |
| test | `make test` | turbo test + `go test` |
| lint | `make lint` | eslint + golangci-lint |
| typecheck | `make typecheck` | `tsc` + `go vet` |
| license-check | `make license-check` | fail if any source lacks the Apache header |

## Development Practices
- Focus on writing maintainable code, clean code. 
- Keep files seperated based on responsibility.
- Proper Fix alawys, no hacks or workarounds unless explicitly specified.
- Dead code is gated. TS: `make deadcode-ts-check` (knip over `@aep/agents` +
  `@aep/playground`; `make deadcode-ts` for a report). Retain unwired infra / a
  deliberate test seam with a `@knipkeep <reason>` JSDoc tag; config + rationale
  in `knip.jsonc`. Go is gated per-module (`services/aep-api`, `//deadcode:keep`).

## PR Guidelines
- Make sure tests are enough to prove the change works as expected.
- Make sure to run /code-review before submitting a PR and then run tests again.
- Include proof of real execution in the PR Description (screenshots, test case from real payload, etc.)
- See if documentation(such as README, ADR, etc.) needs to be updated and update it accordingly.
- The documentation(including comments) should fit the overall project scope and should not be biased towards the specific PR. 

## Design docs

Each package keeps a `design/` folder: concise notes + ADRs written **after** a
feature ships (final state, not plans). Repo-wide ADRs/overview live in `docs/`.

Implementation plans can go to `docs/design/draft` but they should not be commited, once the feature is implemented, the relavant information should fit into package documentation and the draft should be deleted.

## More

`docs/architecture.md` (overview), `docs/decisions/` (ADRs), `docs/glossary.md`
(domain terms), `docs/developer-guide/` (setup/dev flow).

## Cursor Cloud specific instructions

The VM update script runs `make install` + `make tools` + `make gen`, so deps,
`golangci-lint`, and generated code are already in place at session start.

- **Generated code is git-ignored, not committed** (e.g.
  `apps/console/src/generated/aep-api.d.ts`, the TanStack route tree). A fresh
  checkout has none of it. `make build`/`make test`/`make typecheck` run `gen`
  first automatically, but `make dev` (and `pnpm --filter … dev`) do NOT — run
  `make gen` before starting a dev server or the import of generated modules
  fails. The update script already runs `make gen` once.
- **Run the console cluster-free (the practical e2e harness):**
  `VITE_API_MODE=mock pnpm --filter @aep/console dev` → http://localhost:8090.
  `VITE_API_MODE=mock` implies mock auth (no Thunder/OIDC) and MSW-served APIs,
  so no backend/Postgres/k3d is needed. Onboarding + "Create project" work fully
  in mock mode. Mock scenarios: `localStorage['aep:mock:projects'] =
  'empty'|'some'|'error'`. Vite runs no `tsc`, so the dev server starts even
  when `tsc` typecheck fails.
- **The full platform (real BFF + build/deploy) needs Docker + k3d + OpenChoreo
  + Thunder + Temporal + OpenBao** (`deployments/scripts/setup.sh` then
  `start.sh`). Docker is NOT installed on the cloud VM and this stack is heavy —
  it is out of scope for the default cloud setup. Use console mock mode and the
  `playground` (`pnpm play`, needs `ANTHROPIC_API_KEY`) for cluster-free work.
- **Go toolchain:** `go.work`/modules target go 1.26.0 and the system `go`
  auto-downloads it via `GOTOOLCHAIN` on first use — expect a one-time download.
