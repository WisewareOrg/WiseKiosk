# 0016 — Maintained tools for public conventions; authored checks for this repository's own rules

**Status:** accepted
**Decided:** 2026-08-03 (#103 authored-vs-adopted check trade, measured against the cases recorded in
[`../../scripts/README.md`](../../scripts/README.md))
**Rev:** 12

## Revisions

- **rev 12** — 2026-10-04 — the `golangci-lint` adoption row gains `depguard`, confining non-test
  backend code, generated code included, to the standard library and this module's own packages —
  [ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md)'s zero-runtime-dependency constraint's
  first-enforcement case on the Go side; cites [ADR 0001 rev 1](0001-backend-language-go.md) too.
  The adoptions are otherwise unchanged (#425 renovate go module resolution).
- **rev 11** — 2026-09-06 — extends the decision to a further net-new adoption, `Trivy`, scanning the
  built container image and retiring [`../CI.md`](../CI.md) § *Image vulnerabilities*'s "Unbuilt;
  owned by #67" line (#265 image vulnerability scan).
- **rev 10** — 2026-09-06 — extends the decision to three more net-new adoptions, `cosign`, `syft`
  and `check-jsonschema`, retiring [`../CI.md`](../CI.md) § *Publishing and provenance*'s "every
  check below is unbuilt" line; adds two guard-table rows for the two authored scripts landing
  beside them, `scripts/publish/verify_permissions.py` and `scripts/publish/verify_metadata.py`
  (#269 publish verification).
- **rev 9** — 2026-09-05 — extends the decision to two more net-new adoptions, `govulncheck` and
  `npm audit`, retiring [`../CI.md`](../CI.md) § *Dependency vulnerabilities*'s "Unbuilt; owned by
  #67" line (#264 dependency vulnerability gates).
- **rev 8** — 2026-09-05 — extends the decision to a second first-adoption case: `CodeQL`'s default
  code-scanning suite lands as a net-new gate over the project's own Go, Svelte/TypeScript, Python
  and Actions source, retiring [`../CI.md`](../CI.md) § *First-party source scanning*'s "Unbuilt;
  owned by #67" line — the same shape rev 7's net-new rows take, decided by the table's `Replaces`
  column rather than a fresh count. The adoptions are otherwise unchanged (#263 CodeQL gate).
- **rev 7** — 2026-09-04 — extends the decision to a first-adoption case: `golangci-lint` (default
  linter set) and `eslint` (recommended sets, with `svelte-check` alongside) land as net-new gates
  rather than replacing an authored check, retiring [`../CI.md`](../CI.md) § *Lint and type checks*'s
  "Unbuilt; owned by #67" line in place of an authored-check line count — the shape a first adoption
  takes where there is no prior authored check to retire (#262 lint and type check gates). The
  Decision prose's "Four adoptions follow, each replacing its authored check outright" and the
  Consequences section's "Four maintained tools enter CI" both counted only the original four; both
  are reworded to distinguish a replacing row from a net-new one by the table's own `Replaces` column
  rather than a count the next adoption rots.
- **rev 6** — 2026-09-03 — Dependabot is replaced by self-hosted Renovate (run from
  WisewareOrg/wise-renovate) as the dependency updater; the version-comment ground names it; what was
  chosen is unchanged, so the Decided date does not move (#223 renovate cutover).
- **rev 5** — 2026-08-16 — extends the empty-population ruling to every check: an empty population
  reports success, authored checks included, collapsing this wave's three readings of the ruling to
  one. The boundary that stays: a failed or unreadable population enumeration is not an empty
  population and must fail — the materialised-list fix in PR #162 adopt lychee and the untracked-file
  guards of PR #159 fail-ls-files-gates-closed are that boundary, not exceptions to the ruling. The
  adoptions are unchanged (owner ruling recorded on PR #161, 2026-08-16).
- **rev 4** — 2026-08-16 — corrects the zizmor measurements, which were bound to a pre-v1.10.0
  zizmor without recording a version: on the adopted release (v1.29.0) the `pedantic` persona also
  reports `anonymous-definition`, `undocumented-permissions` and `concurrency-limits` against the
  live workflows — audits introduced across v1.10.0–v1.16.0 — plus `dependabot-cooldown` against
  `.github/dependabot.yml` when the input is the whole tree, and `artipacked` reports at Low rather
  than Medium since v1.17.0; a seeded fixture confirms zizmor does report a workflow declaring no
  `permissions:` block, answering the question left open below — the re-run of the retired check's
  recorded cases against the adopted pair, that fixture among them, is recorded in
  [`../../scripts/cases/workflow-audit.md`](../../scripts/cases/workflow-audit.md); the adoptions are
  unchanged (#105 adopt zizmor and actionlint).
- **rev 3** — 2026-08-06 — states where ADR 0006 rev 4 carries the gate-path correction, in place of a
  claim that the correction would land in this record's own change; the adoptions are unchanged
  (#126 absorb amendment blocks).
- **rev 2** — 2026-08-05 — drops a ground that rested on merged ADR text being unrewritable; the tool adoptions are unchanged (#118 ADR revisions).
- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

This repository is guarded by the `check-*` recipes [`just verify`](../../justfile) runs, plus two
that exist only in CI: the pull-request-title check and the `secret-scan` job. There were fourteen
when this was decided; the roster is `just --list`, and no count written here is compared against it. Nearly
every obligation they assert was authored here, and that was never a decision. The gate records argue
at length about *which hand-rolled form* a check should take — [ADR 0005 rev 4](0005-traceability-gating.md) on
where evidence lives, [ADR 0006 rev 6](0006-process-gates.md) on what the gate reads,
[ADR 0014 rev 4](0014-documentation-index-claims-documents.md) on whether a list may be hand-maintained — and
never about whether to author one at all.

The exceptions show the question has an answer whenever it is actually asked, and there are four of
them. `check-site` is a Sphinx build with warnings-as-errors, adopted by
[ADR 0004 rev 2](0004-docs-site-sphinx-needs.md). `check-arch` validates the model with `likec4 validate`,
adopted by [ADR 0003 rev 4](0003-architecture-as-code-likec4.md). `check-reqs` delegates tree integrity to
`doorstop --error-all`, adopted by [ADR 0002 rev 4](0002-requirements-management-doorstop.md) — a delegation
[`../CI.md`](../CI.md) relies on explicitly when it scopes the dead-test gate away from the
requirements tier. `secret-scan` is `gitleaks`. So the precedent
exists and is four times recorded; it had simply never been generalised, and in its absence every new
obligation defaulted to being authored.

In one place that default had gone visibly wrong: `check-workflow-hardening.mjs` scans workflow YAML
as plain text with no parser to check action pinning and top-level write permissions — a hand-rolled
subset of an audit that maintained tools perform against a parse tree.

## Decision

**A check is authored where the obligation it asserts is this repository's own rule. Where the
obligation is a public convention, it is delegated to a maintained tool.** The discriminator is what
a check *asserts*, not what it reads: `check-workflow-hardening.mjs` and `check-verify-ci-parity.mjs`
both read `.github/workflows/`, and only the first asserts something every project shares.

**"Maintained" means a real maintenance base, not merely something published.** A widely-used project
with many contributors and a release history is a different object from a single-maintainer action
wrapping twenty lines of API calls. Adopting the latter replaces a reviewable in-repo script with
unreviewed third-party code running against repository credentials, which *adds* supply-chain surface
— the opposite of the reasoning that justifies adoption. Where only such a wrapper exists, the check
stays authored.

The adoptions below follow. Most replace an authored check outright; a row with no `Replaces` entry
is a net-new gate, adopted where no authored check ever existed for it to replace:

| Adopted | Replaces | Lines it retires |
|---|---|---|
| `zizmor` at the `pedantic` persona, with `actionlint` alongside | `check-workflow-hardening.mjs` | 177 |
| `lychee` | `check-links.mjs`, `upstream-hosts.txt` | 123 |
| `commitlint` at both the commit-message and pull-request-title stages | `check-commit-msg.sh`, `conventional-commit.regex` | 40 |
| `pre-commit` as the local hook layer | `.githooks/`, `check-eol.sh` | 33 |
| `golangci-lint` at the default linter set, plus `depguard` confining non-test backend code, generated code included, to the standard library and this module's own packages ([ADR 0001 rev 1](0001-backend-language-go.md), [ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md)) | — | [`../CI.md`](../CI.md) § *Lint and type checks*'s "Unbuilt; owned by #67" line (Go half) |
| `eslint` (flat config, recommended sets), with `svelte-check` (`--tsgo`) alongside | — | the same line's frontend half |
| `CodeQL`'s default code-scanning suite, over Go, Svelte/TypeScript, Python and Actions | — | [`../CI.md`](../CI.md) § *First-party source scanning*'s "Unbuilt; owned by #67" line |
| `govulncheck`, pinned by a `tool` directive in `backend/go.mod` | — | [`../CI.md`](../CI.md) § *Dependency vulnerabilities*'s "Unbuilt; owned by #67" line (Go half) |
| `npm audit` | — | the same line's npm half |
| `cosign`, keyless signing, SBOM attestation and verification | — | [`../CI.md`](../CI.md) § *Publishing and provenance*'s "every check below is unbuilt" line |
| `syft`, SPDX SBOM generation, `download-syft` pinned by version | — | the same line |
| `check-jsonschema`, SPDX schema validation, `pipx run` pinned by version | — | the same line |
| `Trivy`, image vulnerability scanning, digest-pinned in `scripts/vulns/check_vulns.py` and refreshed by a Renovate `customManagers` entry | — | [`../CI.md`](../CI.md) § *Image vulnerabilities*'s "Unbuilt; owned by #67" line |

**`commitlint` must preserve the two-stage distinction the authored check carries.** `fixup!` and
`squash!` are permitted at the commit-message stage because the squash discards them, and refused on
the pull-request title because the squash makes that title the commit on `main`
([ADR 0006 rev 6](0006-process-gates.md)). `commitlint`'s `defaultIgnores` *pass* such subjects, so the
pull-request-title invocation runs with them disabled. The pattern keeps the single definition
`conventional-commit.regex` holds today; whether one configuration can serve both stages, or one must
`extends` the other, is left to the implementation.

**Where a maintained tool owns an obligation but covers only part of it, the remainder is authored —
and scoped explicitly to the uncovered part**, so the tool's coverage is never silently re-encoded
beside it. **The gap is demonstrated, not asserted:** the tool is run against the case said to be
uncovered, and the result recorded, because *"it only covers part of it"* is otherwise available to
anyone who would rather keep their own script.

`scripts/check-suspect-links.py` is the worked example, and it meets that standard: suspect links are
Doorstop's concept, `doorstop --error-all` decides them for every *active* item, and the authored script
covers inactive items only. The gap is **structural** — Doorstop cannot see an inactive item at all — so
the uncovered part has a boundary a scope can be written against. That is what separates this from the
residue rejected below: a residue check is a second gate kept beside a tool that already reaches the
case, whereas this is the only thing reaching it.

**The unit this rule sorts is an obligation, not a `just` recipe.** A recipe may run several commands
asserting different things, and three of them carry more than one kind: `check-reqs` delegates tree
integrity to Doorstop while five of its six commands assert rules of this repository's own; `check-arch`
delegates model validation to `likec4` while `splice-arch-diagrams.mjs` asserts this repository's
`arch-export` marker format and a following `git diff --exit-code` asserts that the generated artifacts
are byte-identical to a regeneration; and `check-links` is split by this very decision — its link
resolution delegated, its two other obligations retired. Sorting by recipe would misfile all three, and
a census taken that way was wrong three times in the drafting of this record.

The obligations that stay authored are everything in `check-branch`, `check-citations`,
`check-adr-index`, `check-docs-index`, `check-repo-silo` and `check-verify-ci-parity`; the five
repository-specific commands in `check-reqs`; and, in `check-arch`, both the marker checking in
`splice-arch-diagrams.mjs` and the generated-artifact staleness comparison. `git` and `doorstop` are the
mechanisms those last two use; neither owns the obligation, exactly as `git ls-files` does not own the
documentation-index claim.

**The empty-population guards *in the checks this decision adopts away* are retired as a class.** Those
checks fail when they find nothing to scan, on the principle that a check which read nothing must not
report success — `check-workflow-hardening.mjs` on an empty `.github/workflows`, `check-eol.sh` on a tree
with no tracked file, and the same script when `git grep` fails outright rather than matching nothing.
**Owner ruling, for those checks: an empty scan may report success.** None is carried into the adopted
tools or asserted of them.
Measurement incidentally shows `zizmor` exits 3 with `no inputs collected` on an empty tree, and
`pre-commit` skips a hook whose file set is empty and reports success; neither behaviour is now an
obligation, so neither constrains the implementation.
**Cost:** a misconfigured invocation that silently scans nothing reports the same success as a clean run.

**The ruling reaches every check, and its bound is what kind of nothing was seen** (rev 5). An empty
population reports success, in authored checks as in adopted tools. **A failed or unreadable
population enumeration is not an empty population and must fail** — a materialised file list that
cannot be read, a parse producing nothing from input that exists, and a population silently *shrunk*:
the unterminated-fence case, where a fence that never closes blanks the rest of the file, is that
failure and not an empty scan, and is left open below. The guards in the table assert that an
enumeration worked, which the ruling keeps — not that the population was non-empty, which it
releases. The table below is **examples, not an inventory** — deliberately, because a
hand-maintained list of code in an immutable document is the drift
[ADR 0014 rev 4](0014-documentation-index-claims-documents.md) refuses from the other direction, and no check
compares this one against the tree. Including:

| Guard | Source |
|---|---|
| No ecosystem entry parsed from `.github/dependabot.yml` | `scripts/check-repo-silo.mjs` |
| The justfile dump names no recipe | `scripts/check-repo-silo.mjs` |
| `verify` depends on no check | `scripts/check-verify-ci-parity.mjs` |
| A recipe body parsed to no commands | `scripts/check-verify-ci-parity.mjs` |
| No `arch-export` marker found in `ARCHITECTURE.md` | `scripts/splice-arch-diagrams.mjs` |
| No `verify` job found under `jobs:` | `scripts/publish/verify_permissions.py` |
| The pushed index names no platform child | `scripts/publish/verify_metadata.py`, `sbom_attest.py`, `verify_release.py` (every `--step`) |

The first is the guard that closed #68's defect class — a guard keyed on the same literal as the thing it
guards goes to zero alongside it and then reports agreement, and the form that works asserts the parse
produced something at all. The rest apply that form. Nothing here licenses removing any of them.

**Three further obligations are retired outright rather than reimplemented**, beside the
empty-population guards above. Each is dropped as an
obligation, not demoted to an ungated convention: after this, nothing in the repository requires them
and no document asks for them. What each retirement costs is stated here rather than discovered
later.

- **The `# vN` version comment beside a pinned action.** The gate decides presence, not truth: a
  comment left stale against a hand-edited SHA passes clean, so it never secured the property it
  appears to. Renovate's `helpers:pinGitHubActionDigests` preset writes and updates the comment on the
  bumps it performs, which is nearly all of them, and that continues — as a tool's behaviour, not as
  an obligation on anyone.
  **Cost:** a hand-added pin may carry no version comment, and nothing will say so.
- **The host allowlist.** It carries one entry, `github.com`, so its live effect is to refuse
  citations of the tools this repository runs. Documentation may link outward.
  **Cost:** absolute links are no longer constrained to a reviewed set of hosts.
- **The repository-escape check.** **Cost, stated precisely because the retirement is deliberate:**
  [`../../scripts/README.md`](../../scripts/README.md) records two must-fail rows here, not one — a
  destination climbing above the repository root, and a tracked symlink pointing outside it. The
  symlink variant is not hypothetical; it is a defect this check *had* and was fixed for, because
  `resolve()` and `existsSync()` both follow a symlink without reporting that they did. `lychee`
  resolves through such a symlink, finds a file, and passes. Both rows are given up knowingly.

## Alternatives considered

- **Author everything — the status quo.** Its real argument is that [`../CI.md`](../CI.md) states,
  per gate, *what it is allowed to let through*, and no adopted tool ships that sentence. Rejected
  because the sentence survives adoption — what `zizmor --persona=pedantic` lets through is as
  statable as what a script lets through — while the measurements went the other way on substance.
  Against the live workflows `zizmor` reported **seven `artipacked` findings the authored check
  cannot see** (credential persistence on every checkout), and against seeded defects it caught the
  unpinned reference at High severity and both write-permission spellings.
- **Adopt only where the tool is a strict superset, keeping a residue check for the remainder.** The
  shape this decision was expected to take. Rejected, but not uniformly: the version comment and the
  host allowlist did not survive being named, while the repository-escape check did — its symlink row
  records a real defect, and it is given up as a deliberate cost rather than because the obligation
  was found empty. What decides the alternative is that a second gate carrying one residual obligation
  costs more than any of the three is worth, not that all three were weak.
- **`zizmor` at the default persona.** Rejected: it does not report top-level write grants at all,
  which would preserve most of `check-workflow-hardening.mjs` for a narrower reason than it was
  written for. `pedantic` was measured against the live workflows and produced **zero
  `excessive-permissions` findings** — the job-level `pages: write` and `id-token: write` elevation
  in `pages.yml` is not excessive to it, though the separate `undocumented-permissions` audit
  (v1.13.0) requires an explanatory comment beside every grant other than a bare `contents: read`.
- **Suppressing `pedantic`'s disagreement over `permissions: read-all` rather than adopting it
  plainly.** `zizmor` reports `read-all`; `check-workflow-hardening.mjs` explicitly permits it and its
  own failure message recommends it. That is a genuine least-privilege disagreement, not a tool error.
  Rejected as unnecessary: neither workflow uses `read-all` — both declare explicit `permissions:
  contents: read` — so the case is hypothetical, and a suppression written against a hypothetical is
  where a bypass gets spelled ([`../../CONTRIBUTING.md`](../../CONTRIBUTING.md) review checklist item 11,
  *Narrowed guards*).
- **A third-party action for the ticket-linkage obligations in `check-branch`.** Rejected under the
  maintenance test above. GitHub offers no native rule: the `pull_request` ruleset rule takes
  `allowed_merge_methods`, `dismiss_stale_reviews_on_push`, `require_code_owner_review`,
  `require_last_push_approval`, `required_approving_review_count`,
  `required_review_thread_resolution` and `required_reviewers`, and nothing for a linked issue.
  Branch-name enforcement remains a ruleset metadata restriction, which [ADR 0006 rev 6](0006-process-gates.md)
  already rejected as Enterprise-only. What remains are single-maintainer marketplace actions, which
  the maintenance test excludes.
- **`conventional-pre-commit` for commit messages instead of `commitlint`.** Rejected: CI gates the
  pull-request title rather than the commits, so an authored check would still be needed there and the
  pattern would live in two places — the defect the shared regex files exist to prevent.

## Consequences

- **373 lines of authored check are retired**, along with `upstream-hosts.txt` and
  `conventional-commit.regex`, when the implementation tickets land — the four replacing adoptions'
  own consequence, not the net-new rows', which retire no authored check. Every adopted tool in the
  table above enters CI as its own ticket lands.
- **The corollary is the finding worth carrying:** when a maintained tool covers most of a check, the
  residue usually should not survive either. An authored check accretes obligations nobody would
  choose deliberately, and adopting a tool is what forces each one to be named and defended.
- **Seven `artipacked` findings become actionable** — every `actions/checkout` needs
  `persist-credentials: false`. The authored check never reports them.
- **Practice adapts to the tool, not the reverse.** Two commit titles on `main` exceed `commitlint`'s
  default `header-max-length` of 100, the longest at 122. The default stands and titles get shorter;
  configuring the tool around existing practice would forfeit the reason for adopting it.
- **[ADR 0006 rev 6](0006-process-gates.md) is corrected, not superseded.** The gate path it was
  decided under was *plain sh + curl + jq — no toolchain*; `commitlint` and `pre-commit` reverse that
  property while leaving its four gates standing. That record carries the property under its own
  *Alternatives considered*, as one given up, at a rev of its own.
- **Four surfaces are rewritten when each implementation lands**, not here: [`../CI.md`](../CI.md)
  §§ *Action pins and workflow privilege*, *Documentation integrity* and *Repository shape*;
  [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md)'s hook-installation instruction; the `justfile`
  roster, whose `verify` and `install-hooks` doc strings both name mechanisms this retires; and
  `check-verify-ci-parity`, since [`../CI.md`](../CI.md) requires every CI step to be a `just verify`
  check or a named exception — so each adopted tool needs a recipe or an exception.
- **[`../../scripts/README.md`](../../scripts/README.md) loses the sections for the retired checks**,
  and earns its keep for those remaining: its recorded cases, in both directions, were the conformance
  suite this decision was measured against. Seeding the legal variant is what surfaced `pedantic`'s
  `read-all` behaviour.
- **#60 authored-language set was right to block on this.** Its planning had consigned these same 373
  lines to a Python conversion; every one of them would have been rewritten and then retired.
- Adoption is not performed here. One implementation ticket per adopted tool, each sized to be read in
  one sitting.

**Left open, to be carried by the implementation tickets:**

- Whether `lychee` runs online or `--offline`. Online checks absolute links for reachability and costs
  network flakiness; `--offline` skips them entirely, which with the host allowlist retired means
  absolute links get no check at all.
- How `commitlint` and `pre-commit` are pinned. Neither is an action, so neither is covered by
  [`../CI.md`](../CI.md) § *Action pins*; `zizmor`, `actionlint` and `lychee` have official container
  images and are pinned as their digests.
- Whether any adopted tool's fixtures conflict with [ADR 0010 rev 3](0010-runtime-materialised-gate-fixtures.md),
  which forbids committing a vulnerable artifact in resolvable form.
- Sequencing against #101 CI-invoking-just-recipes, which touches the same gate-wiring surface.
- Whether an under-scan needs a guard, distinct from the empty scan the ruling above settles.
  `check-links.mjs` guards an unterminated fence because it blanks the rest of the file; under CommonMark
  such a fence runs to end of document, so `lychee` would extract no links past it and report clean.
  `check-workflow-hardening.mjs` likewise fails a workflow declaring no `permissions:` block at all;
  a seeded fixture confirms `zizmor` reports it (`excessive-permissions`). The fence case remains one
  seeded fixture away from an answer.

**Premise that would reopen this:** an adopted tool is abandoned, or changes its defaults so that it
no longer covers the obligation it was adopted for; or a repository convention appears that the tool
cannot express and that is worth a gate of its own. **That second bar is the one actually applied here,
and it is higher than surviving scrutiny:** the repository-escape check did survive being named — its
symlink row records a real defect — and was retired anyway, because one residual obligation does not earn
a second gate beside an adopted tool. Absent either, do not relitigate — and note that the first is a
per-tool reopening, not a reason to return to authoring generally.
