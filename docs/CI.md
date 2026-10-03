# Continuous integration

What CI does for this repository, and what it refuses to let through.

**None of this is a requirement.** A requirement obliges the running software, and every gate below
constrains the repository instead — its source hygiene, its dependencies, its documentation, its own
wiring. The product's obligations live in [`requirements/`](requirements/README.md) and are failed
against there ([ADR 0011 rev 2](decisions/0011-requirement-or-convention.md)). Adding or retiring a gate is
an edit here and a change to the check, not a specification change.

**Most of what follows is not built yet.** The gates are described in the tense they will run in;
where a gate is unbuilt its ticket is named. That is how this project records scoped work
([ADR 0005 rev 4](decisions/0005-traceability-gating.md)).

## What CI provides

Not everything CI does is a gate. These produce material a person acts on.

- **Dependency update proposals.** Renovate runs every six hours from `WisewareOrg/wise-renovate`,
  opening pull requests under the runner's GitHub App `[bot]` login on `renovate/*` branches. Every
  dependency bump is its own pull request, except the Node runtime, which travels as one grouped
  pull request across `engines` fields, `setup-node` inputs and the `Dockerfile` base image; the run
  opens at most two new pull requests an hour and holds at most ten open at once. Patch, minor, pin
  and digest updates auto-merge: GitHub's native auto-merge places the pull request in the merge
  queue (§ *Gate wiring*) once its required checks pass, and Renovate rebases a branch only when it
  conflicts with `main`, since the queue tests each pull request against the current `main` itself;
  major updates open a pull request and wait. A release sits for three days before its update opens a
  branch. Renovate's default title convention, `chore(deps): …`, is already a Conventional Commit, so
  `checks.yml`'s process job runs its pre-commit install and title-lint steps unconditionally for
  every pull request — the exemption Dependabot's capitalised `build(deps): Bump …` default needed
  has no counterpart to carry. Renovate also keeps one open Dependency Dashboard issue, carrying no
  milestone or type label; the ticket gates ([ADR 0013 rev 4](decisions/0013-work-tracking-invariants.md))
  read only the issue a branch names, so the dashboard issue is exempt by construction and no branch
  may be cut from it. That the root `renovate.json` resolves the pinned preset is § *Repository
  shape*'s. The project's own image is excluded from Renovate: the committed recipe names the
  movable `latest` tag by design ([`DEPLOYMENT.md`](DEPLOYMENT.md) § *Bring-up*), and pinning it
  would loop through `publish.yml` — a merged pin publishes a new digest, which opens the next pin.
- **Code-scanning results.** Every static-analysis finding is reported to the repository's
  code-scanning dashboard and annotated on the pull request, whether or not it fails the build.
- **Complete scan output.** The vulnerability gates, `check-vulns-go` and `check-vulns-npm`, report
  every advisory they resolve — at any severity, regardless of reachability or exception status. The
  gate decides the merge; the output stays complete, so a suppressed finding is still visible.
- **Release provenance material.** A published release carries a signature, an SBOM and a
  build-provenance attestation alongside the image. CI produces them; whoever pulls the image uses
  them to establish what it is and what went into it. What is asserted about them is below.

## First-party source scanning

[`../.github/workflows/codeql.yml`](../.github/workflows/codeql.yml)'s `codeql` job runs CodeQL's
default code-scanning suite over the project's own source, on every pull request, every push to
`main`, and weekly (so a dormant branch is still covered) — one leg per language: Go
(`build-mode: autobuild`), Svelte/TypeScript, Python, and the workflow files under
`.github/workflows` (CodeQL's `actions` language; `scripts/` is first-party source too, reached by
the other three). No `queries:` input: the action's own default is the code-scanning suite, so
widening to a named suite later is a visible diff rather than a silent one.

**Failing on any finding at any severity is two repository settings, not a file this gate can
assert.** The per-language legs above fail only on an execution error; the check that fails on a
finding is GitHub's own `CodeQL` check (`GitHub Advanced Security`), and its severity threshold is a repository
setting defaulting to high rather than every severity. Both settings — the threshold, and the check
names required on the branch's protection — are repository configuration rather than tracked files,
recorded in § *Gate wiring* below rather than asserted here.

**A finding in first-party source has no exception path.** The register below exists to record that
somebody else shipped something broken, and that justification is not available for code this project
writes.

That the gate can fail — a canonical vulnerable pattern per language, run through the production
analysis and observed to raise the expected alert — is proven once against a throwaway branch and
recorded in [`../scripts/cases/codeql.md`](../scripts/cases/codeql.md), per
[ADR 0010 rev 3](decisions/0010-runtime-materialised-gate-fixtures.md): no vulnerable artifact is
ever committed in resolvable form, and fallibility is recorded once rather than re-tested by a
standing meta-gate.

This mechanises the security review a solo project has no second reader to perform.

## Lint and type checks

Every source package's linters run as blocking checks. **No linter is advisory-only** — a lint gate
that reports without failing degrades to noise within a release.

- Go: `golangci-lint`'s default linter set (errcheck, govet, ineffassign, staticcheck, unused), the
  production invocation, non-zero exit asserted. Recorded in
  [`../scripts/cases/check-lint-go.md`](../scripts/cases/check-lint-go.md).
- Frontend: `eslint` (flat config, recommended sets), non-zero exit asserted; `svelte-check`
  (`--tsgo`), non-zero exit asserted; and the whole-project TypeScript typecheck (`tsc --noEmit`,
  TS 7), non-zero exit asserted. Recorded in
  [`../scripts/cases/check-lint-frontend.md`](../scripts/cases/check-lint-frontend.md) and
  [`../scripts/cases/check-typecheck-frontend.md`](../scripts/cases/check-typecheck-frontend.md).

**TypeScript runs two versions side by side.** `typescript-eslint` refuses TS 7 and `svelte-check`'s
own `--tsgo` mode requires TS 6 installed beside it — Microsoft's documented TS 7 side-by-side
transition. So the `typescript` devDependency is the newest TS 6 release, exact-pinned, consumed by
`eslint` and `svelte-check`; TS 7 stays reachable under the exact alias
`@typescript/native@npm:typescript@7.0.2`, which `check-typecheck-frontend` invokes by explicit path.
The alias claims `node_modules/.bin/tsc`, so a bare `tsc` now resolves to TS 7 rather than to a
decided version — every recipe that means a specific one invokes it by path.

## Backend build, vet and tests

The Go tree compiles, `go vet`'s default analyser set reports nothing over it, the backend package
tests pass, and the packages under `internal/` pass a second time under the race detector. Four steps
in one recipe, ordered cheapest-failure-first: `go build` reads the non-test tree alone, so a compile
error is reported against it rather than buried under the test files' copy of the same error.

- Each step is seeded independently — an undefined identifier, a `printf` argument `vet` rejects and
  the compiler accepts, a passing-`vet` change that breaks a test, and a narrowed critical section no
  test failure reaches — because a step is reached only when the ones before it passed, so a seed
  failing at `build` proves nothing about the three behind it. Recorded in
  [`../scripts/cases/check-go.md`](../scripts/cases/check-go.md).

- **A tree holding no Go package fails**, on `vet` rather than on `build`: `go build ./...` warns that
  the pattern matched nothing and exits zero, where `go vet ./...` refuses an empty package set. So a
  backend that resolves to nothing is a failure rather than a clean run, and it is the second step
  that decides it.

- **The `-race` step makes the concurrency tests assert synchronisation rather than a count.**
  `internal/ratelimit`'s mutex-guarded buckets and `internal/upstream`'s single flight are the
  concurrency-bearing packages, and the tests over them create real contention rather than describing
  it — 200 goroutines released at once onto one token bucket. A regression narrowing `Allow`'s
  critical section so the bucket map stays guarded but the token arithmetic does not still returns the
  right grant count: fifty consecutive executions of the seeded test pass without `-race`, where the
  detector fails it on the first, reporting the unsynchronised read and write by source line. Counting
  the outcome cannot separate a synchronised bucket from a lucky one; the detector reads the accesses
  themselves. It is the recipe's one step needing a C toolchain — `-race` refuses to build under
  `CGO_ENABLED=0`, exiting 2 with `-race requires cgo` — which `ubuntu-latest` carries and a local run
  wants on `PATH`.

- **The `cmd` soak is outside the `-race` step**, which is scoped to `./internal/...`. Including it
  would re-run the ~120s bounded-footprint soak under the detector, roughly doubling a step already
  that long, for race coverage the `./internal/...` pass already provides — the soak's purpose is
  live-heap growth, not race detection, and it trips no data race. `cmd` is covered once, by the
  plain `test ./...` before it.

**What it leaves unproven.** `go vet` is a fixed analyser set rather than a linter; a configured Go
linter is § *Lint and type checks*'s. And
**a package with no test in it passes** — `go test` reports one as a non-failure, so a test lost to a
build tag, a wrong directory or a deletion is invisible here, and the whole test set deleted from a
backend that still builds exits zero, measured rather than inferred. Refusing an empty package set
does not reach that: the packages are present and it is the tests that are gone. Closing it is §
*Gate wiring*'s whole-tree discovery gate, which reads the population from the tracked tree rather
than from what a runner happened to execute, and this gate is no substitute for it.

**The race detector is a detector, not a proof.** It reports the unsynchronised accesses a run
actually performs, so an unsynchronised path no test drives is invisible to it and a clean `-race` run
is not a claim that the package is free of races. The step's value is borrowed entirely from the tests
underneath it: run the seeded regression above under `-race` with every test but the concurrent one
selected and it exits 0, the race present and undriven. It also says nothing about `cmd`, which the
step excludes.

What those tests must *prove* is [`TESTING.md`](TESTING.md)'s and the obligations they answer are the
tree's; what is decided here is only that the tree builds, that `vet` is clean over it, that the tier
is executed on the merge path rather than declared, and that the tier's concurrent packages executed
without a detected race.

## Backend fuzz

Three targets — `FuzzShape`, `FuzzDecodeRequest`, `FuzzValidate` — each run under `go test -fuzz` for
a fixed ten seconds, one invocation per target, in `check-fuzz`. Every target asserts no panic and no
hang within the budget; `FuzzShape` additionally asserts its result is deterministic across two calls
on the same bytes. What they must guarantee is [`TESTING.md`](TESTING.md)'s Fuzz row; the targets
named, the rejected alternatives and the reopen premise are recorded on
[#267 backend fuzz tier](https://github.com/WisewareOrg/WiseKiosk/issues/267).

**No panic is the engine's to catch; no hang is the target's own.** Go's fuzzing engine reports a
panic the moment it happens, which a ten-second search catches reliably. Its own documentation
[describes a per-execution timeout for a hang](https://go.dev/security/fuzz/)
("the fuzz target took too long to complete... this may fail due to a deadlock or infinite loop"),
but that detection did not resolve inside this tier's ten-second search in testing — the search's own
graceful end-of-run reliably arrived first, silently dropping the hung input rather than reporting it
(`scripts/cases/check-fuzz.md` records the runs this was measured against). So each target wraps its
own call in a one-second per-input wall-clock deadline (`runWithin`,
`backend/internal/modules/weather/weather_test.go`) that fails the test and names the target that
hung, rather than leaning on a mechanism that arrives too late to help at this budget.

- **The seed corpus is `f.Add` calls in the test file**, shared with the table tests that carry the
  same bounds rather than duplicated. Go's own default location, `testdata/fuzz/<FuzzFuncName>/`,
  holds nothing until a run finds a crasher — no empty directory is committed ahead of one.
- **`check-go`'s plain `go test ./...` already runs every `Fuzz*` function's seed corpus as an
  ordinary test**, from the commit each target lands in. `check-fuzz` is what mutates past the seeds;
  `check-go` is what keeps a committed crasher from regressing silently between fuzz runs.
- **A crasher found on the merge path fails the run**, and the corpus entry Go's minimizer writes
  under `testdata/fuzz/` is what the fixing change commits — the regression is closed by that entry
  passing under `check-go` from then on, the same way a fixed bug elsewhere gains a test.
- **A crasher found the first time a target's fuzzing actually runs is reported, not blocked on.**
  Per the reporting-first rule, that target's `check-fuzz` line lands non-blocking (a `-` prefix, a
  comment naming the follow-on ticket) until the finding is fixed, rather than failing every
  subsequent change for a defect this tier alone is able to see.

Recorded in [`../scripts/cases/check-fuzz.md`](../scripts/cases/check-fuzz.md).

## Unit-test coverage

`check-coverage` is gate 3 ([ADR 0005 rev 4](decisions/0005-traceability-gating.md)): a coverage bar
over first-party product source, tree-blind — it proves source is exercised, not that a `TST` item
claims it. `go-test-coverage` for the backend, configured
([ADR 0016 rev 11](decisions/0016-maintained-tools-for-standard-artifacts.md)); an authored script
over two Istanbul providers for the frontend:

- **Backend** — `go -C backend test -covermode=atomic -coverpkg=./... -coverprofile=cover.out ./...`
  then `go -C backend tool go-test-coverage --config=.testcoverage.yml` (the tool pinned in
  `backend/go.mod`'s `tool` block), against the statement-coverage bar `backend/.testcoverage.yml`
  configures, per-file and total, excluding generated Go (`\.gen\.go$`).
- **Frontend** — `frontend/node_modules/.bin/vitest run --root frontend --coverage`
  (`@vitest/coverage-istanbul`) then `playwright test --config frontend/playwright.coverage.config.ts`
  (`vite-plugin-istanbul`, torn down by `tests/render/coverage-teardown.ts`) each write their own
  `coverage-final.json` over the same `.ts`/`.svelte` sources; neither carries a threshold of its
  own. `frontend/scripts/merge-coverage.ts` then unions the two via `istanbul-lib-coverage` and
  enforces the one bar value in `frontend/coverage-thresholds.json`, per file, across all four
  metrics — line, branch, function and statement — an authored gate, not a configured tool's own
  setting. Its completeness guard fails the run when a file either tier means to instrument carries
  no coverage at all, so a file neither tier still imports is caught rather than silently absent.

Neither recipe needs the network or Docker. `check-coverage` is folded into `just verify`, and the
recipe runs the backend command then the frontend commands in sequence, stopping at the first one
under its bar — a run where the backend fails does not reach the frontend commands. After both
gates, `gcov2lcov` (also pinned in `backend/go.mod`'s `tool` block, run from `backend/` so its
relative `SF:` paths resolve) converts the backend's Go coverprofile to lcov, and `grcov` merges it
with the frontend's own `merge-coverage.ts`-emitted lcov into one further, diagnostic-only HTML
report unioning both languages, uploaded as CI's `coverage-report` artifact; neither gate's
threshold or exit status is affected by it. `coverage`'s CI job runs the same recipe as `verify` —
though the required-checks ruleset naming that job a required status check, so a merge bypassing
`verify` locally would also be blocked by a red run at the platform level, stays a separate, later,
owner-driven action, not designed here.

## Image tests

The container image is built from the tracked tree and the Image tier is run over it, in two jobs of
their own: `image-tests` runs `just check-image` — the five property harnesses under
`scripts/image/`, plus the health-signal check § *Deployment and bring-up* states — and `image-arch`
runs `just smoke-image` once per image architecture. Both need Docker, which is why neither is a
`just verify` dependency (§ *Gate wiring*).

- **The artifact under test is the one this commit builds**, not a published digest: the job builds
  it and hands the ref to each harness, so a change to the Dockerfile is judged by the same run that
  makes it. What the tier must guarantee is [`TESTING.md`](TESTING.md)'s and the obligations it
  answers are the tree's; what is decided here is that it is executed on the merge path rather than
  declared. Recorded in [`../scripts/cases/check-image.md`](../scripts/cases/check-image.md).
- **Both image architectures are smoke-tested, one matrix leg each** — `linux/amd64` and
  `linux/arm64`, the set a release publishes. A leg builds for its own platform and
  runs `scripts/image/smoke.py` over what it loaded: the container answers on its published port, and
  answers the argument vector its own `HEALTHCHECK` declares. `fail-fast` is off, so one
  architecture's failure still leaves the other's verdict; the foreign leg builds and runs under the
  emulation the runner sets up, this runner being amd64 as the publishing one is. The harness is
  handed a ref and asserts nothing about which platform produced it — what was smoke-tested is the
  leg's to decide, and the image's declared platform is printed rather than checked against a
  restatement here.
- **The property harnesses are not re-run per architecture.** Coming up and serving is what varies
  with the architecture; the configuration, layer and isolation properties hold of the image on
  either, so a second run of them would be a repeat rather than a second assertion.

## Native binary run

`native-arch` runs `just smoke-native`: it builds the frontend bundle, cross-compiles the backend for
`armv6l`, and runs `scripts/native/smoke.py` over the pair. It answers the architecture
SRS019<!-- The backend runs on every supported architecture --> names that the section above cannot
reach — no image is published for it, so there is no image to smoke-test. It has a local form and
needs no Docker, but it does need emulation a contributor machine is not assumed to carry, which is
why it is not a `just verify` dependency either (§ *Gate wiring*).

- **What is under test is the application, not an image.** The job builds both halves from the
  tracked tree and starts the binary as a process with the bundle as its static root, which is the
  shape the native deployment model runs in ([`ARCHITECTURE.md`](ARCHITECTURE.md) draws it and this
  summarises it). Nothing here builds, loads or pushes an image, and the container architectures stay
  the section above's — adding a leg there would assert something no published artifact does.
  Recorded in
  [`../scripts/cases/smoke-native.md`](../scripts/cases/smoke-native.md).
- **The architecture is asserted, not printed.** The recipe holds the target in one place, which
  reaches both the build environment and the harness's expectation, so the binary cannot be built for
  one architecture and judged against another. The harness reads the ELF header's machine and the
  `GOARM` the toolchain records in the binary, and fails when either disagrees. Both halves are
  needed: `EM_ARM` is one value for every ARM32 variant, the Go linker emits no `.ARM.attributes`
  section carrying `Tag_CPU_arch`, and a `GOARM=6` and `GOARM=7` build have identical `e_flags`, so
  the header alone would pass a build at the wrong revision — including one where `GOARM` was dropped
  and the toolchain silently defaulted to `7`.
- **The runner executes the foreign binary directly.** `docker/setup-qemu-action` registers
  user-mode emulation as a binary-format handler, so an `armv6l` executable runs on this amd64
  runner the way an `amd64` one does. A binary the emulation cannot start fails the job, reported as
  having judged no binary rather than as an architecture that came up.
- **The bundle is part of what is asserted.** Liveness is answered by a route that never reads the
  served tree, so a static root naming a tree that is not there answers it perfectly well. The
  harness fetches the page as well, which is what makes the frontend build and the static root it is
  pointed at something the job decides rather than arguments nothing reads.
- **A held address is reported as having judged nothing.** The binary compiles its address in and
  takes no flag for it, so a process already on that address answers liveness, the page and the
  binary's own `-health-check` while the binary under test dies on a bind it lost. That is
  indistinguishable from a clean run, so the address is refused before anything is probed.
- **Emulation is not the board.** What the job decides is that the binary is a 32-bit ARM binary the
  toolchain built for ARMv6, and that it starts and serves. Instruction timing, memory pressure and a
  real board's peripherals are outside it, and are named as unproven by the item the harness serves.

## Generated boundary contract

The one OpenAPI schema is hand-authored and both sides' **routes, client, server and types** are
generated from it ([ADR 0008 rev 6](decisions/0008-boundary-contract-openapi-codegen.md)) — at build,
when the generated file is absent, and never committed. A fresh checkout never carries either
generated file, so every build regenerates both before anything downstream reads them: there is no
committed copy for a schema edit, a hand-edit, or a generator version bump to drift from.

- **Routes are inside what is generated.** Because each side's route table is generated rather than
  written, a path that moves in the schema moves identically on both sides — the case a types-only
  setup could not reach, since neither side's generated output would mention a path at all.
- **Each side's own recipe generates its file only when absent** — private `justfile` recipes
  (`_boundary-go-gen`, `_boundary-node-gen`) test for the file and, if missing, invoke that side's own
  generator: Go under `go tool oapi-codegen`, TypeScript under `orval` (and, for the configuration
  types, `json2ts`). `just codegen`/`just config-codegen` stay as unconditional manual escape hatches.
- **Both languages are compiled, not just written.** `go build` (§ *Backend build, vet and tests*) and
  the whole-project `tsc --noEmit` (§ *Lint and type checks*) each fail on output that is
  syntactically present and does not compile. The Go half also catches what mere presence cannot:
  oapi-codegen exits *zero* on any configuration it accepts, including one naming fewer targets than
  this repo needs, and its parser falls back to an older configuration schema rather than refusing a
  mis-shaped one — so a whole target can go missing from a non-empty file. The backend consumes each
  target (the routes through the generated router, the error bodies through the generated models),
  which turns a missing one into an undefined symbol. Because neither artifact is ever committed, a
  fresh checkout's first build of either side compiles against output regenerated in that same
  run — not a narrower check reaching only the boundary package and its per-module `props.ts`
  consumers, but the whole build (Go) and the whole frontend typecheck (TypeScript).
- **A malformed schema or an unresolvable generator fails the build outright**, before a line of Go
  or TypeScript compiles — there is no separate drift gate to catch it after the fact.

**What this leaves unproven** is whether the schema says what the boundary actually carries;
generation and compilation both read the schema and prove nothing against the running system.

## In-code prose

Comments state mechanism. Reason, history and evaluative judgement are authored in a documentation
home and cited from the comment.

- A requirement ID or ADR number cited in a comment names an existing item or decision, and carries
  the item's header in the form § *Documentation integrity* sets out. This is a convention carried by
  review rather than by a gate: `check-citations` reads tracked Markdown outside `.claude/` and every
  item's `rationale` and `verification-justification`, so a comment in Go or TypeScript source is
  outside the population it scans, and nothing widens it.

Whether a comment is narrative rather than mechanism, and whether added comment volume earns its
place, is a review habit rather than a gate: #59 comment-discipline gate closed not-planned (owner,
2026-08-16). The defect class it targeted is carried by the `review-diff.py` pre-commit hook, which
surfaces [`CONTRIBUTING.md`](../CONTRIBUTING.md)'s *Comments* checklist questions before a commit, and by
independent review — across the five-PR ADR 0016 rev 11 adoption wave this produced zero
comment-discipline findings. Reopens if a narrative block or comment bloat reaches `main` past both.

## Dependency vulnerabilities

Any resolved Go-module or npm dependency with a known vulnerability fails the pull request, **at any
severity**, unless the finding has a current register entry. One stdlib-only script,
`scripts/vulns/check_vulns.py`, asserts both ecosystems and applies the one register; `--scope go` or
`--scope npm` selects which (a third, `--scope image`, is § *Image vulnerabilities*'s). `just
check-vulns-go` runs `go -C backend tool govulncheck -json ./...`; `just check-vulns-npm` runs `npm
--prefix frontend audit --json` over the whole dependency tree, no `--audit-level`. Pass/fail is
decided from each scanner's parsed JSON alone — `govulncheck`'s `-json` mode exits 0 whatever it
finds, and this gate holds npm's scanner to the same rule rather than trusting an exit code the other
tool cannot promise. The complete finding list is always printed, per § *What CI provides*, whether or
not a finding is suppressed.

**One allowance, Go only:** the gate may consider reachability — a vulnerability in code that is
present but never called need not fail. Without it the gate produces findings nobody can act on, and
a gate people learn to ignore is worse than none. A finding whose trace reaches a called symbol fails;
one govulncheck reports only at package or module level, with no call reaching it, prints as
informational.

**A Go standard-library finding fails like any other, and the register never covers it.** The
register exists for a third-party dependency with no fix and no alternative available; the standard
library has neither failure shape, since its remedy is always the next Go toolchain bump (which
`main`'s `go.mod` unblocks and Renovate proposes) — an entry naming one is refused regardless of
currency, the same as a first-party finding.

Both recipes need the network on every run (`vuln.go.dev`, the npm registry), so neither is a `just
verify` dependency (§ *Gate wiring*) — each runs instead as a blocking step in its own CI job,
`backend-tests` for Go and `frontend` for npm.

**No fixture is committed** — [ADR 0010 rev 3](decisions/0010-runtime-materialised-gate-fixtures.md)
forbids a resolvable vulnerable artifact in the tracked tree. Each case is built in a throwaway
directory at record time (a Go `main` importing a named vulnerable module version and calling its
symbol; a `package.json` pinning a dependency with a named advisory) and run through the production
script with `--go-dir`/`--npm-dir`/`--register` pointed at it; the seed text and the advisory ids and
versions it names are recorded verbatim so a reviewer can rebuild it. Recorded in
[`../scripts/cases/check-vulns-go.md`](../scripts/cases/check-vulns-go.md) and
[`../scripts/cases/check-vulns-npm.md`](../scripts/cases/check-vulns-npm.md).

## Image vulnerabilities

The built container image is scanned, failing the build on any finding at any severity unless the
finding has a current register entry. `--scope image` of the same script, `scripts/vulns/check_vulns.py`,
runs Trivy — from a digest-pinned image, `TRIVY_IMAGE` in the script — against the local Docker
daemon's copy of the tag the `image-tests` job already built (`wisekiosk:citest`), and holds the parsed
`--format json` output to the one register `--scope go` and `--scope npm` already apply. An empty or
missing fixed version changes nothing: an unfixed finding fails like any other, and a register entry is
always eligible relief, because nothing in a built image is first-party.

This is what covers operating-system and base-layer packages. The source-level dependency gate never
inspects them.

**No fixture is committed** — [ADR 0010 rev 3](decisions/0010-runtime-materialised-gate-fixtures.md)
forbids a resolvable vulnerable artifact in the tracked tree. The case is built as a throwaway image at
record time and run through the production script with `--image` pointed at it; the seed Dockerfile and
the vulnerability ids it names are recorded verbatim so a reviewer can rebuild it. Recorded in
[`../scripts/cases/check-vulns-image.md`](../scripts/cases/check-vulns-image.md).

## Secret scanning

A pull request, and every push to the default branch, is scanned for committed credentials, and a
finding fails the merge. The scan walks **the commits the event carries** rather than the tree at its
tip — the pull request's own commits, or the commits a push delivered — so a secret added and then
removed within one branch still fails: the value is compromised from the moment it is pushed, and the
commit that removes it changes nothing.

The merge queue's `merge_group` run skips the scan: the pinned `gitleaks-action` refuses that event,
and a group holds exactly one pull request, whose commits its own pull-request run scanned and the
push to `main` that lands it scans again.

**What that shape does not reach**, and what may therefore not be read into a green result: history
behind the branch point, which this gate does not re-read; a commit reachable only through a merge's
second parent, because the walk follows first parents and skips merges; and the tail of a range longer
than the event's own commit list, which is only as long as the API page or the webhook payload that
carries it — a batch push or a force-push after a rebase reaches that bound without being
remarkable. The scan is pattern-based besides, so it catches the credential shapes it holds rules for
and nothing reports what it missed. It raises the cost of committing a credential; it is not an
assertion that the repository holds none, and the delivery rules the tree carries stand independently
of it.

**What no check here decides.** GitHub's own secret scanning and its push protection are repository
settings rather than files. Reading one directly means reading `security_and_analysis` on the
repository object, which is returned only to an administrator, and the workflow `permissions:` key
offers no scope a job could request to become one. That indirect route — asking whether the scanner has
produced anything — is closed as well: the secret-scanning alerts endpoint answers a
workflow token `403 Resource not accessible by integration` even where the job requests
`security-events: read` — a scope that does reach code scanning from the same job, so the refusal is
the endpoint's rather than the grant's. No gate here decides whether either setting is on. The
alternative is a standing admin credential held in the repository so that a job can read one, which is
a larger hole than it closes; the scan above stands on the merge path either way. Both settings are
enabled, and this paragraph rather than a check is what records it.

## Publishing and provenance

What a release publishes and what CI asserts about it. Verification runs against the draft's digest,
before the release can lock, in a separate job, `verify`, that reads three surfaces only — the
registry and the public transparency log, both via cosign, and GitHub's attestation API — plus the
draft's own read-back artifact for its assets and notes, and holds no write scope. Installing the
job's own tools — the cosign and syft installers, and the `pipx` fetch of `check-jsonschema` from
PyPI — is the job's only other network activity, distinct from the evidence surfaces above.
What builds and pushes the image is `.github/workflows/publish.yml`, against the set
[ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md) decides. That workflow
runs only when the owner pushes a `vMAJOR.MINOR.PATCH` tag and dispatches it, tags the image by that
semver, and creates and fills a draft release for that tag, reading everything it attached back as a
workflow artifact; `verify`, `bring-up` and `image-swap` all check that artifact, and only once they
all pass does a separate `publish` job re-read the draft and flip it to published
([ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md)) — a separate `latest`
job then moves `latest` to the published digest, for a non-pre-release
([ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md)); the committed recipe
references `latest` so it runs unedited ([`DEPLOYMENT.md`](DEPLOYMENT.md) § *Bring-up*); the release
notes carry exactly one line naming that digest,
`Image: ghcr.io/wisewareorg/wisekiosk@sha256:<digest>`
([ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md)), and it is what an
operator who chooses to verify checks against. Before any registry write, the `draft` job's own first
step fails the run when the tag's release is already published, or when more than one release
carries the tag — a re-dispatch must never move a published release's semver image tag onto a
rebuilt digest its `Image:` line does not name. Recorded in
[`../scripts/cases/publish-draft.md`](../scripts/cases/publish-draft.md).

**The `verify` job's write-scope property is decided, not proposed.**
`scripts/publish/verify_permissions.py` reads `publish.yml` at the release commit, locates the
`verify` job, and fails unless its `permissions` mapping is exactly `{contents: read}` and no step
under it references `secrets.` — the `github.token` expression is the one credential allowed, by
name. It runs locally as `check-publish-permissions`, a `just verify` dependency and a step in the
`docs-and-hygiene` job. #77 Gate CI.md against the workflow it describes fences this document as a
whole; until then, read anything this gate does not itself assert as intent.

**This job runs on a tag dispatch rather than on a pull request**, so it is neither a required
status check nor one of § *Gate wiring*'s no-local-form exceptions. A failed verification fails the
run before the release ever locks: the draft stays a draft, and a re-dispatch on the same tag reuses
it rather than cutting a new version — at a fixed commit first, when the failure was in the code
rather than transient.

- **A release occupies two locations, and each is a separate assertion.** The registry carries the
  image at a digest, with the SBOM, the signature and the build-provenance attestation attached to it
  under the signing tools' conventions (cosign tags and GitHub's attestation store). The release tag
  carries exactly two files, at their committed basenames: the deployment recipe, `compose.yaml`, and
  an example configuration, `config.example.json`. The release notes name the digest, which is what
  ties the tag to the registry. Resolving the artifacts attached to a digest, listing files on a tag
  and reading the notes for a digest are three separate claims, so **a run that reads one
  surface and skips another fails** rather than reporting success over the part it reached — an
  unreadable surface, and resolving no surface at all, are failures and not skips. An undeclared file
  on the tag fails, and so does a declared one that is absent.
  **The image reference is not an asset**, and counting it as one is what let a single sentence stand
  for all three claims: it is a pointer rather than a file, and asserting the notes name it is a
  different act from enumerating a tag.
  **What no check here decides:** the documentation site is deployed from the default branch rather
  than from a tag, so it is not a release asset and nothing asserts any correspondence between what it
  describes and the digest an operator is running. That drift is chosen rather than overlooked, and
  ADR 0020 rev 5 records the choice.
- **Signature.** Keyless `cosign verify` against the draft's index digest and each platform child,
  with the certificate identity bound to this workflow's own runs dispatched on a tag ref
  (`--certificate-identity-regexp`) and the GitHub Actions OIDC issuer, exits zero; against a
  deliberately wrong identity it exits non-zero and prints `no matching CertificateIdentity found`,
  measured at cosign v3.1.3.
- **Provenance.** `gh attestation verify`, bound to this workflow by `--signer-workflow`, validates
  the build-provenance attestation for the draft's digest against GitHub's attestation store. The
  binding is asserted positively, from the command's own `--format json` fields — the subject digest,
  the SLSA predicate type, and the certificate's signer URI, source repository and source repository
  digest — never from a negative's message, because the three refusal cases print three distinct
  texts rather than one shared, discriminating one: against a flipped digest the registry itself
  refuses to resolve the reference before any attestation lookup runs, printing the substring
  `MANIFEST_UNKNOWN: manifest unknown` (measured against a real published digest); against a real
  digest carrying no attestation — true of every release published before this job existed — `gh`
  prints `Error: HTTP 404: Not Found (…/attestations/…)` (also measured); a wrong signer workflow
  against a digest that *does* carry an attestation prints `Error: verifying with issuer
  "sigstore.dev"` (also measured). The wrong-signer-workflow check nonetheless asserts only a
  non-zero exit and an empty stdout — the shape every refusal above shares, with or without
  `--format json` — a deliberately weak assertion, since the six positive field assertions carry the
  verdict. A second invocation, with `--bundle-from-oci`, verifies the copy of the same bundle
  `push-to-registry: true` also lands in the registry, since neither the API-backed invocation above
  nor `cosign tree` reads that copy.
- **SBOM.** Exactly one SPDX 2.3 attestation per platform child is asserted by count — `cosign
  attest` appends rather than replaces, so a re-run of `publish` attaching a second attestation to
  the same child fails here rather than silently validating whichever one `cosign
  verify-attestation` prints first — and is extractable from its attestation, validates against a
  vendored copy of the SPDX 2.3 schema, and enumerates the Go main module
  (`backend/go.mod`'s `module` line, restated nowhere) and the base distribution — read from every
  package's purl `distro=` qualifier rather than a package named for it, since Alpine ships no such
  package; a package set carrying no `distro=` qualifier at all fails. Regenerating with the same
  pinned syft on the same child digest and platform yields a matching set of packages and versions —
  the comparison keys on `(name, versionInfo)` pairs, which a name set alone cannot tell apart on the
  draft's index. A separate, named assertion binds the describing image package's own
  `versionInfo` to the child digest under verification and its purl `arch=` to that child's platform
  architecture, so a cross-wired attestation (the other child's SBOM, which otherwise passes every
  assertion above) fails there rather than by accident. A child without its SBOM attestation fails.
  **The `publish` and `verify` jobs each pin syft and cosign as their own `with:` literal.**
  Renovate's regex manager bumps every match of `cosign-release:` and `syft-version:` in
  `publish.yml` in one pull request, which is what keeps the two per-tool pins equal; a hand edit
  drifting them apart surfaces as a package-set mismatch in the regeneration comparison above.
- **The image says where it came from.** Nine keys — `org.opencontainers.image.title`,
  `.description`, `.url`, `.source`, `.version`, `.created`, `.revision`, `.licenses` and
  `.documentation` — are present and non-empty **as labels on the image config and as annotations on
  the manifest**, which are separate metadata with separate readers
  ([ADR 0015 rev 4](decisions/0015-container-toolchain-and-image-annotations.md)). Both are read: a check
  reading one reports nothing about the other. No value is a `LABEL` line in the Dockerfile, and one
  is bound rather than merely present — **`.revision` is the commit the job published**, so presence
  alone passes on a stale hardcoded value while the binding is what fails it. The check reads the
  annotation levels from the publish workflow's own declaration rather than a restatement here, so
  adding a level extends the gate instead of escaping it, and a level the workflow declares that this
  check does not recognise fails rather than being skipped. A missing key, an empty value, a surface
  it cannot read, a declared level it never inspected, and resolving no surface at all each fail
  rather than being skipped.
  **What no check here decides:** `.description` and `.licenses` resolve from repository metadata
  rather than from the commit, so either can change with no commit and nothing reports it; and
  `.created` is the time the build ran, this project making no bit-identical-rebuild claim.
  **What emits them** is `publish.yml`'s `docker/metadata-action` step (#54): eight of the nine are
  that action's defaults, `.revision` among them and bound to the commit the job runs on, and
  `.documentation` is supplied as a literal on both surfaces. The levels the check reads are
  declared in that step as
  `DOCKER_METADATA_ANNOTATIONS_LEVELS: index,manifest`.
- **Base images are pinned** to a `@sha256:` digest rather than a floating tag, for every base and
  stage in the Dockerfile — three stages, three digests in the committed file (#54).
- **The code generators are pinned** to an exact version, so a toolchain bump cannot present as
  schema drift and a regeneration is reproducible. Structurally the same rule as the line above, and
  no requirement states it: nothing the running software does can violate a pin (#7).

## Deployment and bring-up

What a release must let an operator do, checked by running it rather than by reading it. What each
of these obligations *is*, and why, is [`DEPLOYMENT.md`](DEPLOYMENT.md)'s. The `bring-up` and
`image-swap` jobs below run once the `verify` job (§ *Publishing and provenance*) passes, against the
draft's own digest and its read-back artifact, before the release publishes — pre-release included,
since what they exercise is the draft rather than a published release — so neither exercises a
release whose signature or attestation failed to verify.

- **The documented procedure executes.** `just check-bringup`, run by the `bring-up` job in
  [`../.github/workflows/publish.yml`](../.github/workflows/publish.yml) against the draft this
  dispatch is filling: it runs the first `sh` fence after [`DEPLOYMENT.md`](DEPLOYMENT.md)
  § *Bring-up*'s heading, line by line, unedited, inside the draft's read-back artifact directory,
  which already holds the two assets — with `WISEKIOSK_IMAGE` exported to the fence, so
  `deploy/compose.yaml`'s `${WISEKIOSK_IMAGE:-...}` resolves to this release's digest in place of its
  default `:latest`. It fails if a documented step does not run, or if the sequence completes without
  a serving deployment — so documentation that omits a step fails here rather than at somebody's
  first deployment. Serving is three assertions: the running container's own image resolves to the
  digest under test — what the documented procedure actually brought up, however the recipe's
  `${WISEKIOSK_IMAGE:-...}` resolved it — the compose service's container reaches Docker health
  status `healthy` within a deadline derived from the image's own declared healthcheck, and
  `GET /config.json` returns the artifact's own `config.example.json` byte for byte — health status
  alone cannot see a missing configuration mount, `/healthz` being configuration-blind by design.
  What is run is the documented default path, which edits nothing and verifies nothing: attestation
  verification is the operator's option rather than a step of the sequence
  ([`DEPLOYMENT.md`](DEPLOYMENT.md) § *Bring-up*), so a run that skips it is a faithful bring-up and
  not a gap. No human is in the loop at any point. Recorded in
  [`../scripts/cases/check-bringup.md`](../scripts/cases/check-bringup.md).
- **The committed recipe carries a restart policy.** `scripts/check-restart-policy.py`, run by
  `just check-restart-policy` in the `docs-and-hygiene` job, failing where a service in
  `deploy/compose.yaml` declares no policy or declares one other than `unless-stopped` — the value
  is the assertion rather than the key's presence, `always` being the wrong one for the reason
  [`DEPLOYMENT.md`](DEPLOYMENT.md) gives. A recipe it can find no service in fails as well, a run
  that examined nothing not being a clean one. Recorded in
  [`../scripts/cases/check-restart-policy-py.md`](../scripts/cases/check-restart-policy-py.md).
  **It gates that one key deliberately and no others**: the key is the residue of a
  requirement deleted on #69 tree rebuild, not the beginning of a recipe linter. Every other value in
  the recipe is a sample default an operator is expected to weigh and change
  ([ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md)), and gating one would
  assert a recommendation as an obligation.
- **The image reports its health in both directions.** `scripts/image/health_signal.py`, run by
  `just check-image` in the `image-tests` job, runs the argument vector the image's own
  `HEALTHCHECK` declares — rather than a command restated in the check — in a container that is
  serving and in one where nothing is listening. A test that only ever observes the healthy state
  cannot tell a working signal from a hardcoded one. An image declaring no `CMD`-form healthcheck
  fails rather than reading as a signal in neither direction, and a vector docker could not execute
  at all (125, 126 or 127) is reported as having judged nothing rather than as a correctly reported
  unhealthy state. Recorded in [`../scripts/cases/check-image.md`](../scripts/cases/check-image.md).
- **The example configuration is one the page accepts.** It is loaded in the page and asserted to
  render the configured display with no validation report. Asserting that the tag carries the file
  decides its presence and nothing about its content, and the bring-up check above passes over a bad
  one: validation runs in the page rather than the backend
  ([ADR 0007 rev 2](decisions/0007-config-validation-allocation.md)), so a stale example still serves.
  A schema change that leaves the example behind fails here or nowhere — and the example is what the
  documented procedure tells an operator to copy, so it is the first configuration most deployments
  ever run. **In the page** is the load-bearing part: a schema validator run as a step here would be a
  second implementation of the schema's rules, which that ADR forbids. This exercises the one engine
  rather than authoring another. Recorded in
  [`../scripts/cases/check-render.md`](../scripts/cases/check-render.md). A sibling project,
  `frontend/playwright.policy.config.ts` (`just check-render-policy`), builds the same render tier
  from `vite.config.render.ts` but previews it rather than serving it from the dev server, under the
  backend's served headers (#266 security response headers): it runs the example-configuration and
  emission specs and fails on a console warning of a rejected policy directive or an unrecognised
  Permissions-Policy feature.
- **An image swap preserves the deployment.** `just check-image-swap`, run by the `image-swap` job in
  [`../.github/workflows/publish.yml`](../.github/workflows/publish.yml): the release under test
  swaps for the newest older non-pre-release, both under the same mounted configuration and
  ephemeral published port — a secret directory is #261 secret mount's, this check is config-only.
  Each digest runs, is asserted healthy and serving that configuration, and is
  stopped and removed before the other runs, with no builder invoked at either step — made observable
  by asserting each running container's image resolves to the digest requested. Reporting a changed
  version is the OCI `org.opencontainers.image.version` annotation on the manifest each digest names:
  the two must differ, and each must equal its own release tag with the leading `v` stripped. With no
  older non-pre-release to compare against, the job's `previous` step writes an empty tag and the
  swap step's own `if:` condition skips it, so the job passes without invoking the check. Recorded
  in [`../scripts/cases/check-image-swap.md`](../scripts/cases/check-image-swap.md).

**All five are built.** The recipe and health-signal checks landed with #54 container build and
publish, against the image and the recipe that ticket ships; the example-configuration check
landed with #139 example-configuration check, against the page it renders; the
documented-procedure check landed with #138 bring-up check, against a published release rather
than the tracked tree; the image-swap check landed with #140 image-swap check, against two
published releases — which is how this project records scoped work
([ADR 0005 rev 4](decisions/0005-traceability-gating.md)); what each asserts
was decided by #71 release artifact set, which shipped no code.

## The exception register

A committed register, `.vulnerability-exceptions.json` at the repository root, one entry per finding.
Plain JSON rather than YAML: every plain-`python3` check in this tree is stdlib-only by decision, and
no venv wired into either target job carries a YAML parser. **There is no severity threshold.**

A threshold decides in advance that a whole class of finding is acceptable, sight unseen. That is the
wrong shape here: a low-severity advisory with a published patch is a dependency shipping something
broken, and the answer is to take the patch or take a different dependency. The register replaces a
standing threshold with a decision per finding.

The register is a JSON list; each entry is an object carrying exactly five fields — `advisory` (the
finding's id, matched against a scan's primary id or any alias — a Go finding's OSV id or its GHSA/CVE
aliases, an npm finding's GHSA id, a Trivy finding's own `VulnerabilityID`), `scope` (`go`, `npm` or
`image` — which recipe's findings the entry can cover), `no_fix_because`, `no_alternative_because` —
**why no alternative dependency or base image is viable**, what makes an entry a decision rather than a
suppression: an entry that cannot state it should have been a version bump or a replacement — and
`review_by`, an ISO 8601 date evaluated at check time (today ≤ `review_by` ≤ today + 90 days, UTC). The
last is what stops *accepted for now* becoming *accepted permanently* with no moment at which anyone
looks again — an entry outside that window fails, which puts every live exception in front of a person
quarterly. Committed `[]`.

The gate asserts: every entry is complete and current; every finding suppressed in scan output has a
matching entry; no entry matches a first-party finding — a Go package path under this repository's own
module, or the npm project's own package name — or a Go standard-library finding, none of which has an
exception path regardless of currency (§ *Dependency vulnerabilities*); and no entry exists for an
advisory no scan reports — so the register cannot accumulate rows for problems that no longer exist.
**An image finding is always eligible for a register entry** — nothing in a built image is this
project's own source, so scope `image` has no first-party exclusion to apply. Each of these is decided
**within the scope being checked**:
`check-vulns-go` examines only `scope: "go"` entries against what govulncheck reports,
`check-vulns-npm` only `scope: "npm"` entries against what npm audit reports, and the image scan only
`scope: "image"` entries against what Trivy reports — so validating the whole register needs all three
run, which they do, as three separate CI steps. An entry whose own `scope` is missing or is none of
`go`, `npm` or `image` cannot be routed to any of them this way, so it fails **all three** rather than
falling through unexamined by any.

Recorded in [`../scripts/cases/check-vulns-go.md`](../scripts/cases/check-vulns-go.md),
[`../scripts/cases/check-vulns-npm.md`](../scripts/cases/check-vulns-npm.md) and
[`../scripts/cases/check-vulns-image.md`](../scripts/cases/check-vulns-image.md).

## Documentation integrity

The documentation set is checked for the failures that make it untrustworthy: a link that does not
resolve, a citation to something that does not exist, an index that has drifted from what it indexes.

- Every relative Markdown link in every tracked file resolves inside the repository, decided by
  `lychee` run from its digest-pinned official image over the `git ls-files` Markdown set
  ([ADR 0016 rev 11](decisions/0016-maintained-tools-for-standard-artifacts.md)). All three syntaxes
  carrying a relative path are read — the inline form, a link-reference definition, and a raw HTML
  anchor's `href` — each held against the retired authored check's recorded cases, in both
  directions. A destination may be angle-bracketed or carry a title; neither is part of the path.
  **The gate runs `--offline`, and that is a decision rather than an inherited default.** Online
  checking makes third-party availability a merge condition, which § *Upstream contract checks*
  refuses for its own gates for the same reason; offline buys that stability by checking absolute
  `http`/`https` links not at all. With the host allowlist retired (ADR 0016 rev 11), an absolute
  link is entirely unconstrained: documentation may link outward, and nothing here reviews where to.
  **The gate is a CI-only exception rather than a `verify` check**: the image digest is the pin,
  and a contributor machine is not assumed to run docker. A local run is the same invocation — piping `git ls-files
  '*.md'` into `lychee --offline --no-progress --files-from -` — against a release binary of the
  image's version. The population is the tracked set; an untracked Markdown file is outside it, and
  the untracked-file preflight (§ *Repository shape*) is what fails it rather than this gate.
  **What this gate lets through, measured rather than assumed.** A link inside a fenced block — a
  fence is a sample rather than a reference. A fragment naming no heading in a target whose path
  resolves: fragment checking is off, so only the path half of the destination is proven. A
  destination that leaves the repository and lands on a file that exists, by `../` or through a
  tracked symlink: the escape and symlink obligations are retired knowingly (ADR 0016 rev 11), and
  the CI container failing such a link because only the checkout is mounted is incidental, not
  asserted. A fence that never closes runs to the end of its document under CommonMark, so nothing
  past it is scanned and the run reports clean — seeded and confirmed, and the Sphinx build passes
  the same seed; no residue guard is kept, on ADR 0016 rev 11's own bar that one residual obligation
  does not earn a second gate beside an adopted tool. lychee itself reports success over an
  empty input set — the ruling ADR 0016 rev 11 records for adopted tools — but the CI step
  materialises the tracked-file list and fails on a failed or empty listing before lychee runs: a
  failed measurement is not an empty population, and a scan of nothing must not read as clean. A
  root-relative destination fails, with a message naming the missing root rather than a wrong
  reason.
- Every citation to a requirement ID or ADR number names an item or decision that exists — in
  tracked documentation outside `.claude/`, and in every item's `rationale` and
  `verification-justification`. Fenced code blocks are skipped; an identifier in inline code is a
  citation like any other. An identifier followed by `.yml` names an item's file rather than citing
  it: it must still resolve, and carries no header.
- **An identifier is uppercase.** A lowercase or mixed-case spelling is reported as malformed rather
  than resolved, and the same spelling inside an item's `text` is caught as the citation it is.
  Matching only the canonical case would make every other case invisible to both checks instead of
  wrong, which is the failure a resolution check exists to prevent.
- **A rule here cannot spell its own counter-example.** The checks above scan the raw text of every
  tracked Markdown file, and neither a backtick nor a table cell exempts anything: a malformed
  identifier written to illustrate the rule *is* a malformed identifier, and a broken link written to
  illustrate the link rule *is* a broken link. Describe the form that fails; never write it out. This
  binds every document a check reads, not only this one.
- **A requirement citation carries the item's header, in an HTML comment, closed up to the
  identifier.** The header is verbatim and the comment is the only form —
  `SRS015<!-- One schema, all boundary value classes -->`. The identifier's own closing backtick and
  possessive clitic may sit between the two; whitespace may not, because a browser strips the comment
  and leaves the space, which then reads as a gap before whatever punctuation follows. Closing the
  junction also removes the one place a line could break inside a citation, which is what once split
  paragraphs on the rendered page. A citation may still wrap **after** the comment opens: the reader
  normalises whitespace inside the header, so a break and any blockquote marker continuing it are
  inside the header rather than text separating it. A number is only a handle — a renumber rewrites
  `links:` and leaves the sentence pointing at whatever now occupies it, still reading as correct —
  and the header is what turns that drift into a mismatch a machine can see. An ADR number carries no
  header, and the rev beside it pins the version rather than the identity: numbers are reusable, so
  what holds is that the check fails the instant a number is freed. A citation written on a branch
  across a freeing and a re-taking of the same number is the case that leaves, and nothing here
  decides it.
- **A header in an HTML comment does not open a line that continues a paragraph.** CommonMark reads
  a line-initial `<!--` as an HTML block, which interrupts the paragraph and splits it in two on the
  rendered page while the source still reads as one. Nothing else reports it: the comment is a
  comment, the prose is correct, and Sphinx does not warn. A comment opening a line after a blank
  one is a block already, which is what an issue template's guidance comment is, and passes.
- The `decisions/` directory and its index table agree — every ADR has a row, no gap or duplicate in
  numbering, every row resolves to a regular file. A directory listing reports names, so an entry
  named like an ADR is counted as one until something states it; a directory or a dangling symlink
  carrying the name is not an ADR.
- **Every citation of an ADR pins that ADR's current rev**, in prose as `ADR NNNN rev M` and in a
  markdown link as its title. An ADR is a versioned document
  ([`decisions/README.md`](decisions/README.md)), so revving one reaches every document citing it:
  each is updated or re-decided in the same change rather than left asserting a claim the ADR's
  current rev does not make. The link rule is what the prose rule cannot reach — a citation spelled
  as a bare bracketed number names no ADR in prose at all. **It is anchored on the link's target, not
  its title**: a title is arbitrary text and may carry brackets, so a title-shaped pattern is what a
  link to an ADR escapes through. A link whose title cannot be resolved back to its opening bracket
  fails rather than being passed over, which makes a link to an ADR wrapped across two lines an
  error — legal Markdown this rejects, in exchange for a reader that cannot be stepped around.
  **The prose reader recognises more than it accepts, so that each spelling can be rejected rather
  than passed over**: `ADR` or `ADRs`, any case, up to three of space, underscore, hash or hyphen,
  then one to four digits. Only the canonical form is accepted; everything else that set reaches is
  reported, a reference-style link or a raw `<a href>` at an ADR among them. That set is stated
  literally rather than as a universal because a citation the reader does not reach leaves the
  population and is then reported success over — so what the set *is* is the reviewable claim, and a
  separator outside it is a gap to close rather than a rule already broken.
  The ADR's own head is authoritative for its rev, the index table's column is checked against it, its
  title's number must match its filename's, and its *Revisions* section carries one changelog line per
  rev from 1 — so a rev cannot be taken without recording what changed, and a number two files carry
  is a collision rather than a silent choice between them. An ADR declaring no rev, or two, fails
  rather than being read as rev zero. **Three empty-population guards, one per reader**, because
  either reader falling to zero means it stopped seeing citations while the other kept the run green.
  **One exemption, and it drops staleness alone:** a changelog line and any line continuing it records
  what a rev did at the moment it did it, so a citation there is not held to the current rev. It is
  held to everything else — it must name an ADR that exists and must carry a rev. An index row's
  leading self-link is dropped for the same reason and no further.
  **What this leaves unproven is whether a citation pinning the current rev still means what the ADR
  says.** The pin is gated and the claim hanging off it is not, so a sentence describing what an
  earlier rev said passes green. That is read at review — see § *What is not gated here*.
  **A citation split across a line break is not judged at all**, in either direction: matching is
  within one line, so a line ending in the bare keyword and the next opening with the number match
  neither pattern, and that citation is exempt from the rule for as long as the wrap survives. It is
  a hole a prose pass can *open*, since rewrapping is what one does. Held by review alone.
- **Every tracked Markdown file is claimed by a row in the documentation index**, unless it sits under
  a top-level dot-directory, which holds machinery rather than documents. Both sides are derived from
  the repository rather than read from an inventory: adding a document without indexing it fails,
  retiring one fails until its row goes, and there is no exclusions list to append to — excluding
  anything that is not machinery by that rule takes a change to the check itself
  ([ADR 0014 rev 4](decisions/0014-documentation-index-claims-documents.md)). A row whose *Document* cell
  renders with a trailing slash claims the subtree beneath it, which is how `decisions/`,
  `requirements/`, `contracts/`, `architecture/` and `site/` are covered by one row each. Every row's
  link resolves to a tracked file, one rendered path carries one row, a row names a tracked document
  or a directory holding one, no row indexes a dot-directory, and no *Guarantees* or *Excludes* cell
  is empty. The index does not index itself. A new top-level dot-directory is excluded the moment it
  exists, with no edit anywhere — the trade ADR 0014 rev 4 records, which is why the check names the
  machinery directories it skipped on every run. A dot-prefixed file at the repository root is not a
  directory and must be indexed like any other document. Scope is Markdown: the tree's items are
  claimed by the tree and gated by `check-reqs`, and code is claimed by nothing here.
- The LikeC4 architecture model validates: no undefined element, no unresolved relationship.
- Spliced diagrams and generated architecture artifacts are byte-identical to a regeneration. A
  marker's artifact resolves to a regular file inside `architecture/` — through any symlink, not
  merely by its path text — and carries no fence marker of its own, which would close the generated
  fence early and spill the remainder into the document as prose.
- `architecture/generated/` holds every artifact the model produces and no other. The export clears
  the directory before codegen, which never prunes, so an artifact left behind by a deleted view is
  byte-identical to what is committed and the staleness diff alone cannot see one; and the diff is
  taken after `git add --intent-to-add`, without which a regenerated artifact nobody committed is
  untracked and so invisible to it — a view whose diagram never reaches the document, reported green.
  **The comparison is the commit against the worktree, so the index is never a party to it**, and the
  export rewrites the worktree before the diff runs: staged content diverging from the worktree is
  let through, and a plain commit then lands the index the gate never read. It is a **local false
  green only** — CI checks out a tree whose index equals its commit, so the divergent state cannot
  arise there — reachable by staging work before running the gate locally.
- Every requirement identifier tagged in the architecture model names an item that exists, is active
  and accepted, and is spelled canonically; every declared tag is applied to something. A tag
  carrying anything other than an identifier fails rather than being passed over — the model's tags
  carry requirement links ([ADR 0019 rev 7](decisions/0019-boundary-at-what-deploys-and-tag-tier.md)), so
  one carrying something else is a decision to take, not an exemption to add. A model naming no
  requirement at all fails too: it resolves every tag it carries, so an absent link and a sound one
  would read identically. A tag counts on four subject kinds — the logical model's elements and
  relationships, and the deployment model's, which the export keeps separate; reading only the first
  pair reports a tag applied to a deployment node as applied to nothing, which is a diagnostic the
  input cannot support. **A tag on a view is deliberately not read**, a view being a projection of
  the model rather than a subject in it, so one applied there fails as applied to nothing — the right
  verdict, reached by a message that does not explain it. What this leaves unproven is whether the
  tagged element is the one that requirement obliges, and whether the tier suits the level; both are
  read at review.
- **Every accepted, active `SYS` or `SRS` item is tagged somewhere in the architecture model**, on an
  element or a relationship ([ADR 0019 rev 7](decisions/0019-boundary-at-what-deploys-and-tag-tier.md)).
  There is no exemption record and nothing to add an item to: where one can bind nowhere, the model
  grows to draw what it obliges. The population is decided rather than filtered — a tier outside the
  obliging and verification sets fails, and so does a `status` outside `accepted` and `proposed`,
  because comparing against `accepted` alone reads a mis-spelled status as *not accepted* and drops
  the item from the check's own population. `TST` is outside the rule, a verification item saying how
  an obligation is settled rather than what the software owes; a `proposed` item is outside it
  because the direction above resolves a tag only to an accepted item, so one cannot be tagged; a
  retired item — `active: false`, `status` untouched — is outside it because it obliges nothing. A
  tree that loads no item, one whose obliging tiers are absent, and one where no item is accepted and
  active each fail rather than reporting complete allocation over nothing judged. What this leaves
  unproven is the same thing the direction above leaves unproven, and it is the cheaper pressure this
  rule creates: an item can be tagged onto an element it does not oblige, and the check reads that as
  bound. Held by review alone.
- The documentation site builds under Sphinx with warnings-as-errors. **That is what it asserts, and
  not that the site is internally consistent**: `conf.py` suppresses the missing-cross-reference
  warning, so the MyST reference forms are silently dropped where Sphinx's own role still fails, and
  the index's globbed toctrees adopt any new top-level document so nothing orphan-warns. Both are
  configuration choices; the link half of consistency is the link checker's above.

**Considered and rejected:** a registry mapping each canonical document to the path globs it
describes, failing a change that touches described code without touching its document. Its obligation
is *update the document **or declare it unaffected***, and a gate satisfied by declaring is a
checkbox — a typo fix trips it, so the declaration becomes reflex. It would also not have caught the
staleness that prompted it: the documents that went stale did so because requirement identifiers
changed, which the citation resolver above decides without anyone declaring anything.

## Repository shape

- **Every tracked file is a declared kind** — an authored program in the set
  [ADR 0017 rev 9](decisions/0017-authored-language-set.md) states, a derived format a toolchain
  requires, data an authored check reads, or documentation. **A file type nobody has decided about
  fails**, which is the point: closing that failure is a person deciding which side it falls on. A run
  resolving no tracked file fails rather than reporting a clean tree.
  **The languages that author nothing are not declared extensions.** `sh` and `mjs` author nothing
  under that record, so a *new* file in either fails; the files predating the decision are
  grandfathered **one path at a time**, each naming the record that gives it a disposition, and a
  grandfathered path that stops being tracked fails too — its disposition landed, so its entry goes
  with it. Declaring the extension instead would let the next such file through forever, which is the
  one thing this check exists to prevent.
  **What it leaves unproven** is whether a declared file is classified *correctly*: a Python program
  labelled derived passes, and only a reader catches that.
- **No file git treats as text has CRLF line endings**, and `.gitattributes` decides which those are.
  A glob given the `binary` attribute is exempt from CRLF→LF normalisation at add time *and* skipped
  by the search, so CRLF-terminated text under one commits and survives a fresh clone unseen. The
  attribute is declared on the image, font and PDF globs, which is the attribute used as intended; a
  *text* glob given it is the reachable case, and the owner ruled on 2026-08-02 not to gate it.
- **No untracked, non-ignored file is present when a `git ls-files`-population gate runs.** Those
  gates read tracked state only, so an untracked file is invisible to every one of them — a defect
  in exactly the file most likely to carry a fresh mistake passes a local run and fails in CI once
  committed. Two mechanisms, deliberately redundant: a preflight, `just check-untracked`, fails on
  any untracked, non-ignored file in the tree, and each gate whose population is `git ls-files`
  independently fails on an untracked file of the kind it inspects — so neither protection rests on
  remembering to run the other. The remedy is `git add`, after which the gates judge the file, or a
  gitignore entry, which declares it not material. A CI checkout holds only tracked files and every
  build artifact is gitignored, so neither mechanism fires there; what they gate is the local run,
  the one place an untracked file exists. `check-adr-index` carries no guard: it reads the
  decisions directory rather than the tracked set, so an untracked ADR is judged, not skipped.
  `adr-rev-reach.py` also reads the tracked set and carries no guard either — it reports and exits
  zero by design (§ *What is not gated here*), so there is no verdict for an untracked file to
  escape.
- **Every hook in the local hook layer passes.** `.pre-commit-config.yaml` is that layer
  ([ADR 0016 rev 11](decisions/0016-maintained-tools-for-standard-artifacts.md)): no private key, no
  file over `check-added-large-files`' threshold, every YAML and JSON file parses, no merge-conflict
  marker committed while a merge is in progress (outside one, `check-merge-conflict` judges nothing —
  a bare `=======` is also a Markdown setext underline), no file mixing line-ending kinds — plus the
  authored `check-eol` hook, which carries the
  tracked-tree CRLF scan of the CRLF bullet above, scoped to what `mixed-line-ending` cannot reach:
  that hook judges only the files pre-commit hands it and counts one uniform ending kind as unmixed,
  so a committed-but-unstaged CRLF file and a uniformly-CRLF file both pass it. Its population is
  `git ls-files`, so it carries the untracked guard of the bullet above — an untracked, non-ignored
  file is reported as unsearchable and fails, beside any CRLF finding rather than in place of one.
  Locally the layer is
  advisory fast feedback at commit, on the commit message, and at push (`pre-commit install`, once
  per clone); the binding run for the file-scanning hooks is CI's, over every tracked file — the
  commitlint hooks of the bullet below run at their own stages, not in that run. **A hook whose file set is empty is skipped and reports
  success** — pre-commit's own behaviour, which ADR 0016 rev 11's empty-population ruling permits;
  the authored hook runs regardless of the file set (`always_run`) and reports success over an empty
  tracked tree, under the same ruling. pre-commit itself is pinned in `scripts/requirements-dev.txt`
  — installed into `scripts/.venv` by `just hooks-install` and by CI's install step, and covered by
  Renovate's `pip` manager; each hook repository is pinned by commit SHA in the config, which no
  Renovate manager covers — its `pre-commit` manager stays off — so `pre-commit autoupdate`, run by
  hand, is its updater.
- **The pull-request title is a Conventional Commit, and so is each local commit message** — one
  obligation at two stages, delegated to `commitlint`
  ([ADR 0016 rev 11](decisions/0016-maintained-tools-for-standard-artifacts.md); the gate itself is
  [ADR 0006 rev 6](decisions/0006-process-gates.md)'s). Both stages run through the hook layer's
  `repo: local` hooks and one base configuration, `.commitlintrc.json` —
  `@commitlint/config-conventional`, plus a `scope-case` rule restoring the retired check's
  lowercase-scope refusal, which the preset does not carry. The `commit-msg` hook is the
  advisory local stage: its `defaultIgnores` pass `fixup!`/`squash!`/merge subjects, whose text the
  squash discards. The manual-stage `commitlint-pr-title` hook, run only by CI's `process` job,
  reads `.commitlintrc-pr-title.json`, which extends the base with `defaultIgnores` off, because
  the squash makes the title the commit on `main` — the same string accepted at one stage and
  refused at the other, recorded in both directions in
  [`../scripts/cases/commitlint.md`](../scripts/cases/commitlint.md), beside what the rules let
  through that the retired regex refused: an empty scope, which no rule can refuse without also
  refusing a scopeless header. The npm packages are
  pinned by exact version in the hooks' `additional_dependencies`, written once as a YAML anchor;
  neither Renovate nor `pre-commit autoupdate` reaches such a pin, so those versions are updated
  by hand, beside the hook-repository revs `autoupdate` does move.
- The branch is named `type_number-snake_name`, links an open issue labelled with its type, and its
  default-base pull request records the ticket linkage.
- That issue carries a milestone and **exactly one** type label. The type set is read from
  `scripts/branch-shape.regex` rather than restated, so a new branch type cannot leave the label rule
  behind. A second type label makes the branch type ambiguous, and an unmilestoned ticket is absent
  from the phase axis that carries the definition of done. This is detected at merge, on the change
  whose ticket is wrong, rather than at filing: GitHub cannot refuse to create a malformed issue, and
  CI does not write ([ADR 0013 rev 4](decisions/0013-work-tracking-invariants.md)).
- A pull request's base and its issue's parent agree. A parent implies a non-default base; an
  integration branch implies membership in the ticket anchoring it; and a non-default base that is
  not itself a conforming branch fails rather than skips, because an anchor the check cannot resolve
  must not read as success. Sub-issue membership means a shared merge target — topical grouping is
  the milestone's job.
- The Docker build context excludes `.git` and `node_modules`, which the committed `.dockerignore`
  does, along with `.github/`, `.claude/`, `docs/`, the frontend's build and test output, and the
  `scripts/` virtualenv. That is the whole of what it excludes rather than everything a build of the
  two source silos reads nothing from: `deploy/`, `boundary/`, the rest of `scripts/`, the justfile
  and the root's Markdown are unread and in the context. Neither `.git` nor `node_modules` is secret
  material; both are build hygiene, and a smaller context is a faster and more predictable build. No gate reads that file — this line rather than a check is what records the exclusions, and
  what the image carries is decided by the tier over the built image instead
  (§ *Image tests*). That the image holds no secret is the tree's, under
  SRS025<!-- No secret material in the published image --> (#54).
- No `justfile` recipe carries a shebang. A recipe is a list of commands; shell control flow is a
  script, and a script is siloed under `scripts/` where a case can be recorded against it, like every
  other kind of tooling. Detected from `just`'s own dump rather than from the file's text, so the
  shape is whatever `just` resolves it to.
- A depth-1 listing of the repository root holds no `package.json`, `go.mod`, `pyproject.toml`,
  `requirements*.txt` or `.venv/` — tooling is siloed with the feature it serves. The root
  `renovate.json` parses as JSON and its `extends` list names the `github>WisewareOrg/wise-renovate`
  preset, pinned to a tag — the one dependency-update configuration the repository carries directly,
  the policy itself living in the preset the runner repository publishes. The gate stops at that: it
  does not verify that a manifest exists wherever a dependency lives, because Renovate discovers
  manifests on its own, with no per-directory configuration entry for a gate to check against.

## Action pins and workflow privilege

The workflows are themselves a supply chain and themselves privileged. Both are audited from the
files by two maintained tools ([ADR 0016 rev 11](decisions/0016-maintained-tools-for-standard-artifacts.md)),
each run in CI from a digest-pinned official image over the `.github/workflows` input set: `zizmor`
at the `pedantic` persona for what a workflow may do — action pinning, permission grants, credential
persistence and template injection among its audit set — and `actionlint` for whether a workflow is
well-formed at all: schema, expression and reference errors, with `shellcheck` and `pyflakes` over
`run:` scripts. The audit is CI-only (§ *Gate wiring*): the images run in CI, and no local install
channel is decided.

- **Every action is pinned to an immutable reference** — a commit SHA, or an image digest where the
  step is a container. A tag is a pointer its owner can move after anyone reviewed it; neither of
  those is. A `uses:` beginning `./` is exempt: a repository-local action or reusable workflow moves
  with the commit that calls it, so there is no upstream to pin.
- **No workflow grants a write permission at the top level, and no grant goes unexplained.**
  `excessive-permissions` fails a top-level write grant, and fails a workflow declaring no
  `permissions:` block at all — confirmed by seeded fixture
  ([recorded](../scripts/cases/workflow-audit.md)) rather than assumed — because what an
  undeclared block would inherit is a repository setting no check here can see. A job needing more
  elevates in its own block, which is what confines `pages.yml`'s `pages: write` and `id-token:
  write` to the deploy job. The gate is stricter than the authored check it replaced, and the
  repository adapts rather than configuring the tool down: `permissions: read-all` is refused as
  excessive; every grant other than a bare `contents: read` — read grants included — carries an
  explanatory comment beside it; every checkout sets `persist-credentials: false`; every job carries
  a `name:`, and every workflow a `concurrency:` group.
- **An unreadable workflow fails rather than being skipped.** Both tools parse real YAML, so a layout
  that cannot be read is a syntax error, not a skip — GitHub itself would refuse the same file. A run
  discovering no workflow at all fails too, as `zizmor`'s own behaviour (`no inputs collected`)
  rather than an obligation on it (ADR 0016 rev 11's empty-population ruling).

**What the gate deliberately lets through.** The `# vN` version comment beside a pin is retired as an
obligation (ADR 0016 rev 11): a stale or absent comment passes, and the adopted
`helpers:pinGitHubActionDigests` preset maintains the comments only on the bumps it performs.
The audits needing the GitHub API (`known-vulnerable-actions` and `ref-version-mismatch` among them)
do not run: the gate runs the offline audit set, deterministically, so a verdict moves only when a
workflow or a pinned image does.

**The repository-level default is not decidable here either.** `GITHUB_TOKEN`'s default permission
sits behind the same admin-only API as the settings in § *Secret scanning*. It is read-only; the
top-level blocks are what a check can see, and they are what the rule above constrains.

## Module and framework structure

These keep a module self-contained and the shared framework ignorant of it. They were verification
items in the tree until the extensibility need above them dissolved — its children were architecture
([ADR 0012 rev 2](decisions/0012-module-requirements-in-tree.md)) — and nothing the running kiosk does can
violate any of them, so they are checks here rather than obligations there.

- **Shared framework code names no module.** No shared framework source names any module outside the
  static registration file, and no shared framework package imports a module package, in either the Go
  import graph or the frontend module graph. Runs on every commit rather than only on a module-adding
  change: a diff-scoped form cannot reliably classify which changes are module-adds, and passes
  vacuously on the rest while shared code accretes module knowledge (#12).
- **Route registration has one call site.** A module is registered in exactly one file, which declares
  the registry as a package-level struct type carrying one embedded field per upstream-backed module
  and appearing in no append, index assignment, or map insertion — a registry is exactly a list
  something writes to at run time, so its absence is structural rather than a denylist of
  registry-shaped names. A schema route no field serves is a compile error where that value meets the
  generated server interface, so the discovery case is closed by the type checker rather than by
  comparing two sets (#9).
- **Shaping packages are pure by construction.** Each module's shaping package resolves a transitive
  import set that is a subset of a declared pure-package allowlist, so I/O is absent by construction
  rather than by a denylist of forbidden packages. No exported shaping function's parameters include
  the secret type ([ADR 0023 rev 2](decisions/0023-secret-output-containment.md)) or the URL-builder's
  output type, and the shaping unit tests run against a transport
  that panics on use (#12).
- **Exactly one non-test reference unwraps the confined secret type.**
  [ADR 0023 rev 2](decisions/0023-secret-output-containment.md) makes the secret type unemittable and
  then rests the structural half of that on the unwrap being singular and reviewable — *"the sole call
  site that unwraps it to the raw value is guarded by a lint"*. This is that lint: over the tracked,
  non-`_test.go` Go files under `backend/`, references to the type's unwrap method are counted and any
  count other than one fails, naming every site. Test files are exempt by decision — the unit tier
  proving the redaction paths cannot do so without unwrapping — which is also the gate's widest hole:
  a leak reachable only from a test file is outside the population. The match reaches a method *value*
  as well as a call, so an unwrap aliased behind a variable is counted rather than escaping the count,
  and it is textual, so a mention in a comment or a string literal fails rather than passes.
  **Two empty-population cases fail rather than reading clean**: no non-test Go file under `backend/`,
  and a tree in which the method is not declared at all — a check keyed on a name finds nothing once
  the name is renamed, and that is indistinguishable from a compliant tree unless it is refused.
  Recorded in [`../scripts/cases/check-secret-unwrap.md`](../scripts/cases/check-secret-unwrap.md).
  **What it leaves unproven**: that a secret is not emitted. This counts unwrap sites and reads
  nothing about what the one site does with the value — redaction through every formatting path is the
  `secret` package's own tests', and the behavioural edge is the canary
  ([ADR 0023 rev 2](decisions/0023-secret-output-containment.md) composes the two). It also decides
  nothing about *where* the site sits: singularity is what the ADR obliges, so moving the unwrap to
  another package passes, and only a second one fails.
- **The frontend build emits a static bundle**
  ([ADR 0018 rev 1](decisions/0018-frontend-svelte-vite-static-spa.md)). Exactly one HTML entry whose
  mount element is empty, no server-entry chunk, no SSR target or adapter declared in the build
  configuration, and the npm packages in the emitted module graph a subset of a committed allowlist
  manifest. The allowlist is deliberate: a denylist of named routers and meta-frameworks fails open
  the first time someone hand-rolls a hash router, and any new runtime dependency should fail until
  it is reviewed. **It is read in both directions**: a granted package that reaches no emitted module
  fails too, so a grant cannot outlive the dependency it was made for.
  The module graph is Rollup's, written out by the build — the emitted chunks carry no package names,
  so a check reading only the emitted tree could not decide which packages ship, and *no graph* and
  *an empty graph* are failures rather than skips, a subset test over nothing being satisfied by
  every allowlist there is. That the gate can fail, in each of those directions, is proven once and
  recorded in [`../scripts/cases/check-static-bundle-py.md`](../scripts/cases/check-static-bundle-py.md).
  **What it leaves unproven**: the SSR reading is textual, over the Vite configuration and the plugin
  modules it is composed from, so a target injected from outside that set is outside the population;
  and the allowlist is a package set, saying nothing about how much of a granted package ships.
- **No backend code builds an upstream URL outside a module's shaping library.** The URL a module
  fetches is that module's to construct; shared framework code constructing one is shared code
  holding module knowledge (#9).
- **Module directories and test files stand in bijection.** Every module has a render test beside its
  component, and every module registered against an external source additionally has a unit test
  beside its shaping library. A module with no registration entry is a local module and is not
  expected to have one. What those tests must cover is
  [`TESTING.md`](TESTING.md)'s; that they exist and sit where the runner reaches them is decided here
  (#12).
- **A module component's module graph does not reach the configuration source.** A component receives
  what it needs as props; reaching the configuration itself would let it depend on keys nobody
  declared for it (#12).

## Upstream contract checks

Two jobs, deliberately unequal. What they must prove, and why the pair is composed rather than either
alone, is [`TESTING.md § Where the Contract tier runs`](TESTING.md#where-the-contract-tier-runs-and-how-it-reaches-upstream);
what runs where, and what each is allowed to let through, is here.

- **Recorded upstream fixtures are replayed on every commit, and block a merge.** Each shaping library
  is asserted against a captured response. No credential is present in this job, and no network call
  is made. What it cannot catch is an upstream that changed after the fixture was recorded — a
  fixture is a snapshot, and it stays green against a reality that has moved.
- **A scheduled job runs the same shaping libraries against the live upstreams, off the pull-request
  path.** It is the only thing that detects the drift above. It fails a scheduled run, never
  somebody's change: upstream availability is not a merge condition, and a gate that goes red because
  a third party is having a bad afternoon is a gate authors learn to ignore.
- **That scheduled job holds every upstream credential the module roster needs, and it is the only
  job that does.** Which upstreams those are is [`../README.md`](../README.md)'s roster and each
  module's registration entry; a count here would be falsified by adding a module, by someone with no
  reason to open this document. The credentials are scoped to that workflow and reach no other job,
  and no fixture, log or failure output carries a value —
  [`../SECURITY.md`](../SECURITY.md) rests on that, and
  SRS008<!-- No secret value in any backend output --> obliges the running system to the same rule.

**Why a credential is allowed here at all.** A withdrawn requirement once forbade any CI workflow
from holding an upstream credential. It banned a normal practice, and forced the tier into a nested
module that [ADR 0010 rev 3](decisions/0010-runtime-materialised-gate-fixtures.md) independently found
leaky. Holding it in a scheduled job, off the merge path, is the narrower answer.

Unbuilt; owned by #99 upstream contract checks.

## Gate wiring

These assert that the regime is real rather than declared. Each states a property of machinery
already decided, which is what makes it a check and not a want.

- Every check `just verify` depends on runs in CI **as its recipe**: a job step invokes
  `just <recipe>`, after an explicit `Install …` step per toolchain and a `setup-just` step carrying
  a `just-version:` pin, so the executor CI depends on is a decided version rather than a float. The
  recipe body is the single spelling of each check — what runs locally and what runs in CI are one
  text, so the two-spellings drift an inspection gate would re-verify cannot arise, there being no
  second spelling to drift (#101 CI-invokes-just, which retired the authored comparison check with
  that defect class). What enforces the wiring is execution itself: a step invoking a recipe the
  justfile does not define fails its job loudly, and a recipe edit is an edit to what CI runs.
  **The CI steps below are exceptions with no local form**, named here and machine-checked nowhere:
  secret scanning (gitleaks — walks history a checkout's tree does not carry), the PR-title
  commitlint run (no PR title exists locally), the lychee link check, the zizmor and actionlint
  workflow audits (each run from a digest-pinned image; docker is not assumed on a contributor
  machine, and no local install channel is decided), CodeQL (§ *First-party source scanning*
  — the action provisions its own CodeQL CLI bundle, and no local invocation is decided), and the
  image vulnerability scan (`check_vulns.py --scope image` — needs the daemon-built image and the
  network, so it carries no `just` recipe of its own, unlike its sibling scopes below).
  **A different shape holds `check-vulns-go`, `check-vulns-npm`, `check-image` and `smoke-native`:**
  each has an ordinary `just` recipe, so this bullet's own claim still holds for them, but none is a
  `just verify` dependency. The vulnerability pair needs the network on every run and `verify` stays
  offline (§ *Documentation integrity*, § *Upstream contract checks*); the image pair needs Docker and,
  for one architecture, emulation, which a contributor machine is not assumed to carry. Each instead
  runs as a blocking step in its own CI job.
  **What watches the justfile is CI itself and nothing else** — an accepted, recorded loss of the
  retirement. A recipe edit changes what CI runs with no independent reader to disagree, and a CI
  step added outside both categories — recipe invocation and the exceptions above — fails nothing
  and is caught at review or not at all.
  **What execution does not decide:** nothing maps a recipe to the toolchain its job installs — the
  `Install …` steps must still be right, per job — though a toolchain a recipe needs and a job lacks
  fails the run rather than passing a text comparison. A tool a check *invokes internally* is still
  declared nowhere any gate looks: `check-repo-silo.py` reads `just`'s dump, and only its job's own
  setup step says so. And the pinned CI `just` is compared against nothing local — a contributor's
  `just` is whatever they installed, and skew between the two surfaces only where execution
  semantics differ.
- Every committed test file falls under a configured runner's reach; a file excluded by skip, build
  tag, glob gap, or wrong directory fails. The requirements tier is covered by `doorstop --error-all`
  and is deliberately not re-encoded here.
  **It is a discovery diff, not a second set of globs.** The population is the tracked files whose
  *names* say they are tests — `*_test.go`, and the frontend's `*.test.ts`, `*.spec.ts`, `*.test.tsx`
  and `*.spec.tsx` — repository-wide and deliberately broader than any runner's own configuration,
  so a glob gap is a difference rather than a definition. The reach is the union of what each runner
  answers when asked what it would discover: `go list -json`'s `TestGoFiles` and `XTestGoFiles`,
  `vitest list --filesOnly`, and `playwright test --list`. Asking the runners is what makes the
  answer faithful — a build tag, a host filename suffix and a config-level exclude are already
  applied in what they say, where a check restating their globs would decide none of them.
  A source-level `t.Skip` is outside this: the file still compiles and is still discovered, so it is
  reached, and a test that skips at run time is the tier's business rather than this gate's.
  **There is no per-file allowlist**, by [ADR 0010 rev 3](decisions/0010-runtime-materialised-gate-fixtures.md)
  — a file that stops being reached is closed by wiring it to a runner or by deleting it, never by
  an entry beside it.
  **Both ends fail closed.** A discovery command that cannot run leaves the reach unmeasured, and
  that is reported rather than a difference computed against a partial set: an unreachable runner
  must read as neither a tree of dead files nor a clean one. A population that resolves to no test
  file fails too, a run that judged nothing not being a clean tree. Recorded in
  [`../scripts/cases/check-dead-test-py.md`](../scripts/cases/check-dead-test-py.md).
  **What it leaves unproven** is that a reached test asserts anything: discovery decides that a
  runner would execute the file, and what the tiers must guarantee is [`TESTING.md`](TESTING.md)'s.
- The default branch's required status checks equal the gate jobs the workflow defines — a gate job
  absent from the required set fails, and so does a required entry naming no defined job. **`main`
  merges through a merge queue, and administrators are bound by it**: the queue builds and merges
  one pull request at a time, re-running the required checks on that pull request applied to the
  current `main`, so nothing merges on a stale green and no pull request's green depends on another
  not yet merged; there is no role that merges past a red gate. Batching is rejected — every pull
  request stands alone — and the throughput that costs, one `checks` run per merge, is accepted.
  Strict up-to-date protection is off, the queue being what it protected. These are repository
  settings rather than files, so no check here decides them — the same standing as the
  secret-scanning settings above, and this line rather than a gate is what records it.
  **§ *First-party source scanning*'s required set names five checks**, observed on its own first
  run rather than assumed: the four matrix legs `codeql (go, autobuild)`,
  `codeql (javascript-typescript, none)`, `codeql (python, none)` and `codeql (actions, none)`, which
  fail only on an execution error, and GitHub's own `CodeQL` check (`GitHub Advanced Security`),
  which is what fails on a finding at the repository's configured severity threshold.

The requirements tree's own integrity checks run here too, but what they assert is a property of the
specification rather than of the repository, so they are stated where the specification is:
[`requirements/README.md`](requirements/README.md).

**A passing `check-reqs` run also prints the proposed-item backlog** — the count of `proposed` items
and their identifiers, per tier, against the population each tier holds.
[ADR 0005 rev 4](decisions/0005-traceability-gating.md) makes the tree the backlog and the backlog a
report, never a blocking failure, so this line reports and exits zero whatever the tree holds — like
`just rev-reach` below, it is not a gate, and what the tree holds never moves the run's exit status.
Two shapes it asserts against its own output: a tier with nothing proposed prints its zero rather
than dropping the line, because a message that disappears at zero cannot be told from a report that
did not run; and an item whose `status` sits outside the stored vocabulary is listed rather than
silently counted as baselined, though failing on one is `check-arch-trace`'s to do and only for the
obliging tiers.

## What is not gated here

**Review obligations.** An obligation on an author that leaves no artifact is answered by a reader, in
[`../CONTRIBUTING.md`](../CONTRIBUTING.md)'s checklist, rather than by anything here
([ADR 0011 rev 2](decisions/0011-requirement-or-convention.md)). The pull-request template points
there. Two of those questions have a mechanical artifact to read from, deliberately: the documentation
index this document's gates hold to the tree, and the list of every ADR citation a change re-pinned
without touching the sentence around it, which the `pre-push` hook prints whenever a branch revs one
and `just rev-reach` gives on demand. Neither is a gate — the second reports and exits zero, sits
outside `just verify`, and is named so that it does not read as one. A blocking form was weighed and
refused: a rev that does not touch what a citation asserts is the ordinary case, so failing every
pin-only edit is fail-closed on legal input, and the exemption list it would grow is where a bypass
gets spelled.

**The product's obligations.** What the software must do is in
[`requirements/`](requirements/README.md), verified by tests that trace to it. A gate here can block a
merge; only the tree can say the system is wrong.

**How work is tracked, beyond the ticket a branch names.** Three things were weighed and deliberately
left ungated ([ADR 0013 rev 4](decisions/0013-work-tracking-invariants.md)). Ordering lives in GitHub's
native dependency edges and never in a `⛔ Blocked by:` line in a body: the practice is adopted, but a
literal-string ban flags the very documents that forbid the line, and is respelled for free — what
enforces it is that there is no second place to write ordering. Whether a ticket should be rescoped
or closed, which close-reason applies, whether a scope is correct, and whether a body's acceptance
condition is any good are judgment, and a gate pretending to decide them would be theatre. And a body
need not carry its template's headings — templates churn, and gating them buys a re-review of every
ticket on each template edit.

**A ticket nobody works.** The shape checks above see an issue only when a branch names it, so a
malformed ticket left in the backlog stays malformed and stays absent from its milestone's progress.
A scheduled read-only sweep over every open issue would close that; it was rejected as more machinery
than a backlog small enough to read by eye has earned, and it is the named remedy if the drift
recurs.

**Test architecture.** Which tier a test belongs to and what that tier guarantees is
[`TESTING.md`](TESTING.md). A tier organises tests; a gate is a condition on merging, and several
gates run no tests at all.

**This document.** Every other canonical artifact has something that fails when it drifts: a
requirement's text is hashed and editing it flags its children suspect, and generated diagrams are
compared against a regeneration. This one has no fingerprint, no check comparing it against the
workflow or the scripts it names, and no co-change obligation — the requirements carrying that were
deleted, and a co-change registry was rejected above. So a gate removed from the workflow, with its
section left standing here, reads as live to every reader and to
[`../SECURITY.md`](../SECURITY.md), which publishes claims resting on it. The correspondence check
that would close it is #77, and it is not buildable until enough of the gates described here exist to
compare against.
