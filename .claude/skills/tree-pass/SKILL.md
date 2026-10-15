---
name: tree-pass
description: >-
  Run a triage pass over a tier of the WiseKiosk Doorstop tree — SYS, SRS or TST — deciding which
  items earn their place and rewriting the ones that stay. Carries this repo's mechanics: how to
  parse the tree, the invariants to re-check after every ruling, the method rule, the review
  fingerprint, and how decisions are recorded. Invoke when condensing or reviewing a tier, when a
  requirement "reads like a list", or when auditing the tree for orphans and method mismatches.
  For the judgement itself — what earns its place — use the requirements-triage skill.
---

# Tree pass

Judgement lives in **`requirements-triage`** (global skill): the tells, the dispositions, the
governing rule that a requirement states a want. Load it first. This skill carries only what is
specific to this tree.

Conventions are in [`docs/requirements/README.md`](../../../docs/requirements/README.md) and are
authoritative where this file and it disagree.

## Mechanics you cannot do by eye

Doorstop links are a list of either bare UIDs or single-key mappings. Parse, never grep:

```python
import yaml, glob, os, collections
items = {os.path.basename(p)[:-4]: yaml.safe_load(open(p))
         for p in glob.glob('docs/requirements/*/*.yml')}
kids = collections.defaultdict(list)
for k, v in items.items():
    for l in (v.get('links') or []):
        kids[l if isinstance(l, str) else list(l.keys())[0]].append(k)
```

Run the tool from its own venv — it is siloed with the feature it serves:
`docs/requirements/.venv/bin/python`, installed by `uv sync --project docs/requirements`.

**`pathlib.Path.glob("*.yml")` matches dotfiles; `glob.glob` does not.** Each silo holds its own
`.doorstop.yml`, so a script written with `pathlib` parses three config files as items and every
count it prints is three too high. Skip stems beginning with `.` — two shipped checking scripts had
this bug, and it is why a justification worklist was reported as 88 when it was 81.

**`git checkout -- <path>` restores from the index, not `HEAD`.** After anything has staged a
corrupted tree it appears to succeed while preserving the corruption. Use
`git restore --source=HEAD --staged --worktree <path>`.

## The gate writes to the tree

**Validating the tree modifies it.** Doorstop stamps a review fingerprint into any item whose
`reviewed` is absent or `null`, and into any link carrying no stamp — whether or not a person looked
— and then `git add`s what it changed. `--no-reformat` stops the wholesale file rewriting but not
this. `doorstop review` and `doorstop clear` stage what they rewrite too, so check what is staged
before committing.

So `reviewed: null` is **not a stable "unreviewed" state**. It survives exactly until the next gate
run, and the run that clears it is the one that was supposed to be checking for it. This is the most
likely explanation for bookkeeping drift that five review rounds found and none could source.

`scripts/check-unreviewed.py` runs **first** in `check-reqs` and fails, so Doorstop never reaches the
tree while anything is unstamped. Two rules follow:

- **Never clear that failure by re-running the gate.** It is `doorstop review <uid>`, deliberately,
  after reading the item against its parent.
- **If you run bare `doorstop` by hand, check `git status` afterwards** and restore anything it
  touched. Investigating a tree question costs a mutation you did not ask for.

## Identifiers that changed

A renumber rewrites `links:`. It does **not** touch identifiers written inside `text`, `rationale`
or `verification-justification`, and those fail in two ways:

- **Dangling** — names an item that no longer exists. A citation resolver finds these.
- **Re-pointed** — resolves cleanly to *the wrong requirement*, because that number now belongs to
  something else. **No checker can see this.** A `TST` item citing "the `SRS026` rejection body" read
  perfectly while `SRS026` had become the no-client-identity item.

Re-pointed references are only recoverable through a map, and the map is not in the renumber commit —
git's rename detection catches a fraction. Derive it by matching item text across the commit:

```python
def tree(rev):   # uid -> normalised text, at that revision
    ...          # git ls-tree + git show, yaml.safe_load each item
old, new = tree('<renumber>^'), tree('<renumber>')
```

Every item should match exactly one. Save the result — `~/wisekiosk-69-artifacts/renumber-map.tsv`
is the one for the 2026-07-27 renumber. **Any future renumber needs one built the same way, in the
same commit.**

Then judge each reference in context: a number written *before* the renumber means what it meant
then, one written after means what it says now. The map cannot tell you which, so read the sentence.

## Invariants — re-run after every ruling, not at the end

1. **No orphan.** Every `SRS` has a surviving `SYS` parent; every `TST` a surviving `SRS`.
2. **No childless active item.** An active parent with no children fails the strict gate.
3. **Method rule.** A parent's `verification-method` equals the **least-decidable** method among its
   children, ordered by mechanical decidability: `test` > `analysis` > `inspection` >
   `demonstration`. Gated by `scripts/check-method-consistency.py`. Re-parenting one `inspection`
   child onto an all-`test` parent silently drags the parent down — this has happened, three times
   in one pass.
4. **Cascade recorded.** Deleting an `SRS` orphans its `TST` children. Enumerate them, and note
   which were `active` with real `references` — those are live checks, not placeholders.
5. **Clause-level tracing.** `--error-all` links item-to-item and cannot see a clause of a child
   tracing to no clause of its parent. **This tree's dominant defect class.** Every merge must be
   checked clause by clause by a human.

## The review fingerprint

`Item.review()` stamps `stamp(links=True)`, which hashes the **UID**, the text, `ref`, the references,
the three fenced attributes **and the sorted parent UIDs** (`doorstop/core/item.py`). Four
consequences:

- Renaming an item unreviews it, and makes every child suspect via the parent stamp in `links:`.
- A `reviewed` stamp copied across a UID change cannot match — the tooling catches forged carry-over.
- `verification-method`, `verification-justification` and `rationale` are inside the fence; `status`
  is not.
- **Re-parenting an item unreviews the item itself**, so a moved child needs `doorstop review`, not
  `clear` alone — `clear` rewrites the stamps stored *in* `links:`, and those are not in the hash.
  `--error-all` reports it as `unreviewed changes`, which is why a pass that re-parents active items
  re-stamps them as it goes.

Clearing suspect links and re-stamping is a **human read**, never scripted
(`docs/requirements/README.md`). Inactive items are skipped by Doorstop entirely, so a stamp on a
pending item carries no authority — edits to its *content* are nearly free, and expensive after
activation. Its *links* are not free: re-point one and `scripts/check-suspect-links.py` fires. **Neither
`doorstop review` nor `doorstop clear` can reach an inactive item at all**: `Tree.find_item` is
active-only and both answer `no item with UID`. Stamping a pending item means a loop over
`document._iter()`.

## Where content goes when it leaves the tree

| Content | Home |
|---|---|
| Product premise, operator truth | `README.md` |
| Settled decision with a rejected alternative | `docs/decisions/` (ADR) |
| As-built structure | `docs/ARCHITECTURE.md` — **facts only**, never rationale |
| Test strategy | `docs/TESTING.md` |
| A repository convention a machine decides, or material CI produces | `docs/CI.md` — **outside the tree** |
| An obligation on what ships beside the image, not on the kiosk | `docs/DEPLOYMENT.md` |
| An authoring rule no machine can decide | `CONTRIBUTING.md`'s review checklist |
| A reason | the item's own `rationale` |
| "Revisit if X" | a **reopen premise** in `rationale` |
| Uncommitted feature | a GitHub issue, with the reopen path written into it |
| A lint rule or threshold | tool configuration, described in `docs/CI.md` |

The last three rows are [ADR 0011 rev 2](../../../docs/decisions/0011-requirement-or-convention.md)'s
routing. Two traps in applying it:

- **A convention does not demote to the verification tier.** That tier is inside the tree, where a
  `TST` item carries a status, a fingerprint and gate enforcement — so putting a lint rule there
  keeps it a specification change, which is the thing demoting it was meant to avoid. It leaves the
  tree entirely.
- **A rule with no activation path is a dead letter.** Before routing anything to the review
  checklist, name who is prompted to apply it and when. ADR 0011 rev 2 deletes `inspection` items on
  exactly this ground; an authoring rule parked where nobody walks it fails the same test.

Every item must carry a `verification-justification` — below `test`, what blocks a mechanical check;
at `test`, what the check leaves unproven ([ADR 0009 rev 2](../../../docs/decisions/0009-verification-justification-attribute.md)).

## Recording — in the repository, never in a ticket

**Apply each ruling to the files as it is taken, and record why in the commit that carries it.** The
record lives where the artifact lives. An earlier pass made an issue thread the authoritative record
and it failed in every available way: nothing in the repo can check it, no gate fails when it rots,
it cannot be read from a clone, and after a renumber its ~70 comments described a tree that no longer
existed. It also contradicts `CLAUDE.md`'s standing rule that no reference point sits outside this
repository.

| Decision about | Home |
|---|---|
| An item that survives | its own `rationale` — fenced by the review fingerprint, cannot drift from the item |
| An item deleted, moved or merged away | **the commit message that removes it** |
| A rule that governs the next pass | an [ADR](../../../docs/decisions/README.md) |

The middle row is the one that gets skipped, and it is the reason decisions leaked into a ticket in
the first place: a surviving item carries its reasoning forever, **a deleted one leaves nothing**
([ADR 0011 rev 2](../../../docs/decisions/0011-requirement-or-convention.md)). So the commit that deletes
an item states what went, why, what covers the obligation now, and anything knowingly given up. That
is findable from the absence, which is the hard direction:

```sh
git log -S 'carry nothing beyond what it needs to run' -- docs/requirements/
```

**Before writing a tier, check every clause against the record.** For each distinctive phrase in the
authored text, search the commits that produced it. Nothing found means the clause entered without a
decision — it then reviews as ordinary content while encoding a choice nobody made, which is the
defect the pass exists to catch, committed by the pass. This found a whole sentence of a locked `SYS`
item that appeared in none of fifty-three rulings.

A ticket is still where scheduling lives — what is scoped, what is deferred, what a later ticket
owns. It is not where a decision lives.

## Verification before proposing merge

`just verify` green, confirmed in CI rather than locally. Independent review is mandatory when this
session authored the change — see `CLAUDE.md`. Two reviewers with opposed briefs surface more than
one neutral reviewer: on the pass that produced this skill, the adversarial pair caught six lost
obligations, a reversed decision, and three method mismatches that the authoring session had
verified and believed sound.
