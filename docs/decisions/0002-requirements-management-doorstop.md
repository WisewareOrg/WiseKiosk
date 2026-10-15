# 0002 — Requirements tracking and V&V with Doorstop

**Status:** deprecated
**Decided:** 2026-07-21 (requirements-system spike, issue #14)
**Rev:** 4

## Revisions

- **rev 4** — 2026-09-04 — replaces the stale claim that a `pip` Dependabot ecosystem entry is what
  tracks the venv's dependencies with a timeless statement of the update-tracking entry itself; what
  was chosen is unchanged, so the Decided date does not move (#223 renovate cutover).
- **rev 3** — 2026-08-23 — drops the Status note recording that the Test-method linkage mechanism was
  superseded: [ADR 0005 rev 2](0005-traceability-gating.md) withdraws that supersession and
  reaffirms the mechanism this record decides — a `TST` item's `references` point at the real
  verifying files, for every verification method. The decision text is unchanged and stands in full
  (#25 traceability gates).
- **rev 2** — 2026-08-17 — drops the named exemplar in the dot-directory consequence, which went
  stale a second time (`scripts/check-links.mjs`, retired by [ADR 0016 rev 5](0016-maintained-tools-for-standard-artifacts.md));
  the point about the wiring pattern is unchanged (#160 post-adoption-wave sweep).
- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

WiseKiosk is design-first, but its requirements live only as prose scattered across its design
documents — load-bearing invariants, standing test obligations, and an "each row becomes a testable
requirement… verified by" table. None of it has stable IDs, and nothing enforces that a
requirement actually traces to an artifact that verifies it. That proto-traceability is exactly the
kind of invariant this project prefers to mechanise rather than trust to vigilance: a "verified by"
column with nothing checking it decays silently. The project also has an explicit learning goal, and
internal engineering rigor is a stated driver — so a real requirements-and-V&V methodology is wanted,
not just a register.

## Decision

Adopt **Doorstop** — a Python, Git-native requirements tool — as the requirements and verification
store. Requirements live as per-item YAML under [`../requirements/`](../requirements/README.md) in a
three-document V-model tree: `SYS` (needs) → `SRS` (shall statements) → `TST` (verification items
whose `references` point at the real verifying files). Validation is wired into the same gate
developers and CI already run: a strict `just check-reqs` (`doorstop --error-all`) that fails on any
suspect, unreviewed, orphaned, or unresolved-reference item, mirrored exactly by the `requirements`
job in [`../../.github/workflows/checks.yml`](../../.github/workflows/checks.yml). The pin lives in
`../requirements/requirements-dev.txt` — siloed with the
requirements it serves, not at the repo root; it is dev tooling only, no application Python ships.

## Alternatives considered

- **StrictDoc** — richer (custom grammar, requirement types, reqIF export, HTML/PDF publishing).
  Rejected as heavier than warranted: more methodology surface to learn and maintain than a pre-code
  project of this size needs, for capability it will not use soon.
- **OpenFastTrace** — a mature trace tool, but JVM-based. Rejected: it drags a Java toolchain into a
  Go + Svelte repo whose minimal, native-toolchain-free dependency footprint
  ([`CONTRIBUTING.md`](../../CONTRIBUTING.md), review checklist item 7, *Dependencies*) actively resists exactly
  that. Doorstop's Python is a
  lighter, more idiomatic addition.
- **A homegrown Markdown register + validator script** — a table of IDs plus a `check-reqs.mjs` in
  the existing `scripts/` style. Rejected: it would work, but it teaches no established methodology
  (a stated goal), and it reinvents suspect-link fingerprinting and traceability that Doorstop
  already does correctly. The learning value is in using a real tool's model, not rebuilding a worse
  one.

## Consequences

- **Traceability is now a hard gate, not a convention.** A requirement with no verification item, an
  orphaned item, or a dangling reference fails CI. Editing a parent flags its children suspect until
  re-reviewed, so a requirement change cannot silently leave stale downstream items.
- **Doorstop proves linkage; it does not prove correctness.** A `TST` item's `references` resolving
  to a file only proves the link is real — that the check *passes* is proven by `just verify` and the
  test suite. This division is deliberate and is the mental model to hold: Doorstop = the graph is
  complete and current; the runner = the checks in the graph actually pass.
- **Python enters the dev toolchain.** A venv and a `pip` update-tracking entry are added. This is
  contained to dev tooling; the shipped artifact (a Go static binary) is untouched.
- **Doorstop cannot reference files under dot-directories.** Its reference finder skips any path with
  a dot-prefixed component, so a `type: file` reference to `.github/workflows/checks.yml` (or anything
  under `.github/`, `.venv/`, etc.) does not resolve. Where a requirement is verified by CI wiring
  that lives only under `.github/`, the resolvable artifact is the machine-checked `references` entry
  pointing at the script the wiring runs, and the CI wiring itself is cited in the item's prose. Items
  using the pattern cite this decision rather than naming an exemplar here, which would invert that
  dependency and go stale when the exemplar moves — as it has now done twice.
- **Reviewing is a manual discipline.** `doorstop review` re-blesses a child after its parent moved;
  blindly scripting it away would defeat the re-validation the tool exists to provide.
