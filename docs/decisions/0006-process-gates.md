# 0006 — Gate branch shape, ticket linkage, and PR titles in CI

**Status:** accepted; ticket-metadata obligations extended and the parent-ticket definition added by
[ADR 0013 rev 4](0013-work-tracking-invariants.md), whose read-only stance this ADR's rejected
write-scoped-token alternative supplies; gate path corrected by
[ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md)
**Decided:** 2026-07-22 (process-gates design discussion, ticket #27)
**Rev:** 6

## Revisions

- **rev 6** — 2026-10-01 — drops "strict" from the required-checks consequence: `main` merges through
  a merge queue, not strict up-to-date protection, and how the merge path is protected is
  [`../CI.md`](../CI.md) § *Gate wiring*'s to state, not this record's; what was chosen is
  unchanged, so the Decided date does not move (#414 merge queue).
- **rev 5** — 2026-09-04 — replaces two stale Dependabot references — the branch-shape exemption and
  the rationale for it — with timeless statements naming the dependency updater's own branch prefix
  and title convention; what was chosen is unchanged, so the Decided date does not move (#223
  renovate cutover).
- **rev 4** — 2026-08-17 — corrects the local mirror of the advisory hooks: `just install-hooks`
  named a recipe that no longer exists, renamed `just hooks-install` (toolchain install) with
  `pre-commit install` as the separate step that wires the hooks, when #108 pre-commit adoption
  landed; no gate changes (#160 post-adoption-wave sweep).
- **rev 3** — 2026-08-13 — two *Consequences* the tree contradicted: a count of required CI checks
  saying five against a workflow defining six, and a named legacy branch asserted as blocked which no
  longer exists. Both replaced by the rule they were instances of; no gate changes (#145 prose pass).
- **rev 2** — 2026-08-06 — drops the claim that the gate path is toolchain-free, recording that
  property under *Alternatives considered* as given up, and states gate 3's single-declaration
  property without the file glob that carried it; the four gates are unchanged (#126 absorb
  amendment blocks).
- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

Nothing tied a pull request to a ticket or constrained branch and commit naming: a branch with any
name, referencing no issue, under any title could merge. GitHub's native enforcement for exactly
this — ruleset metadata restrictions on branch-name and commit-message patterns — is
Enterprise-only, so on this repo it is a control that does not function where deployed — plausible,
documented, and inert for its entire life. Whatever enforces these conventions has to live in the
repo and run where the repo runs.

## Decision

Four gates, enforced by a `process` CI job on `pull_request` (to become a required check) and
mirrored locally by `just check-branch` and the advisory hooks `pre-commit install` wires, once
`just hooks-install` has put the toolchain in place:

1. **Branch shape.** Branches are named `type_number-snake_name` — `type` one of `task`, `bug`,
   `design`, `module`; `number` a GitHub issue number; `snake_name` lowercase snake_case. The full
   pattern is `^(task|bug|design|module)_[1-9][0-9]*-[a-z0-9]+(_[a-z0-9]+)*$`. `main` and the
   dependency updater's own branch prefix (`renovate/*`) are exempt and pass with the reason stated.
2. **Ticket link.** The `number` must resolve via the GitHub API to a real issue (not a pull
   request), currently open, whose labels include the branch `type`. A shape-valid branch naming a
   dead or mislabeled ticket fails.
3. **Conventional-Commit PR title.** The PR title matches
   `type(scope)?!?: subject` with `type` one of `feat`, `fix`, `docs`, `style`, `refactor`, `perf`,
   `test`, `build`, `ci`, `chore`, `revert`. The repo squash-merges, so the title is the commit
   that reaches `main`.
4. **Recorded PR↔ticket linkage.** Every open PR's Development field (`closingIssuesReferences`)
   must link the branch's issue, whatever the PR's base — so the ticket closes when the PR merges
   into the default branch.

- **The branch types are exactly the issue-template set**, so branch type and ticket template
  cannot drift: a `bug_…` branch links a bug-report ticket, a `design_…` branch a design-decision
  ticket, and a new template implies a new branch type in the same change.
- **CI gates only what survives the squash.** Branch commit messages are discarded at merge; an
  advisory `commit-msg` hook applies the same pattern locally, additionally passing
  `fixup!`/`squash!`-prefixed and merge messages, which never reach `main`. Each pattern is
  declared once and read by every stage that enforces it — a stage carrying its own copy of a
  pattern is what lets the two drift apart.
- **Gate 4 constrains GitHub's recorded state, not prose.** The record is the PR's Development
  field (`closingIssuesReferences`): link the issue there, or let a body keyword (`Closes #N`)
  write the same record — the gate checks the record pre-merge as a required-check constraint, on
  every base. Integration and epic branches are supported practice: body keywords observably
  record nothing against a non-default base, so a PR into one carries its link via the manual
  Development-field link instead. The close semantics compose correctly — a linked ticket closes
  only when the PR merges into the default branch, so an epic-internal merge leaves its ticket
  open until the work reaches the mainline. Write paths are asymmetric and closed: no public API,
  GraphQL mutation, or CLI writes the manual link (schema introspected, 254 mutations) — the body
  keyword is the only scriptable writer, and only against the default base; the manual link is
  UI-only and confirmed live to satisfy the gate.
- **This is a process/scheduling control, explicitly not a traceability channel.** Requirements
  trace stays diff-derived per [ADR 0005 rev 4](0005-traceability-gating.md); the branch↔issue link
  schedules work, it never evidences it.

## Alternatives considered

- **GitHub-native rulesets** (metadata restrictions on branch and commit patterns). Rejected:
  Enterprise-only, so on this repo the control is inert where deployed — plausible, documented, and
  doing nothing.
- **Gating every branch commit's message in CI.** Rejected: those messages are discarded at squash,
  so CI would block work over text that never reaches `main`. CI gates the surviving title; local
  hooks advise on the rest.
- **Decoupling branch types from issue templates** (the Conventional-Commit vocabulary as branch
  types). Rejected: it loses the branch↔ticket-template correspondence — the property that a
  branch's type names the template its ticket was opened from.
- **Shape-only check without resolving the issue.** Rejected: a typo'd number passes, and the link
  the gate exists to prove goes unchecked.
- **Regex over the PR body for a closing keyword.** Rejected: prose can claim a link the platform
  never recorded — observed live on a stacked PR carrying the keyword with an empty
  `closingIssuesReferences`. The gate reads the recorded state, not the text.
- **CI creating the linkage itself via a write-scoped token.** Rejected: gates verify, they do not
  mutate, and CI stays read-only.
- **A gate path with no toolchain** — *plain sh + curl + jq — no toolchain*, the property this record
  was decided under. Rejected under
  [ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md): the Conventional-Commit
  convention is a public one rather than a rule of this repository's own, so it is delegated to a
  maintained tool and the local hook layer follows it there. What the property bought was a
  contributor needing nothing installed to run the gates; what it cost was an authored parser for a
  convention that already has a maintained implementation.
  [ADR 0017 rev 9](0017-authored-language-set.md) ends what remains of it, moving the authored gates
  to Python.

## Consequences

- Every gate job the workflow defines is required on `main`, `process` included (admins bound),
  so a PR from a nonconforming branch cannot merge. That the required set equals that job set
  is [`../CI.md`](../CI.md) § *Gate wiring*'s to assert; a count here would be falsified by adding a
  gate job, by someone with no reason to open this record.
- A branch that predates this rule is blocked until renamed and its ticket labeled, the gate reading
  the branch it is given rather than when it was created.
- The dependency updater is exempt from branch shape; its PR titles already conform to the
  convention this gate checks.
