# Test architecture

A **specification**, not a description. It is written before the tests exist, so it says what tests
must *prove* — not what some accumulated suite happens to do. A strategy written first is
enforceable; one reverse-engineered from existing tests only ratifies whatever accumulated. It is
reviewed on a schedule (see [Review cadence](#review-cadence)).

---

## Tiers

Each tier states what it **guarantees** and when it runs.

| Tier | Guarantees | Specified by | Runs |
|---|---|---|---|
| **Unit** | Shaping libraries transform known upstream responses into correct payloads. Pure, fast, no network | [module contract](contracts/module-contract.md), part 4 | Every commit, in CI |
| **Boundary** | The frontend and backend agree on every value that crosses: parameter names *and types*, success payloads, the structured upstream-failure body, the client-error rejection body, and every status code the frontend discriminates on | SYS005<!-- Single-definition internal contract --> / SRS015<!-- One schema, all boundary value classes --> | Every commit, in CI |
| **Integration** | Routes serve; the TTL cache honours its TTL; parameter validation rejects bad input; config validation fails loudly on bad config | SRS009<!-- Every source reachable through the backend, statelessly --> / SRS011<!-- Upstream request rate is bounded, and the bound is not operator-tunable --> / SRS012<!-- Request parameters validated against known-good per-source constraints --> / SRS002<!-- A module-scoped configuration error is reported at that module --> | Every commit, in CI |
| **Render** | Each module renders from its props; the page assembles with a known-good config; and the assembled page is read for the values a viewer depends on: which region each module landed in, emission, type scale, region geometry, the configured edge band, reflow, and overflow; and a backend that stops answering under the served page is reported once for the page rather than by each module that would report for itself, beside a layout the report displaces rather than covers | [module contract](contracts/module-contract.md), part 1 / SRS017<!-- Full-screen assembly at kiosk; reflow, no horizontal scroll, at narrower widths --> / SRS031<!-- Content too large for its region overflows --> / SRS030<!-- Only content is rendered above the emission ceiling --> / SRS032<!-- Readable text is carried at full emission --> / SRS033<!-- Text holds a minimum size against the display, at every resolution --> / SRS034<!-- The laid-out regions keep clear of the display edge --> / SRS035<!-- The masked edge band is the deployment's to declare --> / SRS026<!-- The display says when the backend is gone --> | Every commit, in CI |
| **Secret canary** | A value planted as a source's secret reaches no backend output: not a success body, not an error body, not a response header, not a log line — swept across every route and across the failure path, not only the paths that succeed | SRS008<!-- No secret value in any backend output --> / [ADR 0023 rev 2](decisions/0023-secret-output-containment.md) | Every commit, in CI |
| **Bounded footprint** | A backend driven under sustained mixed load — cached, uncached, rejected and failing requests together — ends where it started on memory, open descriptors and goroutines, so nothing it holds grows across a continuous run | SRS022<!-- A bounded running footprint --> | Every commit, in CI |
| **Image** | The published image is what a deployment may assume: it runs as a non-root user, serves the configuration bind-mounted into it byte for byte and 404s where none is mounted, carries no configuration file and no deployment-specific environment value, declares no shared writable volume and keeps two instances of itself independent, and holds no value matching the committed secret-pattern set in any layer — with a planted canary required to be reported. It comes up serving on each architecture it is built for, which is every architecture a published image carries — an architecture with no published image is the Native tier's — and the health signal it declares reports healthy while the backend serves and unhealthy while it does not | SRS020<!-- Non-root container user --> / SRS018<!-- One generic published image --> / SRS029<!-- One instance, one configuration, nothing shared with another --> / SRS025<!-- No secret material in the published image --> / SRS019<!-- The backend runs on every supported architecture --> / [`DEPLOYMENT.md`](DEPLOYMENT.md) | Every commit, in CI, from `../scripts/image/`: the property and health-signal harnesses over one image in one job, and the smoke test once per architecture, a matrix leg each |
| **Native** | The application built from source comes up serving on an architecture no published image carries: the binary is the architecture and ARM revision it was to be built for, and the cross-compiled backend answers liveness beside the bundle it was pointed at, serves the page from that bundle, and passes the liveness check its own binary carries | SRS019<!-- The backend runs on every supported architecture --> | Every commit, in CI, from `../scripts/native/`: one job builds the bundle, cross-compiles the backend for armv6l, and runs the smoke harness over the pair under the runner's user-mode emulation |
| **Contract** | Upstream APIs still return what the shaping libraries expect | this document | Fixtures every commit, in CI; a live run on a schedule, off the merge path |
| **Fuzz** | No panic and no hang, the latter by a per-input deadline in the target, on crafted bytes fed to the parsers the module contract names for fuzzing: a shaping library's response parser, and a route's request decode and validation, within a fixed ten-second search budget | this document | Every commit, in CI |

The canary tier's guarantee rests on planting: a sweep that never proves the value *reached* the
backend passes on a backend that never held one, so each canary case asserts the upstream saw the
planted value before it asserts nothing else did. The footprint tier's rests on the reverse — a
threshold loose enough to absorb the growth it is watching for reports a bound it did not measure, so
what it can and cannot resolve belongs in the item, not in a comfortable margin. The fuzz tier's is
bounded by its own budget: ten seconds finds what ten seconds of mutation can find, and a clean run is
not a claim of coverage beyond it.

### Where a backend test goes

Every backend test is a Go test, and the whole tier is one `go test ./...` inside `just check-go`,
with the `internal/` packages run again under the race detector
([`CI.md § Backend build, vet and tests`](CI.md#backend-build-vet-and-tests)). Nothing is held back
behind a build tag or a short-mode skip, so a backend test runs by existing — which is what makes its
*location* the thing an author has to get right. The one platform constraint is the footprint tier's
sampling, which reads open descriptors from `/proc` and live heap from the Go runtime's own memory
statistics, and is confined to Linux by its filename; the predicates it judges with are not.

- **Package mechanics** — beside the package, `backend/internal/<package>/*_test.go`. What one
  package's own logic does, decided without a server: the cache against its clock, the rate limiter
  against its bucket, the secret type against its own formatting paths.
- **Route and integration behaviour** — `backend/internal/router/`. Anything answered in terms of a
  request and a response: status, cause, which module a failure names, what reached upstream and what
  did not. This is the altitude the pipeline's bounds compose at, so a test about cache *and* limit
  *and* the outbound call sits here rather than with any one of them.
- **Process level and soak** — `backend/cmd/`. Anything needing the assembled binary's own wiring —
  the mounted path spaces, liveness, the self-check — and anything measuring the process as a whole
  over time, which is where the footprint tier lives because a footprint is a property of a process
  and not of a package.

The canary tier is deliberately in two of those three places at once, `internal/router/` and `cmd/`,
because the surfaces it sweeps are different: one is what a route returns, the other is what the
assembled process emits. Their shared vocabulary — the planted value itself — is one exported
constant rather than a literal in each, so the two tiers cannot drift into sweeping for different
things while reading as one mechanism.

### Where a frontend test goes

Which runner executes each tier is
[ADR 0027 rev 1](decisions/0027-frontend-test-runners.md)'s; where a test *sits* is here, because
"a new test is wired in by its location alone" is only actionable if the location is stated.

- **Unit** — `frontend/src/**/*.test.ts`, beside the source it exercises. Vitest's configured
  population is that glob, so a file matching it is reached and a file outside it is not.
- **Render** — the framework's own, `frontend/tests/render/*.spec.ts`, one file per obligation, with
  the stub components a fixture places under `frontend/tests/render/stubs/`; and a module's, in that
  module's own directory. Playwright's root is the frontend package and both populations are named to
  it as patterns, so neither is reached by being the runner's default directory.

A module's own two tests sit with the module ([the module contract](contracts/module-contract.md),
part 3; [ADR 0021 rev 4](decisions/0021-repository-layout.md) fixes the directory), which is inside
the unit glob for a shaping library's tests and is why the render runner's population is stated as
two patterns rather than one place — a module's render test is reached by the same runner from a
different place.

The Boundary row enumerates the value classes deliberately: "payload shape and parameter name" reads
narrower than SRS015<!-- One schema, all boundary value classes -->, and an error body or a status
code left out of the schema is exactly the value that crosses unproven.

### Where the Contract tier runs, and how it reaches upstream

**Decided 2026-07-28 by the owner: shapes 1 and 3, composed.** Recorded here because this is the
tier designer's decision and this document is where tier strategy lives. What the tier must prove is
unchanged: **an upstream API still returns what the shaping library expects.**

- **Recorded fixtures, replayed in CI, on every commit.** Each upstream's response is captured and
  the shaping library asserted against it. No credential anywhere, and it gates every change like
  every other tier. Its limit is inherent: a fixture is a snapshot, so it detects drift only when
  someone re-records.
- **A scheduled live run, off the pull-request gate.** Credentialed, against the real APIs. This is
  what catches the drift fixtures cannot — and because it is off the merge path, an upstream having
  a bad afternoon fails a nightly rather than somebody's change.

They compose deliberately, and neither alone is sufficient: fixtures decide merges, and the
scheduled run decides whether the fixtures still describe reality.

**Rejected: an encrypted CI secret scoped to one workflow, running live on every pull request.** It
costs quota against a rate limit nobody watches, and it inherits upstream availability on the merge
path, which trains an author to ignore red. Few upstream sources need a credential at all — which
ones is the [module roster](../README.md)'s and each module's registration entry — so this would have
been a general solution to a narrow case. Confining the credentialed job to the schedule, off the merge
path, is the narrower answer to the nested module
[ADR 0010 rev 3](decisions/0010-runtime-materialised-gate-fixtures.md) found leaky.

**Decided 2026-07-28 by the owner: no requirement.** Nothing WiseKiosk does can violate "an upstream
still returns what we expect" — that obligation is on somebody else's API. The half that *is* ours,
what the product does when an upstream returns what the shaping library did not expect, is already
**SRS001<!-- A failed module shows why, and only that module -->**, whose
`TST001`<!-- Pending: upstream-failure error payload test --> names malformed payload as a failure
class, so a tree item here would restate it. What is left is machinery — recording a fixture is a
procedure an author follows, a scheduled credentialed job is a repository-facing check — so both sit
in [`CI.md § Upstream contract checks`](CI.md#upstream-contract-checks)
([ADR 0011 rev 2](decisions/0011-requirement-or-convention.md)), and the fuzz tier's own budget and
corpus convention are the same kind of machinery, routed the same way to
[`CI.md § Backend fuzz`](CI.md#backend-fuzz).

---

## The Boundary tier is generated, not hand-written

The backend is Go and the frontend is TypeScript, so they share no types
([ADR 0001 rev 1](decisions/0001-backend-language-go.md)). The tier is therefore not a pair of
hand-maintained type declarations checked for agreement by a test — it is **one schema, with both
sides generated from it at build, when missing, and never committed**
([ADR 0008 rev 6](decisions/0008-boundary-contract-openapi-codegen.md)), and the tier's job in CI is
to prove the generation is real and that what is compiled is what was generated:

- Generation from the one schema by the codegen mechanism, under each side's own toolchain, with
  version-pinned generators so regeneration is deterministic. Because neither generated file is ever
  committed, a fresh checkout always regenerates both before anything downstream reads them — there
  is no committed copy for a schema edit or a generator bump to drift from. What is generated is the
  whole wire contract on each side — types, the route table and a client — so a route that moves in
  the schema moves identically on both sides, rather than a hand-written path on each side needing a
  check of its own. Generation is **SRS015<!-- One schema, all boundary value classes -->**'s, and the
  version pin is no requirement's — a repository convention, in [`CI.md § Publishing and
  provenance`](CI.md#publishing-and-provenance).
- That the generated types are the ones actually *used* on both sides, including the per-module
  error-render path, rather than shadowed by a hand-declared twin —
  **SRS016<!-- Both sides consume the generated types -->**, under
  **SYS005<!-- Single-definition internal contract -->**. Because the generated files are never
  committed, every build's first compile of either side — `go build` and the whole-project
  `tsc --noEmit` alike — is against output regenerated in that same run: a hand-declared twin
  compiles today and fails the moment the schema moves it, and there is no window where a committed,
  possibly-stale copy could stand in for it instead.
- That the frontend adds no second, runtime validator over proxied payloads, so agreement rests on
  generation and compilation rather than a bundled re-check —
  [ADR 0008 rev 6](decisions/0008-boundary-contract-openapi-codegen.md), which carries the decision
  and its premise. No requirement states this: it was deleted as a prohibition against a case that
  does not exist.

A value that must agree across the boundary but can be *neither* generated from the schema *nor*
proven to agree by a test is **a finding about the architecture**, not something to paper over with
a comment.

---

## Standing obligations

Gate on these — they name what must be proven. Not on a coverage number. Each is stated in the short
form a test author needs and cited to whatever governs it: the [requirements
tree](requirements/README.md) where the obligation is on the running software, the module contract
or [`DEPLOYMENT.md`](DEPLOYMENT.md) where it is not
([ADR 0011 rev 2](decisions/0011-requirement-or-convention.md)).

- **Every value crossing the frontend/backend boundary is generated from one definition** →
  SYS005<!-- Single-definition internal contract --> /
  SRS015<!-- One schema, all boundary value classes --> /
  SRS016<!-- Both sides consume the generated types -->, and
  [above](#the-boundary-tier-is-generated-not-hand-written).
- **A figure a requirement states is asserted by a check some `TST` item names.** Where an item's own
  text carries a number — an interval, a bound, a count, a horizon — the trace must reach a check
  that reads *that* number, through a `references` entry. This does not make uncited tests
  illegitimate: a test needs no item to earn its place, and most behaviour is covered by tests no
  item names. It is the narrower claim that a figure the specification argues cannot be left to
  coverage nothing points at, because then the tree asserts a number whose only evidence is a test
  the tree cannot see, and deleting that test breaks nothing anybody is told about. **The failure to
  look for is a check that reads the module's own constant instead of the figure** — that asserts the
  module agrees with itself and passes at any value.
- **Every module supplies a render test for its component, and — where it registers against an
  external source — unit tests for its shaping library.** A module with no registration entry is a
  local module and is not expected to have one. A module missing either is an incomplete module, not
  a passing one. What they must cover is stated here; that the files exist and sit where the runner
  reaches them is gated by [`CI.md § Module and framework
  structure`](CI.md#module-and-framework-structure).
- **The configuration schema rejects a realistic malformed input, in a test** →
  SRS002<!-- A module-scoped configuration error is reported at that module -->. The operator is not
  the author, so validation failing correctly and legibly is a product feature and is tested as one.
- **Repo-wide checks live at repo level** — see [Where a check belongs](#where-a-check-belongs).
- **A verification item's fit against its parent is re-read when the item is activated.** Every
  `TST` item is `active: false` until the code it checks exists, and Doorstop skips inactive items
  entirely, so the suspect-link mechanism that normally flags a check whose parent moved beneath it
  is inert across the whole tier. check-reqs's `check_suspect_links` stage restores that one signal
  ([`requirements/README.md § Running the gate`](requirements/README.md#running-the-gate)): a parent
  rewritten under a pending check fails a gate in the commit that rewrites it. What it cannot decide
  is the question activation exists to ask, and that stays a human read — does this check still
  assert a clause its parent still states?

  **Two things are read, not one.** The clause: name the sentence in the parent that this check
  asserts, and if none can be named, the check belongs under a different parent or asserts something
  nobody required. And the item's own text: a stub's text was written before the code and describes
  the test somebody planned, so at activation it is read against the test that exists and rewritten
  where the two differ. An item activated with its stub wording intact describes a test nobody wrote,
  and it reads in the trace exactly like verification — which is worse than an absent item, because
  an absent item is visible to the gate and this is not.
- **A test's declaration is its trace, and the item owns that trace.** The `TST` item that a test
  discharges names *it* — one `references` entry per verifying site, keyed on the line that declares
  the test ([ADR 0005 rev 4](decisions/0005-traceability-gating.md)). **The citation is the trace and
  nothing reads a test's name**, so reading a test's obligation means reading the item that cites it.

  **A module's cited tests carry the citing item's id in the name they declare as well** —
  `TestTST0NN_…`, `test('TST0NN: …')` — so a run naming a failure names the obligation with it. That
  is a reading aid local to a module's own test files rather than a rule of the tree, and it obliges
  nothing: a cited test in a module's own files carries the id, a cited test in a framework file does
  not, and the framework one carrying none is correctly named.
  What such an entry guarantees, and what editing a referenced test costs, is a property of the
  specification rather than of the suite, so it is stated where the specification is:
  [`requirements/README.md`](requirements/README.md) § The V&V model.
- **Every test and check in the repository is executed by CI** — the whole-tree discovery and
  verify/CI wiring gates are [`CI.md § Gate wiring`](CI.md#gate-wiring)'s, not the tree's. A test no
  runner reaches is a false signal, so a new test is wired in by its location alone.

---

## On coverage

Coverage is **diagnostic, never evidence.** A line can be fully covered while the invariant that
matters — that two things agree, that a control functions where deployed — is untested by
construction, so a high number buys confidence it has not earned. Gate on the standing obligations
above; read coverage to find what they missed.

**`just check-coverage` gates a chosen coverage bar over first-party product source**
(gate 3, [ADR 0005 rev 4](decisions/0005-traceability-gating.md)): per-file and total, tree-blind —
it proves source is exercised, not that a `TST` item claims it. Each language's own config holds the
number — `backend/.testcoverage.yml`, `frontend/coverage-thresholds.json` — never this record. One
tool for the backend, an authored script over two tools for the frontend:

- **Backend** — `go-test-coverage` against a `-coverpkg=./...` profile, the **statement**-coverage
  bar `backend/.testcoverage.yml` configures, per-file and total. Go's own toolchain implements no
  other coverage measure — branch coverage is unimplemented
  ([golang/go#28888](https://github.com/golang/go/issues/28888)), and "function" coverage is a
  per-function statement percentage rather than a distinct measure — so the backend is gated on
  statement coverage, stated rather than approximated with a number the toolchain cannot produce.
- **Frontend** — the unit tier (`@vitest/coverage-istanbul`) and the render tier
  (`vite-plugin-istanbul`, torn down into a `coverage-final.json` by
  `tests/render/coverage-teardown.ts`) each instrument the same `.ts`/`.svelte` sources and write
  their own Istanbul coverage; neither Vitest nor Playwright carries a threshold of its own.
  `frontend/scripts/merge-coverage.ts` unions the two via `istanbul-lib-coverage` and enforces the
  one bar value in `frontend/coverage-thresholds.json`, per file, across all four metrics —
  line, branch, function and statement. Its **completeness guard** fails the run if a file either
  tier is meant to instrument carries no coverage at all — the population is read from each tier's
  own instrumentation glob, so a file either tier stops importing is caught rather than
  silently absent from the report; a `.svelte` file, render-tested rather than Vitest-tested, is
  covered by the render tier's own union member instead of reading a permanent, false 0%.

`check-coverage` is folded into `just verify`, and runs each command in sequence, stopping at the
first one under its bar — a run where the backend fails does not reach the frontend commands. After
both gates, `gcov2lcov` converts the backend's Go coverprofile to lcov and `grcov` merges it with the
frontend's own `merge-coverage.ts`-emitted lcov into one further, diagnostic-only HTML report over
both languages together; neither gate's threshold or exit status is affected by it.

---

## Where a check belongs

Put a check at the altitude it is true at:

- **Repo-wide** (spans both packages, or is about the repo as a whole) → the repo-level verification
  target, `just verify`, mirrored in CI. Not a test target: most of what belongs here — link and
  line-ending hygiene, tree integrity, lint, build — is a check, not a test.
- **Package mechanics** (how *this* package's own logic behaves) → that package's own tests.
- **A module's shaping/rendering** → with that module.

A check placed by convenience — "here, because a test runner already existed" — instead of by altitude
is a defect in the suite's architecture, not a neutral choice.

---

## Review cadence

The test architecture is reviewed **whenever a module is added** and **whenever the boundary
transport** (the OpenAPI schema / codegen mechanism,
[ADR 0008 rev 6](decisions/0008-boundary-contract-openapi-codegen.md)) **changes**. This is
scheduled deliberately: removing or reshaping a test feels like a regression even when the test
proves nothing, so without a scheduled review the suite silently becomes permanent architecture
nobody revisits. Code gets that review by default; tests must be given it explicitly.