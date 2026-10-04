# WiseKiosk task runner. `just` lists recipes; the check recipes are what CI runs.

# Show available recipes.
default:
    @just --list

[group('checks')]
[doc('No untracked, non-ignored file in the tree — the git ls-files-based gates cannot see one, so it must be tracked or gitignored before a local run means anything')]
check-untracked:
    python3 scripts/check-untracked.py

[group('setup')]
[doc('First-time setup: install the pinned pre-commit toolchain into scripts/')]
hooks-install:
    python3 -m venv scripts/.venv
    scripts/.venv/bin/pip install -r scripts/requirements-dev.txt

[group('checks')]
[doc('Every pre-commit-stage hook in .pre-commit-config.yaml passes over every tracked file: no private key, no oversized file, YAML and JSON parse, no merge-conflict marker mid-merge, no mixed line endings, no CRLF in the tracked tree; the commitlint hooks run at their own stages, not here')]
check-hooks:
    scripts/.venv/bin/pre-commit run --all-files

[group('checks')]
[doc('Branch is named type_number-snake_name; its issue is open, type-labeled, milestoned, and parented to match the PR base; the PR records the linkage')]
check-branch *ref:
    python3 scripts/check-branch.py {{ref}}

[group('checks')]
[doc('Requirements tree validates: refs resolve, no suspect/unreviewed/orphan items, methods consistent, headers non-empty and prefix-free, no identifier cited in an item statement; the proposed backlog prints on a passing run (reports; not a gate)')]
check-reqs:
    docs/requirements/.venv/bin/python scripts/check-unreviewed.py
    docs/requirements/.venv/bin/python scripts/check-suspect-links.py
    docs/requirements/.venv/bin/doorstop --error-all --no-reformat
    docs/requirements/.venv/bin/python scripts/check-method-consistency.py
    docs/requirements/.venv/bin/python scripts/check-text-citations.py
    docs/requirements/.venv/bin/python scripts/check-headers.py
    docs/requirements/.venv/bin/python scripts/report-proposed.py

[group('checks')]
[doc('Every requirement identifier and ADR number cited outside .claude/ resolves, and every requirement citation carries its item header')]
check-citations:
    docs/requirements/.venv/bin/python scripts/check-citations.py

[group('checks')]
[doc('The decisions directory and its index table agree: every ADR has a row, every row a file, numbering contiguous')]
check-adr-index:
    python3 scripts/check-adr-index.py

[group('checks')]
[doc('Every ADR citation pins that ADR current rev, in prose and in a link title; the index rev column matches each ADR own head')]
check-adr-revs:
    python3 scripts/check-adr-revs.py

[group('checks')]
[doc('Every tracked document outside a top-level dot-directory is claimed by a row in the documentation index, every row links a tracked file, and no cell is empty')]
check-docs-index:
    python3 scripts/check-docs-index.py

[group('checks')]
[doc('No manifest or .venv/ at the repository root, no recipe is a shell script, and renovate.json extends the pinned WisewareOrg/wise-renovate preset')]
check-repo-silo:
    python3 scripts/check-repo-silo.py

[group('docs')]
[doc('First-time setup: install the pinned Sphinx toolchain into docs/site/, and swagger-ui-dist (locked) for the API explorer page')]
site-install:
    python3 -m venv docs/site/.venv
    docs/site/.venv/bin/pip install -r docs/site/requirements-dev.txt
    npm --prefix docs/site ci

[group('docs')]
[doc('Regenerate the needs pages from Doorstop, copy the API explorer assets, and build the docs site (warnings-as-errors)')]
site-build:
    docs/site/.venv/bin/python docs/site/doorstop_to_needs.py
    docs/site/.venv/bin/python docs/site/copy_explorer_assets.py
    docs/site/.venv/bin/sphinx-build -W -b html -c docs/site docs docs/site/_build/html

[group('checks')]
[doc('Docs site builds clean with warnings-as-errors')]
check-site:
    just site-build

[group('docs')]
[doc('First-time setup: install the pinned LikeC4 toolchain into docs/architecture/')]
arch-install:
    npm --prefix docs/architecture ci

[group('docs')]
[doc('Validate the architecture model, regenerate its browser-free gated artifacts, and rebuild the embeddable interactive webcomponent bundle')]
arch-export:
    docs/architecture/node_modules/.bin/likec4 validate docs/architecture/model
    rm -rf docs/architecture/generated
    docs/architecture/node_modules/.bin/likec4 codegen mermaid docs/architecture/model -o docs/architecture/generated
    python3 scripts/splice-arch-diagrams.py
    rm -rf docs/architecture/embed
    docs/architecture/node_modules/.bin/likec4 codegen webcomponent docs/architecture/model -o docs/architecture/embed/likec4-views.js

# `add --intent-to-add` reaches regenerated artifacts that are untracked; the diff is taken against
# HEAD because that same `git add` stages the deletion `arch-export` makes of an orphan, which an
# index-relative diff then reads as agreement. Both are required and neither suffices alone.
[group('checks')]
[doc('Architecture model validates and its generated artifacts are neither stale, orphaned, nor uncommitted')]
check-arch:
    just arch-export
    git add --intent-to-add -- docs/architecture/
    git diff --exit-code HEAD -- docs/architecture/ docs/ARCHITECTURE.md

[group('checks')]
[doc('Every requirement identifier tagged in the architecture model resolves to an accepted item, and every accepted, active SYS or SRS item is tagged somewhere in the model')]
check-arch-trace:
    docs/requirements/.venv/bin/python scripts/check-arch-trace.py

[group('docs')]
[doc('Live-preview the architecture model in a local dev server (browser; not a gate)')]
arch-dev:
    docs/architecture/node_modules/.bin/likec4 start docs/architecture/model

[group('setup')]
[doc('First-time setup: install the browser the render tier drives')]
render-install:
    frontend/node_modules/.bin/playwright install --with-deps chromium

[group('setup')]
[doc('First-time setup: install the pinned boundary code generators into the package roots they belong to')]
boundary-install:
    go -C backend mod download
    npm --prefix frontend ci

[group('boundary')]
[doc('Regenerate the Go and TypeScript boundary contract — routes, client, server and types — from the one schema')]
codegen:
    cd backend && go generate ./internal/boundary
    cd frontend && node_modules/.bin/orval

[private]
_boundary-go-gen:
    test -f backend/internal/boundary/boundary.gen.go || (cd backend && go generate ./internal/boundary)

[private]
_boundary-node-gen:
    test -f frontend/src/lib/boundary/client.ts || (cd frontend && node_modules/.bin/orval)
    test -f frontend/src/config/types.ts || just config-codegen

# `go build` compiles the non-test tree alone, where `vet` and `test` compile the test files with it,
# so a compile error common to both is reported against the smaller of the two first. A step runs only
# where the one before it exited zero; what the four together assert is docs/CI.md § Backend build,
# vet and tests. The `-race` pass covers the concurrency-bearing packages under `internal/`; the
# bounded-footprint soak in `cmd` is run once without it, because the detector's own allocation
# perturbs the memory that soak measures.
[group('checks')]
[doc('The backend Go tree builds, passes vet, its package tests pass, and the internal packages are free of data races; needs `just boundary-install`')]
check-go: _boundary-go-gen
    go -C backend build ./...
    go -C backend vet ./...
    go -C backend test ./...
    go -C backend test -race ./internal/...

[group('checks')]
[doc('Every module fuzz target (weather: FuzzShape, FuzzDecodeRequest, FuzzValidate; park_wait_times: FuzzShapeRides, FuzzDecodeRequest, FuzzResolvePark) runs fuzzed for 10s, asserting no panic or hang on crafted bytes; needs `just boundary-install`')]
check-fuzz: _boundary-go-gen
    go -C backend test ./internal/modules/weather/ -run '^$' -fuzz '^FuzzShape$' -fuzztime 10s
    go -C backend test ./internal/modules/weather/ -run '^$' -fuzz '^FuzzDecodeRequest$' -fuzztime 10s
    go -C backend test ./internal/modules/weather/ -run '^$' -fuzz '^FuzzValidate$' -fuzztime 10s
    go -C backend test ./internal/modules/park_wait_times/ -run '^$' -fuzz '^FuzzShapeRides$' -fuzztime 10s
    go -C backend test ./internal/modules/park_wait_times/ -run '^$' -fuzz '^FuzzDecodeRequest$' -fuzztime 10s
    go -C backend test ./internal/modules/park_wait_times/ -run '^$' -fuzz '^FuzzResolvePark$' -fuzztime 10s

[group('checks')]
[doc("The backend Go tree is clean under golangci-lint's default linter set and depguard's standard-library-and-module allow-list, non-zero exit on any finding")]
check-lint-go: _boundary-go-gen
    go -C backend tool golangci-lint run ./...

[group('checks')]
[doc('No resolved Go module dependency carries a known vulnerability reachable from the build, unless the exception register covers it; needs the network (vuln.go.dev)')]
check-vulns-go:
    python3 scripts/vulns/check_vulns.py --scope go

[group('checks')]
[doc('Exactly one non-test reference in the backend unwraps the confined secret type to its raw value; test files are exempt')]
check-secret-unwrap:
    python3 scripts/check-secret-unwrap.py

[group('run')]
[doc('Seed the gitignored local display configuration from the shipped example if absent — what `just dev`, `just run` and `just run-container` read')]
config-seed:
    @[ -f frontend/public/config.json ] || { mkdir -p frontend/public && cp deploy/config.example.json frontend/public/config.json; }

[group('run')]
[doc('Serve the display page on a local dev server (hot reload); /api and /healthz are proxied to `just serve`, and the page reads the gitignored frontend/public/config.json the `config-seed` prerequisite writes')]
dev: config-seed _boundary-node-gen
    frontend/node_modules/.bin/vite frontend

[group('run')]
[doc('Run the backend with reload on change, serving /api and /healthz for `just dev` to proxy to; weather is keyless (Open-Meteo), so this serves live data with no secret plumbing')]
serve: _boundary-go-gen
    go -C backend run github.com/bokwoon95/wgo@v0.7.1 run ./cmd -static-root ../frontend/dist

[group('run')]
[doc('Build the bundle and serve the whole app from the backend on :8080 — the working tree as one process, no reload')]
run: config-seed check-build _boundary-go-gen
    go -C backend run ./cmd -static-root ../frontend/dist

[group('run')]
[doc('Build the working tree into the container image and run it on :8080 with the local config mounted; needs Docker')]
run-container: config-seed
    docker compose -f compose.dev.yaml up --build

# Depends on `check-build` so what is served is a bundle that exists and is current, rather than
# whatever a previous run left in `dist/`.
[group('run')]
[doc('Serve the built static bundle rather than the dev server — what a deployment ships, including the compiled configuration validator')]
preview: check-build
    frontend/node_modules/.bin/vite preview frontend

[group('run')]
[doc('Build and serve the whole docs site at :5174, proxied to `just serve` — the API explorer page is interactive there; needs `just serve` running for "Try it out" to reach a live backend; also regenerates the gated architecture artifacts (`arch-export`) if the model changed')]
docs-serve: arch-export site-build
    @echo 'Docs: http://localhost:5174/ (API explorer: http://localhost:5174/api-explorer.html)'
    docs/site/node_modules/.bin/vite --config docs/site/vite.config.ts

[group('checks')]
[doc('The frontend builds to a static single-page bundle; needs `just boundary-install`')]
check-build: _boundary-node-gen
    frontend/node_modules/.bin/vite build frontend

# Depends on `check-build` rather than assuming an emitted tree: what it reads is what that build
# emitted, and `just` runs a dependency once per invocation, so `verify` does not build twice.
[group('checks')]
[doc('The frontend build emits a static single-page bundle: one HTML entry with an empty mount, no server half, no SSR target or adapter declared, and every npm package in the emitted module graph allowlisted')]
check-static-bundle: check-build
    python3 scripts/check-static-bundle.py

[group('checks')]
[doc('The frontend is clean under eslint (flat config, recommended sets) and svelte-check (--tsgo); needs `just boundary-install`')]
check-lint-frontend: _boundary-node-gen
    cd frontend && node_modules/.bin/eslint .
    cd frontend && node_modules/.bin/svelte-check --tsconfig ./tsconfig.json --tsgo

[group('checks')]
[doc('No npm dependency carries a known vulnerability, at any severity, unless the exception register covers it; needs the network (the npm registry)')]
check-vulns-npm:
    python3 scripts/vulns/check_vulns.py --scope npm

[group('checks')]
[doc('The whole frontend typecheck (tsc --noEmit, TS 7) is clean; needs `just boundary-install`')]
check-typecheck-frontend: _boundary-node-gen
    frontend/node_modules/@typescript/native/bin/tsc --noEmit -p frontend/tsconfig.json

[group('checks')]
[doc('The frontend unit tier passes (Vitest); needs `just boundary-install`')]
check-unit: _boundary-node-gen
    frontend/node_modules/.bin/vitest run --root frontend

[group('checks')]
[doc('The frontend render tier passes at each supported viewport (Playwright); needs `just boundary-install` and `just render-install`')]
check-render: _boundary-node-gen
    frontend/node_modules/.bin/playwright test --config frontend/playwright.config.ts

[group('checks')]
[doc('The render tier renders under the security response headers the backend serves, and raises no console warning of a rejected directive or an unrecognised feature; needs `just boundary-install` and `just render-install`')]
check-render-policy: _boundary-node-gen
    frontend/node_modules/.bin/playwright test --config frontend/playwright.policy.config.ts

[group('checks')]
[doc('Every committed test file is discovered by a configured runner, asked of each runner rather than restating its globs; needs `just boundary-install`')]
check-dead-test: _boundary-go-gen _boundary-node-gen
    python3 scripts/check-dead-test.py

[group('checks')]
[doc('The unit-test coverage bar: go-test-coverage over the backend, and one merged Istanbul gate (vitest coverage-istanbul + vite-plugin-istanbul, unioned by scripts/merge-coverage.ts) over every frontend `.ts` and `.svelte` file, each against its own configured bar, per-file and total; then one diagnostic HTML report over both languages (gcov2lcov + grcov), never gating; needs `just boundary-install`, `just render-install` and `grcov` on PATH')]
check-coverage: _boundary-go-gen _boundary-node-gen
    go -C backend test -covermode=atomic -coverpkg=./... -coverprofile=cover.out ./...
    go -C backend tool go-test-coverage --config=.testcoverage.yml
    frontend/node_modules/.bin/vitest run --root frontend --coverage
    frontend/node_modules/.bin/playwright test --config frontend/playwright.coverage.config.ts
    node frontend/scripts/merge-coverage.ts
    mkdir -p coverage
    go -C backend tool gcov2lcov -infile=cover.out -outfile=../coverage/go.info
    grcov coverage/go.info frontend/coverage/frontend-report/lcov.info -s . -t html -o coverage/unified-report/

# Outside `verify`, and invoked by a CI job of its own: this tier builds and runs the image, and
# docs/CI.md § Gate wiring is what decides where a check needing Docker sits.
[group('checks')]
[doc('The container image builds, runs non-root, serves the configuration a deployment mounts and nothing in its place, carries no deployment content, keeps two instances independent, reports its health in both directions, and holds nothing secret-shaped in any layer; needs Docker')]
check-image:
    docker buildx build --load --tag wisekiosk:citest .
    python3 scripts/image/nonroot_uid.py wisekiosk:citest
    python3 scripts/image/config_mount.py wisekiosk:citest
    python3 scripts/image/no_deployment_content.py wisekiosk:citest
    python3 scripts/image/two_instances.py wisekiosk:citest
    python3 scripts/image/health_signal.py wisekiosk:citest
    python3 scripts/image/layer_secret_scan.py wisekiosk:citest

# One architecture per invocation, named by the caller: a matrix leg supplies the platform, and a
# foreign platform needs emulation the runner provides rather than the recipe.
[group('checks')]
[doc('The container image builds for one platform — `linux/amd64` or `linux/arm64` — and comes up serving on it; needs Docker, and emulation for a platform this host is not')]
smoke-image platform:
    docker buildx build --platform {{platform}} --load --tag wisekiosk:citest .
    python3 scripts/image/smoke.py wisekiosk:citest

# The architecture is fixed in the body rather than named by the caller, unlike `smoke-image`
# (docs/CI.md § Native binary run). Depends on `check-build` so what is served is the bundle a
# build emits. The two settings below reach both the build environment and the expectation.
native_goarch := "arm"
native_goarm := "6"

[group('checks')]
[doc('The application builds from source for armv6l — the bundle, and the backend cross-compiled — and comes up serving on it; needs emulation for a binary this host cannot execute')]
smoke-native: check-build _boundary-go-gen
    mkdir -p bin
    GOOS=linux GOARCH={{native_goarch}} GOARM={{native_goarm}} CGO_ENABLED=0 go -C backend build -o ../bin/wisekiosk-armv6 ./cmd
    python3 scripts/native/smoke.py bin/wisekiosk-armv6 frontend/dist {{native_goarch}}/{{native_goarm}}

[group('checks')]
[doc('The documented bring-up procedure reaches a serving deployment from a release directory holding its own compose.yaml and config.example.json, running the given digest; needs Docker and the network (ghcr.io)')]
check-bringup release_dir digest:
    python3 scripts/bringup/bring_up.py {{release_dir}} {{digest}}

[group('checks')]
[doc('The previous published release and the release under test, the second read from a release directory, each serve their mounted configuration and report their own version, under byte-identical mount arguments, with no builder invoked; needs Docker and the network (gh, ghcr.io)')]
check-image-swap previous_tag previous_digest tag release_dir digest:
    python3 scripts/bringup/image_swap.py {{previous_tag}} {{previous_digest}} {{tag}} {{release_dir}} {{digest}}

[group('config')]
[doc('Regenerate the configuration-object TypeScript types from the configuration schema')]
config-codegen:
    frontend/node_modules/.bin/json2ts --input frontend/src/config/schema.json --output frontend/src/config/types.ts --additionalProperties false

[group('review')]
[doc('List every ADR citation this branch re-pinned without touching the sentence around it, per file and line (reports; not a gate)')]
rev-reach *ref:
    python3 scripts/adr-rev-reach.py {{ref}}

[group('checks')]
[doc('Every tracked file is a declared authored or derived kind; a legacy file is grandfathered by path, never its language')]
check-languages:
    python3 scripts/check-languages.py

[group('checks')]
[doc('Every service in the committed deployment recipe declares restart policy `unless-stopped`; that one key and no other value in the recipe')]
check-restart-policy:
    python3 scripts/check-restart-policy.py

[group('checks')]
[doc('The `verify` job in publish.yml holds no write scope: its permissions are exactly `{contents: read}` and no step under it references `secrets.`')]
check-publish-permissions:
    python3 scripts/publish/verify_permissions.py

[group('checks')]
[doc('Run every check the PR gate runs that has a local form and needs neither Docker nor emulation nor the network; secret scanning, the PR-title check (commitlint, via the hook layer), the link check (lychee, from a digest-pinned image) and the workflow audit (zizmor, actionlint) are CI-only, the image tier is `just check-image`, the native armv6l run is `just smoke-native`, the bring-up check against a release directory is `just check-bringup`, the image-swap check against the previous published release and a release directory is `just check-image-swap`, and the two online dependency-vulnerability checks are `just check-vulns-go` and `just check-vulns-npm`')]
verify: check-untracked check-hooks check-branch check-reqs check-citations check-arch check-arch-trace check-go check-fuzz check-lint-go check-secret-unwrap check-build check-static-bundle check-lint-frontend check-typecheck-frontend check-unit check-render check-render-policy check-site check-adr-index check-adr-revs check-docs-index check-repo-silo check-languages check-dead-test check-restart-policy check-publish-permissions check-coverage
