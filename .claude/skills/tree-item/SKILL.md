---
name: tree-item
description: >-
  Add one item to the WiseKiosk Doorstop tree — SYS, SRS or TST — and leave it stamped, linked and
  gate-clean. Carries the create-attributes-link-stamp sequence, the two stamps a new item needs and
  which command writes each, and the per-item checklist a gate will otherwise fail you on. Invoke
  whenever writing a new requirement or verification item, or when `check-unreviewed` fails on
  something just authored. For deciding whether an item earns its place, use tree-pass.
---

# Add a tree item

Conventions are [`docs/requirements/README.md`](../../../docs/requirements/README.md)'s and are
authoritative where this file and it disagree. This skill is the sequence and the checklist; that
document is the reasoning.

Run every command from the silo's venv: `docs/requirements/.venv/bin/doorstop`.

## The sequence

```sh
D=docs/requirements/.venv/bin/doorstop
$D add SRS                 # creates the next SRS0NN.yml at the next level
# …author the file…
$D link SRS0NN SYS0MM      # child first, parent second
$D review SRS0NN           # stamps the item body
$D clear SRS0NN            # stamps the parent link
```

**A new item needs two stamps and `review` writes only one.** After `link` the link reads
`- SYS0MM: null` and `reviewed:` is null; `review` fills `reviewed:` and **leaves the link null**;
`clear` is what writes the parent fingerprint into `links:`. check-reqs's `check_unreviewed` stage fails on
that null, so add-link-review with no `clear` reds the gate on every item it produced.

Their order is immaterial: neither command reads what the other writes, and the two sequences produce
byte-identical files. Re-blessing a child after its parent moved uses the same two commands for a
different act — a human re-reading the child against the parent it has.

**Editing an item after stamping unreviews it** — `rationale`, `verification-method`,
`verification-justification` and the parent UIDs are all inside the fingerprint. Re-run `review`.
`status` is outside it.

**Editing a parent** moves its fingerprint and makes every child's link suspect: `clear` each child.
Re-parenting a child unreviews the child itself, so it needs `review`, not `clear` alone.

## Authoring the file

`doorstop add` picks the id and the level. **Take both** — the level is the next integer in that
document, an item's position in the outline is not what says what it belongs to, and the parent link
is. Quote a two-digit tail (`level: '1.10'`) or YAML collapses it to `1.1`.

Keys are alphabetical. `ref: ''` on every item. `derived: false`. `normative: true` unless the item
obliges nothing, in which case its `verification-method` is empty and it carries no justification.

**`doorstop add` writes `status: proposed`, and the sequence above never changes it.** An item
authored to the commands alone is stamped, linked and un-baselined: `check-arch-trace` refuses a tag
naming a `proposed` item, so the model cannot carry it and the change cannot close. Set
`status: accepted` by hand, in the same edit that writes the content — accepting is the human act of
having read it, which is why no command does it for you. `status` sits outside the review
fingerprint, so setting it moves no stamp and can be done before or after `review`. The exception is
a deliberate stub, which stays `proposed` — see [pending-stub](../pending-stub/SKILL.md).

## Checklist before you commit — each of these is a gate, not a preference

- **`status: accepted`**, unless the item is a deliberate stub. `doorstop add` writes `proposed` and
  no command in the sequence changes it, so this is the one field an author has to remember; a
  `proposed` item cannot be tagged in the model and reds `check-arch-trace`.
- **`verification-justification` on every item.** Required with no exception
  ([ADR 0009 rev 2](../../../docs/decisions/0009-verification-justification-attribute.md)); at `test`
  it says what the check leaves unproven, below `test` what blocks a mechanical check. The single most
  commonly missed attribute, because it is the one an item copied from older content will not have.
- **`rationale` is required at the `SYS` tier**, optional below.
- **`header`**: non-empty, drawn only from `A-Z a-z 0-9` space and `` , . ' ( ) & : ; - ``, and not a
  prefix of any other item's header. The charset is an allowlist, so a `/` or a `|` is rejected — write
  *twelve-hour*, not `12/24-hour`.
- **`text` names no other item.** The obligation reads on its own; an identifier inside it defeats that
  and survives a renumber pointing at whatever now holds the number. Cite freely in `rationale` and
  `verification-justification` instead.
- **A `TST` item's text describes the check that exists, not the one that was planned.** A stub's
  wording is a plan; the activating change re-reads it against the test that landed and rewrites it
  where they differ, and names the parent clause the test asserts —
  [pending-stub](../pending-stub/SKILL.md) § Activation, later carries the three questions. An item
  whose text describes a test nobody wrote passes every gate in this repository.
- **A citation carries the header verbatim in a comment, closed up**:
  `SRS026<!-- The display says when the backend is gone -->`. Every occurrence, not just the first —
  a second bare mention in the same paragraph fails the gate. Reword to avoid the repeat rather than
  restating the comment.
- **`verification-method` is one of `test`, `analysis`, `inspection`, `demonstration`**, and a parent's
  equals the **least decidable** among its children (`test` > `analysis` > `inspection` >
  `demonstration`). Adding one `inspection` child to an all-`test` parent drags the parent down.
- **An active parent needs at least one child link**, or the strict gate errors. An **inactive** child
  satisfies this.
- **`git add` the new file before trusting any local gate.** The doc gates derive their population from
  `git ls-files`, so an unstaged item is invisible to them and a green run measured nothing.

## Then

`just check-reqs` and `just check-citations`. Never bare `doorstop` — validation stamps items that
lack a stamp, so a diagnostic run blesses the very thing in question.

Accepting an item is a human act. An item lands `accepted` + `active: true` + reviewed in the same
change, or it is not baselined; only something awaiting its decomposition or its verifying artifact
stays `active: false` — see [pending-stub](../pending-stub/SKILL.md) for that shape.
