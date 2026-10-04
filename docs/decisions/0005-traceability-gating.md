# 0005 — Trace all work to the requirements tree with in-repo gates

**Status:** accepted; stored-attribute set superseded by
[ADR 0009 rev 2](0009-verification-justification-attribute.md); traceability scope narrowed and gate 4 retired
by [ADR 0011 rev 2](0011-requirement-or-convention.md)
**Decided:** 2026-09-07 (rev 3's coverage-bar reframe, in the #190 coverage closure gate planning
session; rev 2's evidence-channel pivot taken 2026-08-23 in the #25 traceability gates planning
session; the surrounding model was taken 2026-07-22 at the traceability-gating design discussion,
under the requirements rewrite #18); rev 4's binding reframe and 100% ratchet target taken
2026-09-08 in the #304 coverage gate glue planning session
**Rev:** 4

## Revisions

- **rev 4** — 2026-09-09 — #304 coverage gate glue folds `check-coverage` into `just verify`: gate 3
  runs under the same mandatory sequence as every other check, though the CI job's promotion to a
  required status check in the branch-protection ruleset stays a separate, later action. Removes the
  deferred exemption register: the anti-gaming mechanism is the completeness guard and the rule
  against deleting a defensive branch to win the bar, not a catalogue of exceptions. Records the
  ratcheted destination — 100% coverage, backend statement and frontend four-metric, each language's
  toolchain ceiling — reached later, one bump per PR, not by this rev. Corrects the gate table: gate
  3's frontend half is `scripts/merge-coverage.ts`, authored, not Vitest's own `coverage.thresholds`
  — that line was never true of the shipped tree (owner, 2026-09-08).
- **rev 3** — 2026-09-07 — reframes gate 3 from traceability closure to a **unit-test coverage bar**
  over first-party product source, tree-blind: 90% statement coverage per-file and total for the
  backend (go-test-coverage) and 90% line/branch/function/statement per-file for the frontend
  (Vitest's `coverage.thresholds`, `@vitest/coverage-v8`). The source→test leg is coverage's; the
  test→`TST`→`SRS`→`SYS` legs stay gate 1's, plus the #192 and #191 reports — the reframe does not
  reopen the gap gate 2 was dropped over. Non-blocking first — its own `check-coverage` recipe and CI
  job, outside `verify` and the required-checks ruleset, with no exemption register while non-blocking
  (owner: nothing to exempt while non-blocking; if we do, that's a problem at that time) — and
  #300 coverage gate blocking is what promotes it
  and designs the exemption mechanism (owner, 2026-09-07). Built from two maintained tools
  ([ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md)), no custom check script (#190
  coverage closure gate).
- **rev 2** — 2026-08-23 — walks back the bespoke per-test attribution channel. Forward evidence for
  Test-method items is native Doorstop `references` carrying a `keyword`, which reaffirms
  [ADR 0002 rev 3](0002-requirements-management-doorstop.md)'s mechanism rather than superseding it;
  content drift is caught by Doorstop's own `item_sha_required` plus its `item_validator` hook; gate
  2 is dropped, because a declaration-anchored keyword closes the gap it was to cover and a blocking
  reverse gate would price structural tests out of the suite; and gate 3 moves to #190 coverage
  closure gate (#25 traceability gates).
- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

A branch passed every gate while nothing mechanically tied it to a requirement. `just check-reqs`
proves the Doorstop tree is internally sound — parent links resolve, no suspect or orphan items —
but nothing checks the reverse arrow: that work entering the repo points back into the tree. The
project ethos is that no work exists without a requirement authorizing it, and an ethos nothing
enforces decays silently. Two constraints shaped the answer: all trace information must live in
code, where automation can check it and the docs site can render it (never in out-of-band metadata);
and the project's founding prose is dissolving into hard requirements under
#18, so the model must stand without a prose document as its axiom.

**What rev 2 reopened.** Rev 1 displaced Doorstop's `references` channel for Test-method items with a
per-test in-code attribution convention and a scanner to read it. The scanner was never built, the
tree went on using the native channel rev 1 said it had replaced, and read against the pinned
Doorstop 3.2 source two of the three objections that displaced it do not hold. The evidence channel
below is stated against what the tool actually offers, so that what is bespoke here is only what
Doorstop cannot do.

**And what rev 2 answers differently.** The reverse arrow the Context opens with is two claims, not
one: that every item's evidence exists in the repo and is current, and that every test in the repo
names an item. Rev 2 holds the first mechanically and declines the second, for the reason given
against the third objection below. The record's answer to its own opening question is therefore
narrower than rev 1's, and deliberately so.

## Decision

Every requirement carries a `verification-method` attribute (`test` | `inspection` | `analysis` |
`demonstration`); gates route on the method. `rationale` is required at the `SYS` tier, where there
is no parent to inherit from; lower items may carry one but are not gated on it.

**The forward evidence channel is Doorstop's native `references`, one entry per verifying site,
each carrying the `keyword` of the test that discharges it** (owner, 2026-08-23). The keyword is the
test's **declaration as the source writes it** — `func TestXxx` for Go, `test('<title>'` or
`` test(`<title>` `` for Playwright — so Doorstop resolves the entry to a file *and a line*, the
line being the one that declares the test. Anchoring on the declaration rather than on the bare name
is what makes a deletion visible: Doorstop returns the first line matching the keyword anywhere in
the file, so a bare name also matches the doc comment this repo writes above every Go test, and a
deleted test stays resolvable through its own obituary.

**Anchoring does not reach a parameterized test's cases.** Where the `test(` call sits inside a loop
over a data array, as three render-tier specs do, the keyword is the template as written, which is
none of the titles the runner prints; deleting a *row* of the array retires runtime tests while the
`test(` line, and so the entry, still resolves, and emptying the array leaves an entry resolving
over zero running tests. What catches that is the whole-file drift sha below, and the human
re-review it forces — not an unresolvable reference. Stated rather than designed around, on the
same terms as the drift hook's whole-file granularity.

**A reference to a whole-file check artifact that no runner discovers may omit `keyword`.** Where
the verifying site is a script invoked as one command, every assertion of which discharges the one
item — the `scripts/image/*.py` harnesses — there is no declaration to anchor on and no runner title
to name; drift is still carried by the entry's file `sha`. Every reference to a runner-discovered
test carries a declaration-anchored keyword.

This reaffirms
[ADR 0002 rev 4](0002-requirements-management-doorstop.md): a `TST` item's `references` point at the
real verifying files, for every verification method, and rev 1's partial supersession of that
mechanism is withdrawn.

Rev 1 rejected the native channel on three objections. Their disposition, each settled against the
pinned tool rather than against recollection:

- **Granularity** — *a file holds many tests, so a file-grain reference smuggles untraced siblings
  past the gate*. **Does not hold.** A `references` entry takes a `keyword`, and Doorstop resolves it
  to a file and line number. The channel is line-grain; rev 1 was written as though `keyword` did not
  exist. Anchored on the declaration, the entry vouches for one test rather than for the file holding
  it, so a sibling is neither smuggled nor claimed.
- **Multiplicity** — *one reference cannot carry the relation the tree needs*. **Does not hold.**
  `references` is a validated many-to-many list: an item may name several sites, several items may
  name one site, and every entry is checked rather than only the first. That is exactly the relation
  [`../requirements/README.md`](../requirements/README.md) states, where a `TST` item is a
  verification obligation rather than a test function.
- **Reverse direction** — *nothing proves that no sibling test is untraced*. **Survives as a fact
  about the tool, and is rejected as a gate** (owner, 2026-08-23). Doorstop's traceability is
  item-anchored: it asks whether this item's reference resolves, never whether every test in a file
  is claimed by some item. What that observation was carrying was the granularity objection above,
  and the declaration-anchored keyword settles that natively. What is left is the demand that every
  test the runner discovers name a `TST`, and that is refused: a suite holds requirements-based tests
  *and* structural ones — a table-driven boundary case, a regression pinning a fixed bug, a fuzz
  seed — and obliging each to carry a verification obligation either invents items that state no
  want or prices the test out of the suite. Degrading the test protocol to satisfy a gate is the
  wrong trade. What is left after that refusal is a **deliberately accepted, non-gated gap**: no
  gate and no report measures the tests no item claims, and #192 verification-debt report is not
  that report — it runs the other way, over accepted Test-method items with no resolving reference.
  The two populations are complements, and #192 says so in its own body.

**Drift on the evidence channel is caught natively too.** The `TST` document sets
`item_sha_required` in its `.doorstop.yml`, so `doorstop review` records a SHA256 of each referenced
file in its entry, and the document's `item_validator` hook — Doorstop's documented per-document
extension, described in its scripting guide and dogfooded on upstream's own tree under `reqs/ext/`,
though not carried in the installed package — compares the recorded hash against the file on every
validation run. The copy this repo runs, `tst/.req_sha_item_validator.py`, is authored here. A
referenced test whose content changed fails `doorstop --error-all` until the item is re-reviewed.
This is the suspect-link discipline the tree already applies to parent requirements, extended to the
files that verify them, and it runs inside the existing `check-reqs` rather than as another sibling
script. Granularity is whole-file: any edit to a referenced test re-reviews every item
referencing it, and that churn is accepted rather than designed around; hashing only the keyword's
region is a possible refinement if it bites. The extension is per-document, so `sys/` and `srs/` are
untouched.

Two gates, both in-repo, both mirrored byte-identically in CI, and both run inside `just verify`.
The numbering is historical, so a retired gate's number is not reused:

| # | Gate | Proves |
|---|---|---|
| 1 | `check-reqs` (exists) | Tree integrity: parent links, no suspect/unreviewed/orphan items; every `TST` reference resolves to a file, and to the declaration its `keyword` names where it carries one; no referenced test drifted since review |
| 3 | `check-coverage` (exists) | First-party product source meets its language's coverage bar, per-file and total for the backend, per-file for the frontend — each threshold read from that language's own config (`backend/.testcoverage.yml`, `frontend/coverage-thresholds.json`), never this record |

Gate 2, a reverse-direction claim over every discovered test, is dropped for the reason given against
the third objection above. Gate 4, an inspection file-claim over non-code silos, is retired by
[ADR 0011 rev 2](0011-requirement-or-convention.md) for having no possible subject.

- **Gate 1 runs forward, from the tree outward**, proving each item's evidence exists, at the line
  that declares it, and has not drifted. **Gate 3 is tree-blind**: a coverage bar over first-party
  product source, source → test only. It proves nothing about test → `TST` → `SRS` → `SYS` — that leg
  stays gate 1's, plus #192 verification-debt report and #191 verification matrix, the forward and
  reverse views over accepted items' evidence. So the reframe does not reopen the gap gate 2 was
  dropped over: a structural test with no `TST` naming it still counts toward coverage, exactly as it
  counted before gate 2 was dropped — arguably more visibly, since an uncovered line reads as coverage
  debt whether or not any item ever claimed the test that would have covered it.
- **Analysis and demonstration items close through referenced artifacts** — the item references the
  analysis document or demonstration procedure, and derived verification is that reference resolving
  plus the item's `reviewed` fingerprint. Human judgment stays in the sign-off; only the linkage is
  mechanized. The channel is the same one Test-method items use; what differs is what sits at the
  other end of it.
- **Stored state records human decisions only**: `status: proposed | accepted`, where acceptance is
  the review act (fingerprint, rationale, method present). Verified/implemented is **derived** from
  evidence and is never stored. Retirement is Doorstop's existing `active: false`.
- **The tree is the backlog.** `proposed` items live on `main`; accepted-but-unverified items are
  reported as the work queue, never a blocking failure — a scoping-only session stays green.
  Implementing against a `proposed` item fails: that is building on unbaselined spec.
- **The axiom tier is the parentless `SYS` items**, fenced by Doorstop's `reviewed` fingerprints and
  suspect-link propagation — editing a `SYS` item fails `check-reqs` until every child is
  re-reviewed. No prose document sits above the tree. ADR links are optional provenance where a
  decision genuinely drove a requirement; a mandatory source field at the axiom tier would force
  boilerplate over the honest "because the owner decided".

Trace direction depends on the work: implementation traces down to accepted items through the
`references` its `TST` items carry; requirements-authoring self-traces up via parent links in its own
diff; `SYS`-tier work traces to nothing by design, gated by review.

The four stored attributes — `verification-method`, `status`, `verification-justification` and
`rationale` — are untouched by this rev and are not reopened by it.

## Alternatives considered

- **A bespoke per-test in-code attribution convention, read by an authored scanner** — rev 1's
  choice, in which a test names the `TST` item it verifies in its own source and `TST` `references`
  are reserved for inspection, analysis and demonstration artifacts. Rejected: it re-implements what
  the tool already does, and does it worse. Doorstop validates that a referenced path is tracked and
  that its keyword is present, carries the reference inside the item's `reviewed` fingerprint, and
  renders it in published output; a parallel convention gets none of that, and a renamed or deleted
  test file breaks nothing. Two of the three objections that justified the replacement do not hold
  against the tool as shipped, and the third asks for a gate this rev declines to build.
- **Requirement IDs in PR bodies, and a PR→issue link gate.** Rejected by a partition argument:
  every file a PR touches falls to exactly one of the gates, so its trace is fully derivable
  from the diff against the tree — a PR-body ID either restates that or contradicts it, and in a
  contradiction the diff is the truth. PR metadata lives outside the checked zone: no scanner reads
  it, no page renders it, nothing fails when it rots. Issues remain as scheduling views over the
  backlog; branch shape is process-gated by [ADR 0006 rev 6](0006-process-gates.md) but stays outside the
  traceability evidence channel — the rejection here stands.
  The partition once obliged gate 4's claim mechanism to reach files Doorstop references cannot —
  paths under dot-directories, a limit 0002 records — but that gate is retired, so the obligation
  lapsed with it.
- **A stored `implemented`/`verified` state.** Rejected: it is derivable from evidence the gates
  already have, and a hand-set flag survives the deletion of the test that justified it — the same
  hand-declared-in-two-places defect class the boundary-contract rule exists to kill.
- **Requirement→source design-allocation refs.** Rejected: refs into a volatile source tree churn on
  every rename, and design allocation, where wanted, belongs to the architecture model, not the gate
  system. This is about references into the *source* tree and is untouched by rev 2, which concerns
  the test evidence channel alone, and by rev 3's coverage bar, which is tree-blind rather than a
  substitute for such a ref.
- **`item_sha_required` without the validator hook.** Rejected: Doorstop writes the hash at review
  time and no core command ever re-derives it, so the recorded shas would sit in every item's YAML,
  ride its fingerprint, and be compared by nothing. The extension supplies the data for a drift
  check, not the check; adopting the writer without the reader buys noise.
- **A "needs re-verification" state.** Rejected: Doorstop's suspect-link machinery already fails
  `check-reqs` when a parent changes until children are re-reviewed; a parallel state would
  duplicate an existing enforcement. With the drift hook above, the same machinery also covers a
  changed test file.

## Consequences

- **Verification debt is visible, not blocking.** The docs site renders a verification matrix from
  derived status (sphinx-needs `status` + `needtable`); unverified accepted items are backlog rows,
  not red CI. If a release gate is ever wanted, it blocks there, not at merge. Building that matrix
  is #191 verification matrix.
- **Editing a verified test costs a re-review.** The drift hook hashes whole files, so a comment, an
  import reorder or a lint fix in a referenced spec unreviews every item referencing it, and
  `doorstop review <UID>` is the only way to clear it. That is deliberate pressure in the direction
  [ADR 0002 rev 4](0002-requirements-management-doorstop.md) warns about — re-blessing is a human act
  and must not become a reflex — and it is the cost this rev accepts in exchange for the tree
  noticing when its evidence moves.
- **Gate 1 is the tool's, configured; gate 3 is mixed.** The drift hook is the extension point
  Doorstop documents, not a sibling script, so gate 1 stays configured rather than authored. Gate 3's
  backend half is `go-test-coverage`, likewise configured; its frontend half is
  `scripts/merge-coverage.ts` — the per-file four-metric union and the completeness guard below are
  this repository's own rule, so [ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md)'s
  authored/configured line runs through gate 3 rather than around it. Vitest itself carries no
  coverage thresholds (`vitest.config.ts` states none); the one number gate 3 enforces is
  externalised to `frontend/coverage-thresholds.json`, a config an authored script reads, not a
  configured tool's own setting. Dropping gate 2 leaves gate 3 as the table's one partly authored
  entry. The configuration that arms the drift hook is fragile in a way no gate here covers: keeping
  the `extensions:` block present is an obligation on whoever edits a `.doorstop.yml`, and it leaves
  no artifact, so a reader answers it in [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md)'s checklist
  ([ADR 0011 rev 2](0011-requirement-or-convention.md)).
- **An unclaimed test is not a build failure.** Nothing fails when a test the runner discovers is
  named by no item, so a structural test lands without inventing a `TST` for it. What that costs is
  visibility into that population, and nothing here buys it back — the gap is accepted rather than
  mitigated, and saying so is the honest state. #192 verification-debt report is a *separate*
  forward-direction report over items with no evidence, not a substitute for it.
- **Coverage is a chosen bar, per-file and, for the backend, total** — statement for the backend,
  where the toolchain implements no other measure (golang/go#28888), line/branch/function/statement
  per-file for the frontend. The number itself is never this record's: it lives in
  `backend/.testcoverage.yml` and `frontend/coverage-thresholds.json`, so a change to it is a config
  diff, not an ADR rev. Coverage proves execution, not specification: a line covered incidentally
  counts. That residue is held by review and test quality, and no gate pretends otherwise. **100%,
  each language's toolchain ceiling, is the ratcheted destination** — reached by
  raising those two config files one bump per PR; this rev records the direction and states no
  number of its own, so it stays true through every later bump. **Meeting a raised bar restructures
  the code or forces the error path under test — it never deletes a defensive branch to win the
  number** (owner, 2026-09-08): a bar met by deleting the thing it measures gates nothing.
- **The ungated surfaces are named rather than counted.** The axiom tier, held by human review and
  mechanized by fingerprints. The reverse direction, accepted above. And source below the coverage
  bar, which fails `just verify` and the CI `coverage` job exactly as every other check does —
  though the branch-protection ruleset naming that job a required status check, so a merge
  bypassing `verify` locally would also be blocked at the platform level, stays a separate, later,
  owner-driven action. What the gates do reach, they reach mechanically; nothing else is claimed.
- **No exemption register exists, and none is planned.** The anti-gaming mechanism is three things:
  the completeness guard (`merge-coverage.ts` fails when a file either coverage tier is meant to
  instrument carries no coverage at all, so a deleted test's absence is visible rather than read as
  100% of nothing), the restructure-or-force-the-error rule above, and **no coverage-ignore
  annotation of any kind on first-party source** (owner, 2026-09-08) — an instrumented line the
  tooling cannot reach is restructured or removed, never marked past the gate. A register of
  exceptions is a list of gates the tree has agreed to fail; this rev's answer is to hold the bar
  rather than catalogue exceptions to it.
