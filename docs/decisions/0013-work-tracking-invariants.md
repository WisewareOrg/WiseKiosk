# 0013 — Gate the ticket's metadata at merge, and define a sub-issue as a shared merge target

**Status:** accepted
**Decided:** 2026-08-02 (work-tracking invariants discussion, ticket #64 work-tracking invariants)
**Rev:** 4

## Revisions

- **rev 4** — 2026-08-17 — renames `scripts/check-branch.sh` to `scripts/check-branch.py` throughout,
  converted by #109 check-branch conversion; no ground changes (#160 post-adoption-wave sweep).
- **rev 3** — 2026-08-06 — drops the appended restatement of the advisory half's grounds; the ground
  that holds for any filer is stated once, under *Consequences* (#126 absorb amendment blocks).
- **rev 2** — 2026-08-05 — corrects a ground that named the retired amendment mechanism; the CI-mutation stance is unchanged (#118 ADR revisions).
- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

[ADR 0006 rev 6](0006-process-gates.md) specifies exactly one property of a ticket — that it is open and
type-labelled — and only because `scripts/check-branch.py` can decide it. Everything else about how
work is tracked was convention, and convention drifted twice in five days. On 2026-07-24, ten of
sixteen open tickets could not legally be branched on: unlabeled, or carrying stock `enhancement` in
type duty, which is not a branch type. Four days after that cleanup, #70 configuration generator
arrived with the identical defect, alongside three tickets with no milestone and four stale sub-issue
links to a closed parent. A convention a document merely describes is a comment-enforced invariant —
the defect class this project forbids for code, wearing project-management clothes.

The root cause is not habit. `gh issue create --title … --body …`, which is how a ticket gets filed
from an agent session, bypasses the template picker entirely: all four `.github/ISSUE_TEMPLATE/`
files apply to none of it, so no type label, no headings and no milestone arrive. GitHub also drops a
template's declared label when no such label exists, which is how `new_module.md` — declaring
`labels: module` against a label this repository had never created — produced unlabeled, unbranchable
module tickets from the path that was supposed to be the safe one.

What GitHub does **not** offer is enforcement at creation. An `on: issues` workflow runs after the
issue exists; there is no equivalent of a required check for issue creation. Every candidate below is
therefore detection, and the only question is where the detection sits and whether it can refuse
anything.

## Decision

Four candidates were weighed. Two are gated in `check-branch.py`, one is recorded ungated, one is
dropped.

**Gated: the ticket a branch names carries a milestone and exactly one type label.** The type set is
read from `scripts/branch-shape.regex`, never restated, so adding a branch type cannot leave the
label rule behind; an unreadable set fails rather than counting zero. A second type label makes the
branch type ambiguous — the branch type is supposed to name the template the ticket was opened from,
and two candidates break that correspondence. An unmilestoned ticket is absent from the phase axis
that carries the definition of done. Both read fields already present in the issue payload
`check-branch.py` fetches, so neither costs an API call.

**Gated: a sub-issue means a shared merge target.** A parent ticket exists *iff* there is an
integration branch, and a ticket is that parent's sub-issue *iff* its pull request targets that
branch. All other grouping is the milestone's. This is forced rather than chosen: an integration
branch is a branch, so under ADR 0006 rev 6 it links an open type-labelled issue of its own, and that issue
cannot close until the branch merges. It also separates the two progress bars — the milestone answers
*is the phase done*, the parent answers *is this branch ready for the mainline*. The gate asserts the
biconditional in both directions, and a non-default base that is not itself a conforming branch fails
rather than skips, because an anchor the check cannot resolve must not report success.

**Ungated, recorded: ordering lives in native dependency edges.** No hand-maintained `⛔ Blocked by:`
line in an issue body; ready-to-work is the derived `-is:blocked` view. The practice is adopted — the
mirror lines were stripped on 2026-07-24, and four of the eight found had already drifted from the
edges they mirrored. It is not gated; see below.

**Dropped: phase gating as a property of the graph.** That blocked work cannot surface as ready is
GitHub's `-is:blocked` behaviour given edges that exist, not an obligation on anyone. It states no
proposition, so nothing can fail it, and recording it in [`../CI.md`](../CI.md) would put a
non-decidable statement into the document that exists to hold the decidable ones —
[ADR 0011 rev 2](0011-requirement-or-convention.md)'s mistake, one level up from where it usually happens.

Enforcement is at merge and read-only. It fires on the change whose ticket is wrong, never on an
unrelated one, and it refuses a merge rather than annotating an issue.

## Alternatives considered

- **Detect-and-flag on `issues:` events** — an Action labelling `orphan`/`needs-triage`, which is
  what #64 work-tracking invariants proposed. Rejected: it needs `issues: write`, and ADR 0006 rev 6
  rejected CI mutation on the ground that gates verify rather than mutate. That stance is **held
  here, not revisited**. The underlying argument is narrower than its wording — what it forbids is CI
  writing the evidence a gate then reads, which a flagger does not do — but the owner chose to keep
  the broad line rather than carve it. The flagger also would not have been construction-time
  enforcement in any case, only earlier detection.
- **A read-only scheduled sweep over every open issue.** Rejected: a new workflow, a schedule, a
  `CI_ONLY_ALLOWLIST` entry and its own verification record, to reach a backlog population that has
  been caught by eye twice at no cost. It remains the named remedy if the drift recurs.
- **Auditing every open issue from the pull-request job.** Rejected: it fails somebody's change over
  an unrelated ticket's metadata. [`../CI.md`](../CI.md) § *Upstream contract checks* already settles
  this shape — a gate that goes red for something outside the author's change is one authors learn to
  ignore.
- **Gating the `⛔ Blocked by:` string.** Rejected: a literal-string ban flags the very documents that
  forbid the line — the ticket establishing the rule was the only open issue matching it — and it is
  respelled for free as "Depends on". What enforces the invariant is that there is no second place to
  write ordering; the duplicate was deleted, so there is nothing left to drift from.
- **Deferring the sub-issue gate for want of a second consumer.** There has been one integration
  branch, so `CONTRIBUTING.md`'s no-second-consumer rule argued for waiting. Rejected by the owner: a
  second is imminent, and the historical record already holds an instance the gate would have caught
  — PR #79 merged into `design_18-closing_review` while its ticket #69 tree rebuild was never a
  sub-issue of that branch's anchor.
- **Writing these as requirements in the tree.** Rejected under ADR 0011 rev 2: nothing the running kiosk
  does can violate any of them.

## Consequences

- **A ticket nobody works is never seen.** The gate reaches an issue only when a branch names it, so
  a malformed ticket left in the backlog stays malformed and stays absent from its milestone's
  progress. This is the cost of holding ADR 0006 rev 6's read-only line, and it is accepted knowingly.
- **#64 work-tracking invariants' own timing argument is overturned rather than ignored.** It held
  that a branch-side gate fires too late, when the context that would have made the ticket correct is
  gone. That is sound for body *content*, which decays; it is weak for a milestone and a type label,
  which anyone can supply correctly weeks later in seconds and lose nothing.
- **The label set is load-bearing.** `module` was created and stock `enhancement` deleted. A new
  branch type requires a label of that name to exist before its template can apply it, or the picker
  silently drops it and the safe path produces an unbranchable ticket again.
- **The write-time half is advisory by construction.** Nothing at filing time refuses a malformed
  ticket: GitHub cannot decline to create one, and CI does not write. That is why the merge gate is
  the half with teeth, and the pairing is ADR 0006 rev 6's own — an advisory `commit-msg` hook beside a
  required title check.
- **Judgment stays judgment.** Whether a ticket should be rescoped or closed, which close-reason
  applies, whether a scope is correct, and whether a body's acceptance condition is any good are not
  gated, and no gate should pretend to decide them.
