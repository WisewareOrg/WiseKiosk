# 0029 — Render the API explorer with a build-fetched Swagger UI, interactive locally

**Status:** accepted
**Decided:** 2026-09-09 (#195 API explorer implementation, owner-ruled the same day)
**Rev:** 2

## Revisions

- **rev 2** — 2026-09-15 — drops "drift-gated" from the parenthetical describing
  `boundary.gen.go`: `check-boundary` is deleted, and the file is `oapi-codegen` output generated at
  build rather than a committed, drift-gated artifact
  ([ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md)). What this ADR decides is unchanged,
  so the `Decided` date does not move (#339 generated-code-not-committed).
- **rev 1** — 2026-09-09 — first written (#195 API explorer).

## Context

`boundary/openapi.yaml` ([ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md)) is the whole
wire contract, but it had no browsable or interactive rendering — a developer read the YAML by hand.
ADR 0008 rev 6 withdrew the docsite-renderer decision and delegated it to #195, deferred until real
content existed; `boundary/openapi.yaml` now carries `/healthz` and `/api/weather`, so that deferral
has lapsed.

## Decision

Render the schema with [Swagger UI](https://github.com/swagger-api/swagger-ui), fetched at build time
as an npm devDependency (`swagger-ui-dist`, pinned) in a new `docs/site/` npm silo, and copied into the
Sphinx build output by `docs/site/copy_explorer_assets.py` (`html_static_path`, gitignored, regenerated
every build). **Swagger UI's source is never committed** — no `.js`/`.css`/`.map` is ever git-tracked;
the built site's `_static/swagger-ui/` and `_static/openapi.yaml` are pure build output, the same way
`docs/site/generated/` (the sphinx-needs pages) already is.

This needs no new rev of [ADR 0017 rev 9](0017-authored-language-set.md): it already carves out
"documentation, and the assets a build serves" from the authored-language-set decision entirely — a
build-fetched, build-copied bundle a docs build serves is exactly that carve-out, not a new authored
JS/CSS surface.

**"Try it out" is interactive locally, and reference-only on the published site — self-contained in
the docs/site silo, with no frontend involvement.** `docs/site/vite.config.ts` is the docs silo's own
dev server: `vite` is a devDependency in the same `docs/site` npm silo `swagger-ui-dist` already lives
in (the obvious reuse — the silo has an npm silo already, and Vite trivially does both static serving
and proxying, the two things this needs). It serves the *whole* built docs site at its root — every
page, `_static` asset and the search index, not the explorer alone — and proxies `/api`,`/healthz` to
the backend's fixed `:8080` ([ADR 0020 rev 5](0020-release-artifact-set-and-operator-tooling.md)): a
plain reverse proxy, no CORS header needed because the browser only ever talks to the docs server's
own origin. `just docs-serve` is the one command that builds the docs site and launches this server
(fixed port `5174`, so it never collides with the frontend's own dev server), so there is no separate
build-then-serve step to remember. **`frontend/` carries none of this** — no plugin, no config change,
zero docs knowledge — because the explorer's self-containment in `docs/site` is the silo's whole point:
a developer who never touches `frontend/` can still get the interactive explorer, and a change to
either silo cannot break the other. `curl`, the generated boundary client and now the explorer page all
reach the backend the same, unmodified way `just serve` already exposes it. No backend code changed, no
requirement or gate touched. On the published static Pages site — no backend behind it at all — the
explorer stays browsable-only, which is not a limitation of this change but of what a static site can
ever do.

One accepted duplication: the proxy target `http://localhost:8080` is now hardcoded in *two*
independent places — this file and `frontend/vite.config.ts`'s own, unrelated dev proxy (which exists
for the frontend's own reasons, reaching the same backend the same way). Both already just restate the
port ADR 0020 rev 5 fixes in the Go binary; left as two literals rather than inventing shared machinery
across two otherwise-unrelated dev-tooling silos for one number that is not expected to change.

## Alternatives considered

- **Hard-committing Swagger UI's source.** Would make `.js`/`.css`/`.map` tracked, tripping
  `check-languages.py` (no `.js` extension is declared at all — a hand-committed bundle fails outright)
  and `check-added-large-files`, forcing a language-gate carve-out. Rejected precisely *because* it
  would weaken a gate for this feature's sake; the build-fetch route avoids that entirely, confirmed by
  `git ls-files` carrying zero `swagger-ui-dist` files after implementation.
- **sphinxcontrib-openapi.** Renders the schema browsably from within Sphinx with no extra JS, but has
  no "Try it out" affordance of any kind — fails the interactive half of the ask outright.
- **Scalar.** Also interactive, but at decision time its dist was roughly double Swagger UI's size, with
  materially more frequent releases (near-daily vs. weekly) and no established advisory track record in
  this project's dependency-scanning experience — more vendored surface to re-vet for no rendering
  capability Swagger UI lacks here.
- **Standalone, cross-origin docs server reaching the backend directly.** Serve the built docs site
  from its own static-file origin and point Swagger UI straight at the backend's fixed `:8080`.
  Rejected: the backend sends no `Access-Control-Allow-Origin` header anywhere
  (`backend/internal/headers.Wrap`, the sole place response headers are set) — confirmed empirically
  against the real handler and via a real-browser cross-origin `fetch()`, both failing for `GET
  /healthz` and `POST /api/weather`. Fixing it would need a backend CORS capability this docs ticket
  has no business adding: there is no existing dev/prod switch in the backend at all, and
  [ADR 0020 rev 5](0020-release-artifact-set-and-operator-tooling.md) already closes "the operator
  interface to the binary is its flags, and there are two" — a new gate would need a rev to that
  decision, not just code; `headers.Wrap` is itself requirement-cited
  (SRS010<!-- The display page reaches no origin but the backend's -->,
  SRS027<!-- The display page holds no device capability it does not use -->,
  SRS028<!-- Served responses declare their type, and forbid the browser inferring one -->), so a new
  observable header behavior there reads as a new obligation under
  [ADR 0011 rev 2](0011-requirement-or-convention.md);
  and the OPTIONS preflight `/api/weather` needs cannot be added to the generated router
  (`backend/internal/boundary/boundary.gen.go`, `oapi-codegen` output generated at build) without
  either
  forking `boundary/openapi.yaml` or a second, separate bypass-middleware mechanism ahead of it.
- **Serving the explorer through the backend itself, via its existing `-static-root` flag** (pointed at
  the built docs site instead of `frontend/dist` — the same one-origin composition `just
  run`/`run-container` already use for the compiled frontend bundle, needing no new flag). This avoids
  CORS entirely by being genuinely same-origin, but fails on a different wall: `headers.Wrap` sets a
  strict Content-Security-Policy (`script-src 'self'; style-src 'self'; img-src 'self'`, no
  `unsafe-inline`, no `unsafe-eval`, no `data:`) over the *entire* mux, including the static handler.
  Confirmed empirically (real backend, real headless-browser render): Swagger UI never renders at all
  under it — its own inline init script, the inline styles it injects at runtime, its `data:` SVG icons
  and an internal `eval`/`new Function()` call are all blocked. Loosening that CSP is a backend change
  to the same requirement-cited function as above
  (SRS010<!-- The display page reaches no origin but the backend's -->,
  SRS027<!-- The display page holds no device capability it does not use -->,
  SRS028<!-- Served responses declare their type, and forbid the browser inferring one -->), on the
  identical binary that runs the physically deployed kiosk — out of scope here for the same reason the
  CORS route is.
- **A frontend-coupled Vite dev-plugin** (the first cut of this ADR): reuse `frontend/`'s own Vite dev
  server, adding a dev-only plugin there that mounts the built docs site and rides the frontend's
  existing `/api` proxy. This worked technically — verified end to end, no CORS, no CSP issue, since
  Vite's dev server sets neither — but it was rejected on self-containment grounds: the interactive
  explorer is a docs/site concern, and self-containment in that silo is the silo's own purpose. Coupling
  it to `frontend/`'s dev server gives `frontend/` docs knowledge it has no reason to carry, and ties
  the explorer's availability to a package that isn't the one that builds or owns it.

Four interactive-delivery options were considered; the docs-owned Vite server was chosen because it is
the only one that needs no backend change **and** keeps the explorer entirely inside the silo whose
purpose is to own it.

## Consequences

Makes the boundary contract browsable from the docs site, and interactive against a real backend for
local development, entirely from within `docs/site/` — at the cost of `vite` joining `swagger-ui-dist`
as a second devDependency in that silo (Renovate coverage unaffected, same default-manager reasoning as
`docs/architecture/`'s silo) and one new `justfile` recipe (`docs-serve`, depending on the existing
`site-build`) that builds and serves the whole docs site locally, the explorer being one page of it.
`frontend/` carries zero docs knowledge, so `just dev` and everything else about the frontend's own dev
loop is entirely unaffected. The server still 404s gracefully if the docs build is ever missing, but
that is a safety net, not the documented path — `just docs-serve` always builds first. Forecloses
committing any interactive-explorer JS to the tree outright, by construction (the same gates that
already forbid it
for anything else). The published site's "Try it out" stays reference-only, which is inherent to a
static site having no backend behind it, not a gap this ADR leaves open.
