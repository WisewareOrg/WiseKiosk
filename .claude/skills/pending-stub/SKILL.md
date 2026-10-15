---
name: pending-stub
description: >-
  Write an inactive `Pending:` item in the WiseKiosk Doorstop tree — a verification obligation whose
  test does not exist yet, or a requirement whose decomposition has not arrived. Carries the stub
  shape and the one route that produces a valid review stamp on an item Doorstop cannot reach.
  Invoke when a requirement needs a child but the code it checks is unwritten, or when stamping an
  `active: false` item. Composes tree-item.
---

# Pending stub

The idiom for an obligation that is real but whose artifact does not exist yet. It is **not**
retirement, which is also `active: false` — a retired item's `text` reads as a record of an
obligation that fell, a pending one describes what is coming.

Mechanics for authoring the file itself are [tree-item](../tree-item/SKILL.md)'s; this is what a stub
adds. Reasoning is
[`docs/requirements/README.md` § Pending verifications](../../../docs/requirements/README.md).

## The shape

- `active: false`, `status: proposed`.
- **`Pending: ` on both the header and the text.** The prefix convention is the `TST` tier's alone —
  a pending `SYS` or `SRS` carries no prefix. Retirement uses `Retired:` in the same place.
- **No `references:` key at all** while pending. It is added at activation.
- The parent link is a normal **stamped** mapping, not `null` — see below.
- The text states what will be asserted, then closes by naming what it lands with:
  *"Lands with the first vertical slice (#12); activated and given a references entry then."*
  Name the ticket, never a bare number alone.

  **That ticket is a choice, not boilerplate.** The example above is a real open issue and it copies
  cleanly into a stub it is wrong for, where it reads as a decision somebody made. Work out which
  ticket writes the artifact this stub waits on and check the issue says so —
  `gh issue view <n> --json title,body` — before writing the sentence. A stub naming the wrong ticket
  is invisible to every gate and is read as fact at activation, which is the point where getting it
  wrong costs the most.

## Stamping one — the whole problem

check-reqs's `check_unreviewed` stage reads inactive items and fails on a missing stamp on the item **or** on its
link. But **neither `doorstop review` nor `doorstop clear` can reach an inactive item**:
`Tree.find_item` is active-only and both answer `no item with UID`. And editing a pending item through
the Python API silently drops `links`, `level`, `reviewed`, `rationale` and both verification
attributes.

So author it **active**, stamp it, and flip it inactive as a text edit:

```sh
D=docs/requirements/.venv/bin/doorstop
# file authored with `active: true`
$D link TST0NN SRS0MM && $D review TST0NN && $D clear TST0NN
# then flip line 1 to `active: false` as plain text
```

The flip is legitimate because **`active` is not among the values the stamp hashes**, so it moves
nothing. Edit the line as text — never round-trip a pending item through the API.

The same applies to *re-*stamping: to touch a stub after the fact, flip it active, run the commands,
flip it back.

## What still bites while it is pending

- **Its parent moving is not free.** Doorstop skips inactive items, but check-reqs's
  `check_suspect_links` stage fails when a pending item's parent moves after the link was reviewed — that is the drift Doorstop
  cannot see. Edit a parent and you owe its pending children a re-stamp.
- **Its own content is nearly free**, since nothing reads it until activation, which is exactly why
  activation is a full re-read rather than a formality.
- **One stub per active parent, not one per test.** An active parent with no child links errors the
  strict gate, so every active requirement needs at least one child even when several would be
  discharged by a single test later. A `TST` item is a verification obligation, not a test function.

## Activation, later

The change that activates a stub re-reads it in full, drops the `Pending:` prefixes, adds the
`references` entry, and stamps it then — never before, never scripted. Dropping the prefix without
re-stamping unreviews its parent's trace.

**The re-read answers three questions, and none of them is answerable by looking at the stub alone.**

1. **Does the text describe the test that exists?** The stub's text is a plan written before the
   code, and by the time the code lands it is usually wrong in some particular — a different staging,
   a narrower scope, an assertion that turned out to be two. Rewrite it to describe what was built.
   This is the reason activation is never scripted: a script can drop a prefix and cannot read a
   test.
2. **Which clause of the parent does that test assert?** Name it — quote it. If no clause can be
   quoted, the item belongs under a different parent, the parent is missing a clause, or the test
   verifies something nobody required. Each of those is a finding rather than a formality.
3. **Does the parent state a figure, and does this check read it?** A number in the parent's text that
   no cited check asserts is the gap
   [`docs/TESTING.md` § Standing obligations](../../../docs/TESTING.md) names.

An item activated without this reads, in the trace, exactly like a verified requirement. That is what
makes it worse than leaving the stub pending: a pending item is visibly incomplete, and this is
invisibly wrong.
