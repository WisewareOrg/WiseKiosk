# 0009 — Record what a verification settles, and what it does not

**Status:** accepted
**Decided:** 2026-07-24 (closing review pass of the requirements rewrite #18)
**Rev:** 3

## Revisions

- **rev 3** — 2026-10-15 — drops the `scripts/check-method-consistency.py` path, retired with the
  other six requirements-tree scripts in favor of wise-ci's `check-reqs`; what was chosen is
  unchanged, so the Decided date does not move.
- **rev 2** — 2026-08-06 — the Context argues the ground the decision is held to — that the attribute
  records the limit of a check as well as the blocker below one — with the audit evidencing it, and
  states each count as a measurement of its moment rather than of the tree (#126 absorb amendment
  blocks).
- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

[ADR 0005 rev 4](0005-traceability-gating.md) gave every item a `verification-method` and routed the gates
on it. What the method never carries is the argument behind its value: it records a conclusion and
discards what produced it. That costs on both sides of the `test` boundary.

**Below `test`**, an item weak because proving a negative is genuinely undecidable is
indistinguishable, in the file, from one weak because nobody tried. A promotion pass has nothing to
audit against and must re-derive every verdict from scratch, and the weakening direction is
unguarded: a method downgraded under deadline pressure reads exactly like one that was always
correct.

**At `test`**, the same discard is harder to see, because the file reads as settled. A green check
decides what it asserts and nothing more, and nothing in the item records where that stops.

The first cost is what forced the decision. It was taken partway through #18's closing pass, which
pushes every item to the strongest method it honestly supports; 115 of the tree's 236 items
then sat at `inspection`, `analysis` or `demonstration`, and that pass could be neither checked nor
repeated unless each item's blocker was stated. The pass promoted most of them and the #69 tree
rebuild finished the work, leaving every item at `test` — so the residue those grounds describe is
empty.

The second cost is what the attribute is held to, and #69 tree rebuild's audit of 2026-07-28 is the
evidence for it. Three reviewers, filing blind against opposed briefs, produced two of their
strongest convergent findings by reading justifications:
`SRS027`'s<!-- The display page holds no device capability it does not use --> admission that no
check settles *"whether an entry on that allowlist is justified"* exposed a check asserting over a
set no artifact defines, and
`SRS028`'s<!-- Served responses declare their type, and forbid the browser inferring one -->
*"whether a declared type is the correct one for the bytes beneath it"* scoped all three readings of
a requirement that obliged the browser rather than the product. Fourteen items carried the attribute
at the time; both findings came from that fourteen.

## Decision

A fourth stored attribute, `verification-justification`, free text. **Required on every item**, it
records what that item's verification settles and what it does not:

- **Below `test`** — what specifically blocks a mechanically-decidable check: human judgment, live
  credentials, physical hardware, a wall-clock window, an unbounded search space, a property true
  only of the whole corpus.
- **At `test`** — what the mechanical check leaves unproven. A check asserting a schema's key set
  against a committed allowlist decides set equality, not that no key is secret-bearing; a check
  asserting a render under emulation decides that the page renders on that architecture, not that it
  renders fast enough on the device. Both pass while the obligation above them is unmet, and the
  field is where that gap is stated.

It is fenced by the review fingerprint alongside `verification-method` and `rationale`, so weakening
a method cannot land without re-review.

The stored-attribute set is [ADR 0005 rev 4](0005-traceability-gating.md)'s, and this record is what
puts `verification-justification` in it — the addition 0005 records as superseding its own earlier
set. 0005's method values, its gate table, its derived-verification model, and its tree-as-backlog
stance all stand unchanged.

Two distinctions the attribute depends on:

- **`rationale` answers why the obligation exists; `verification-justification` answers why a machine
  cannot settle it.** Different questions with different lifetimes — an obligation's reason for
  existing survives a change in how it is checked, and a blocker can dissolve the moment a tool
  appears without the obligation moving at all.
- **It is not the reference channel.** 0005 closes analysis and demonstration items through a
  referenced artifact; that reference is the *evidence the weaker method produced*. The justification
  is why that channel is in use at all. An item at `inspection` owes both.

The gate asserting that every item carries a non-empty justification is check-reqs's
`check_method_consistency` stage. Of the gates it was to be built alongside,
[ADR 0005 rev 4](0005-traceability-gating.md) drops the reverse-direction one, and gate 3 is a
tree-blind coverage bar (#190 coverage closure gate), folded into `just verify` like every other
gate here.

## Alternatives considered

- **Fold it into `rationale` as a trailing convention.** Rejected: it conflates two questions in one
  field, so neither can be gated independently — a check asserting "every non-`test` item states its
  blocker" cannot tell a justification paragraph from the surrounding prose without parsing English.
  It also breaks the one-fact-one-home rule the corpus was reconciled against under #42, held today
  by [`CI.md`](../CI.md)'s documentation-integrity gates, inside the very tree that states it.
- **State it in the `TST` item's `text`.** Rejected: the non-`test` items then spanned all three
  tiers, and the `SYS` tier — where `analysis` and `inspection` concentrated, and where there is no
  parent to inherit an argument from — would have stayed uncovered entirely. It also puts a claim
  about verification method inside a field reserved for the obligation.
- **Add the attribute but leave it outside the review fingerprint.** Cheaper, and measurably so:
  populating a fenced attribute across the 115 items then below `test` cost 115 re-reviews plus 76
  suspect-link clears, cascading from the 34 items with baselined children. Rejected anyway — an
  unreviewed justification is precisely the rot the attribute exists to prevent, and the cost is paid
  once during a pass that re-reads those items regardless.
- **Derive it from the method and the item's references.** Rejected: the blocker is not recoverable
  from either. Two items can share a method and a reference shape and be weak for unrelated reasons.

## Consequences

- **Neither direction is free.** Moving an item away from `test` requires naming the blocker; moving
  it to `test` requires stating what the mechanical check leaves unproven. A method change always
  rewrites the field and re-passes review, so a weakening cannot arrive as a silent deletion and a
  promotion cannot arrive as an unexamined win.
- **Adding the attribute cost no fingerprint churn.** Doorstop's `stamp()` hashes only attributes
  present in an item's own file and does not write defaults back retroactively, so the items already
  in the tree were untouched by the schema change; only items that gain a justification re-flag.
- **A per-item audit trail replaces a per-pass one.** A reader asking what the tree's verification
  leaves unsettled gets one specific answer per item instead of a policy statement.
- **The docs site gains a column where it renders verification fields** — the justification sits in
  needtables beside the method, so what each check leaves unproven is browsable rather than
  discoverable only by grep. `docs/site/doorstop_to_needs.py` emits no verification field and needs a
  `needs_extra_options` declaration for one, which is #191 verification matrix's work.
- **It adds an authoring obligation to every item**, including pending ones. An item written before
  its blocker is understood must state the blocker; one claiming `test` must state what the check
  leaves unproven. "Unsure" has no representation, which is intended.
