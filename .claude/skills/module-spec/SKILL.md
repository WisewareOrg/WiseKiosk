---
name: module-spec
description: >-
  Author a WiseKiosk display module's specification — its one `SYS` need, the `SRS` decomposition
  bounded by the framework universals, a pending `TST` stub per requirement, and the module drawn in
  the architecture model. Sequences the work and points at the contract that decides its content.
  Invoke when adding a module to the requirements tree, before any of that module's code is written.
---

# Specify a module

**This is the requirements & architecture phase — the middle of a module's three** (UI design spec →
requirements & architecture → implementation); the epic and integration-branch shape they sit in is
the module contract's ([§ How a module is tracked](../../../docs/contracts/module-contract.md)). This
skill sequences this phase; it does not file the epic or the sibling phases.

**A module's requirements are written before it is built** — after phase 1's UI design spec has
decided what the module shows, and before any of the build steps. This skill is the sequence; what
each item must *say* is
[`docs/contracts/module-contract.md` § Writing the module's requirements](../../../docs/contracts/module-contract.md),
and it is authoritative. Do not re-derive its rules from here — read it. Read
[§ A module is its capability, not its supplier](../../../docs/contracts/module-contract.md) with it:
a module is identified by what a viewer gets from it and never by the service it happens to read, so
**no requirement names a supplier** — not the need, not anywhere in the decomposition. The supplier is
named in the architecture model's external system and in the route key, and nowhere in the tree.

That a module is a need at all is
[ADR 0012 rev 2](../../../docs/decisions/0012-module-requirements-in-tree.md). A module not on the
roster in [`README.md`](../../../README.md) has nothing authorising a need for it.

## Which shape

**Upstream-backed** or **local**, and it changes the size of the job. A local module renders from
something already present, fetches nothing, and has no shaping library, no route registration and no
boundary-schema fragment — so no source, parameter-pattern, payload or timing items, and one drawn
element. Settle this first; most of the contract's clauses are marked with the shape they apply to.

**What crosses the boundary is generated on both sides, and that reaches further than the payload.**
The names of the parameters a request carries and the props the failure path renders are boundary
values like any other — [`docs/TESTING.md` § Tiers](../../../docs/TESTING.md) enumerates the classes
the Boundary tier covers, and SRS016<!-- Both sides consume the generated types --> obliges both
sides to consume them rather than declare a twin. So a module author writes none of them by hand: not
a parameter name in Go, not an error prop type in the component. Where a value has to exist for the
module to work, it goes into the schema fragment (the contract's part 6) and is read back from the
generated types.

## Enumerate the figures before you write any

**Do this before step 1.** Every observable number the module will ship is written down in one list,
first, while the list is still short enough to be honest. What is on that list is the contract's
([`docs/contracts/module-contract.md` § Writing the module's
requirements](../../../docs/contracts/module-contract.md)) and is not restated here — a second copy
goes stale against the governing one with nothing comparing them. This is the place in the sequence
where that enumeration is discharged.

**Each figure leaves the list by one of exactly two routes.** Into a requirement that argues it — and
then the figure is in the item, argued in the item, and the code reads it from there. Or into the
module's own written record that nothing constrains it, saying which choice it is and why the
specification could not have decided it. A figure that leaves by neither route is invented, and it is
invented whether or not it is a good number: what makes it invention is that nothing in the
specification could have been read differently to produce a different one, so nothing can ever find
it wrong.

**The record is documentation, not a comment at the constant.** A comment says what the constant does
and may cite the record; it is not the record. A comment carrying the argument as well is the shape
the framework's own timeout constants take, and it is still not the record: what makes the reason
findable is the citation in it, pointing at the document where the figure is written down. A reason
living only beside the code is unfindable by anyone reading the specification, which is where the
next author looks to see whether the figure was a decision or an accident.

Write the list into the change that carries the module's items — a commit message or the pull request
— with the route each figure took. It costs a paragraph and it is the artifact that makes a missing
figure visible to a reviewer, who otherwise sees only the figures that were written.

## The sequence

1. **One `SYS` need.** What a viewer gets, one sentence, one `shall`, enumerating nothing — a need
   listing its own decomposition is a hat over its children. Header an indicative noun-phrase claim,
   not the sentence beneath it repeated. Written for a reader with no stake in the code.
   **It obliges a capability** — *shall be capable of showing …* — because a module is configured in
   or left out; the contract says why.
2. **The `SRS` decomposition.** What is true of this module and nothing else. Each names its module —
   beneath a module need that is the item's correct form, not an instance to triage away.
3. **One `Pending:` `TST` stub per active `SRS`** — [pending-stub](../pending-stub/SKILL.md).
4. **Draw the module** — [arch-model](../arch-model/SKILL.md). Every module gains a component under the
   frontend; an upstream-backed one also gains a backend component, an external system and the edge to
   it. This lands in the **same change** as the items, because they land accepted and active and every
   accepted active item must bind to something drawn.
5. **Cite these items into the phase-1 UI design spec.** That spec was authored first and could not
   yet name the requirements it realises ([module contract, § The module's UI design
   spec](../../../docs/contracts/module-contract.md)); now that the items exist, add those citations
   to `frontend/src/modules/<module>/README.md`. No gate enforces it, so it is a step here or it is
   left undone.

Item mechanics throughout — creating, linking, the two stamps, the per-item checklist — are
[tree-item](../tree-item/SKILL.md)'s.

## Commit granularity, and a red step 2

**Steps 2 and 3 are gate-complete only together.** An active `SRS` with no child link errors the
strict gate, so a commit holding step 2 alone reds `just check-reqs` until step 3's stubs land, and a
commit holding steps 1–3 without step 4 reds `check-arch-trace` until the model does. Both are
expected rather than a mistake.

**Either shape is fine, and the choice is the change's, not this skill's.** One commit per step
reviews the reasoning in the order it was made and leaves intermediate commits red; one commit for
steps 2 and 3 together leaves every commit green and merges two arguments into one message. **Only
the PR head is gated** — CI runs against the merge result, not each commit — so a red intermediate
commit blocks nothing. Where a step is red in isolation, say so in that commit's message, so the next
reader meets the reason rather than a mystery.

## The boundary, and its second stage

The decomposition stops where the framework universals begin — what is obliged of every module
already, which a module restating has written twice.

**Do not check a draft against a list of them.** The boundary is categorical and a roster was
explicitly refused, because a list is wrong the moment the framework gains an item and an author
working down it stops thinking. The test is what a draft leaves once its module is taken out of it,
and **it runs over rewording and over abstraction — an author is done only when both have been
tried.** Rewording swaps the module named and keeps everything else. Abstraction goes a step further
and drops what is particular to the module, leaving the shape underneath: *the weather module shall
report on the location its configuration names* abstracts to *a module reports on the subject its
configuration names*. Rewording alone misses the item most worth catching, because an item whose
every clause names its own subject survives it untouched while the sentence beneath it is a
universal; abstraction alone runs the other way, since abstracted far enough every item reduces to *a
module does what it is for*, so the abstraction that counts is the one that drops the module and
keeps the obligation. Where the two disagree, the abstracted reading is the one that goes to the
second stage.

**And if what the test leaves is a sentence the framework *would* say but does not — stop.** That is
not the same answer and the first stage cannot tell them apart, and dropping it loses a real
obligation silently.

**So the second stage asks two questions rather than one, and it has three outcomes.** Does a
framework item say it — if yes, drop the draft. Does the contract itself say it — if yes, the draft
cites that clause in its own `rationale`, and nothing is promoted and nothing is escalated: the
contract states structure directly and what it states binds without a tree item behind it
([ADR 0011 rev 2](../../../docs/decisions/0011-requirement-or-convention.md)). Only when neither says
it is the sentence homeless, and only then is it the owner's: it becomes a framework item, or this
module's against a recorded trigger for promoting it — and **which one is the owner's call**. Surface
it; do not choose. **Search both before raising one.**

## Escalate rather than invent

A module spec is where invented interfaces are cheapest to write and most expensive to find. A
configuration key's name and default are the configuration schema's, not a requirement's — an item
states what the choice *does*. Where the specification is silent on something observable, halt and
ask.

**Halt and ask is a rule with a list, because as an instinct it does not fire.** Every one of these
reviews as ordinary work when it is invented, which is what makes it worth naming them rather than
trusting care:

- **An interface name** — a route, a type, a field, a constant something else must spell the same.
- **A payload shape** — what crosses the boundary, and which of it is required. A field declared
  required ahead of anything that reads it is its own case, and the contract rules on it (part 6).
- **A configuration key** — its name, its default, and whether it is offered at all.
- **A failure behaviour** — what the module does when its source answers wrongly, partially, or not
  at all, and what a viewer sees while that lasts.
- **A threshold** — every figure the enumeration above collects, plus any bound a check would have to
  be told.
- **A new file or a new dependency** — where it sits, what it brings with it, and whether an existing
  home would do.
- **The input transport** — how a request carries what it carries. It is the schema's to state and a
  module author's to ask about, never to pick.
- **The cache policy** — what an answer is keyed under, how long each kind of answer is held, and
  what a second asker gets while the first is outstanding.

Asking costs a message. Each of these, invented, costs a review round at best and ships a decision
nobody made at worst — and it ships looking exactly like a decision somebody did make.

## Closing

`just check-reqs`, `just check-citations`, `just check-arch-trace`, `just check-arch`. Those four are
what this work can fail, and all four run from `docs/requirements/.venv` and
`docs/architecture/node_modules`, which the module-spec path already needs.

Then `just verify` — with one caveat, because **it halts at the first failing recipe**, so a stop
early in the list means every gate after it did not run and the change was not measured:

- **`check-site` needs `docs/site/.venv`**, which nothing on this path installs. Create it with
  `just site-install` once, or expect the stop.

This failure says nothing about the module spec. When `verify` stops on it, run the rest by
name rather than reading the stop as a verdict — the recipe list is in the `verify` line of the
[justfile](../../../justfile). **CI is the source of truth either way**, and it has both.

## Done when the documents agree again

The gates above prove the tree is consistent and the model is drawn. **None of them reads whether the
prose still describes what was built**, so it is a step rather than a consequence:

- Every document the change falsified is corrected in the same change. Which document describes what
  is the [documentation index](../../../docs/README.md)'s, and which *kind* of statement belongs in
  which home is
  [ADR 0011 rev 2](../../../docs/decisions/0011-requirement-or-convention.md)'s: an obligation on the
  running software is a requirement, a convention a machine settles is a check, an obligation on an
  author that leaves no artifact is a checklist question.
- **A reason that ended up in the wrong home is the common case, not a rare one.** A decision written
  into an as-built description, a rationale written into a code comment, a figure argued in a commit
  message — each reads as documented and none of them is findable where the next author looks.
- The review checklist's questions 1 and 2 (*Formalised prose*, *Described code*) are the same
  obligation put to a reviewer; answering them here means the reviewer confirms rather than finds.

Adding a module is a test-architecture review trigger
([`docs/TESTING.md` § Review cadence](../../../docs/TESTING.md)).
