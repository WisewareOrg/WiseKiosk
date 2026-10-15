# Requirements & verification (Doorstop)

WiseKiosk's requirements are tracked as a **Doorstop** tree under this directory — a Python,
Git-native requirements-management tool that gives every requirement a **stable ID**, decomposes
stakeholder needs into testable software requirements and then into verification items, and **fails
CI** when a change leaves the trace stale ([what that comes to](#running-the-gate)). Why this exists
and why Doorstop specifically:
[ADR 0002 rev 4](../decisions/0002-requirements-management-doorstop.md).

## The three documents

A V-model tree — needs on the left, verification on the right:

| Prefix | Document | Holds | Links up to |
|---|---|---|---|
| `SYS` | [`sys/`](sys) | Stakeholder / system-level needs (the validation anchor), framework and per-module alike | — (top) |
| `SRS` | [`srs/`](srs) | Decomposed, testable "shall" statements | `SYS` |
| `TST` | [`tst/`](tst) | One item per test/check; one `references` entry per verifying site, naming the file and — where a runner discovers the test — the `keyword` declaring it | `SRS` |

A module is a need: each carries one `SYS` item for its user-facing want, decomposed into `SRS` items
stating only what is specific to that module. Obligations true of every module stay on the framework
needs and are not restated ([ADR 0012 rev 2](../decisions/0012-module-requirements-in-tree.md)).

Each item is one YAML file named for its ID (`SYS001.yml`) — the prefix plus a zero-padded 3-digit
number. **An ID is permanent**: once assigned it is never reused or renumbered, so external
references to it stay valid.

> **One-time ID reset.** The tree seeded alongside the tooling (the first six `SYS` items and their
> children) was placeholder content and was cleared by the requirements rewrite; IDs were
> re-established from `001` with new meanings. Permanence applies from that reset forward.

## Item attributes

Beyond Doorstop's native fields, every item carries four stored attributes
([ADR 0005 rev 4](../decisions/0005-traceability-gating.md),
[ADR 0009 rev 3](../decisions/0009-verification-justification-attribute.md)):

| Attribute | Values | Meaning |
|---|---|---|
| `status` | `proposed` \| `accepted` | Human review state. `proposed` items live on `main` — the tree is the backlog — but implementing against one is building on unbaselined spec |
| `verification-method` | `test` \| `inspection` \| `analysis` \| `demonstration` | How the requirement is verified; gates route on it |
| `verification-justification` | free text | What the item's verification settles and what it does not. Below `test`, what specifically blocks a mechanically-decidable check; at `test`, what the check leaves unproven. **Required on every item** |
| `rationale` | free text | Why the requirement exists. **Required at the `SYS` tier**, optional below |

**A requirement states the property and names no resources** — which file, endpoint, package or tool
delivers it is not the item's. Nothing decides this mechanically, so it is question 15, *Named
resources*, on [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md)'s review checklist, where every
obligation that leaves no artifact is carried
([ADR 0011 rev 2](../decisions/0011-requirement-or-convention.md)).

**A `TST` item is a verification obligation, not a test function.** It states what must be proven, so
several items may be discharged by one test and one item may need several. Counting `TST` items
against test functions measures nothing.

**`header` is machine-read, so it is constrained.** Prose cites an item by identifier and header
together, the header carried verbatim inside an HTML comment ([`../CI.md`](../CI.md) § Documentation
integrity), so header text is embedded into running Markdown. Every header must therefore be
non-empty, drawn from `A-Z`, `a-z`, `0-9`, the space, and `` , . ' ( ) & : ; - ``, and not a prefix
of another item's header. The permitted set is an allowlist because a list of characters to *reject*
fails open on the one nobody thought of: a header carrying `|` would split a table while one carrying
`-->` would close its own comment, neither visible to any other check. A character absent from the
list is unadmitted rather than forbidden on sight, and admitting one takes a check that it renders as
written both inside a comment and inside a table cell. Prefix-freeness is the reader's constraint
rather than the checker's, which compares a header exactly: two items whose headers begin alike are
told apart only by a reader who reaches the end of both.

An item at `inspection`, `analysis`, or `demonstration` is claiming a human must judge it, and the
justification is that claim's evidence, so a method can never be weakened without saying what blocks
the stronger one. An item at `test` is claiming a machine settles it, and the justification bounds
that claim — a green check that decides set equality against a recorded list has not decided that the
list is right.

`verification-method`, `verification-justification`, and `rationale` are fenced by the review
fingerprint (editing them re-flags review), as are the item's **parent links** — `Item.review()`
stamps the sorted parent UIDs along with the content, so re-parenting an item unreviews it. `status`
is not fenced, because a state transition is not a content change. Verified/implemented is **derived
by tooling from evidence, never stored** (ADR 0005 rev 4).

**An item's `text` names no other item.** The obligation must be readable on its own —
ISO/IEC/IEEE 29148's *complete* and *singular* characteristics — and an identifier inside it defeats
that twice over: the reader cannot tell what is required without a lookup, and a renumber rewrites
`links:` while leaving the sentence pointing at whatever now occupies the number. `rationale` and
`verification-justification` explain the tree to someone reading it as a tree, so they may name items
freely; check-reqs's `check_text_citations` stage fails on `text` only.

### Choosing a verification method

Methods are ordered by **mechanical decidability** — how much of the verification a machine settles,
unattended, on every commit:

> `test` > `analysis` > `inspection` > `demonstration`

This is deliberately **not** the classical V&V rigor ordering, which ranks evidential strength and
places demonstration above inspection. Ranked that way, "move to the stronger method" would push an
item out of continuous integration and onto physical hardware or a wall clock — the opposite of what
this tree is for. `demonstration` sits last because it is the one method that can never gate a merge,
not because a thirty-day soak proves less than reading a file.

Two rules, in order:

1. **An item sits at the most decidable method it honestly supports.** Decidability is what the
   obligation *can* bear, not what has been built yet: an unwritten check is a backlog entry, not a
   reason to claim `inspection`.
2. **A mixed-method item is a defect, not a classification case.** Where an item's text welds a
   mechanically-decidable clause to a residual judgement, split it so each clause sits at its own
   honest method. Averaging the item down hides the decidable half; leaving it at `test` hides the
   judgement half, which is worse — it reads as a machine guarantee nobody has.

It follows that **a parent's method equals the least decidable method among its children**: the
parent's obligation is the conjunction of theirs, so it can be no more decidable than the least, and
claiming less than that understates what the tree already proves. Both directions are defects — a
parent below every child is under-classification, a parent above its least-decidable child is a false
signal. The one exception is a parent legitimately holding a residual obligation no child carries, a
conclusion drawn by comparing the children's bounds against something outside the system: it may sit
below its children, and the `verification-justification` says why.

**For a normative item the four method names are the whole permitted set, and a value outside it is a
defect rather than an exemption.** An unrecognised spelling — a typo, a capitalised token, or the
attribute missing — ranks nowhere in the decidability order, so a rule that merely skipped what it
could not rank would excuse the item from the comparison and report the tree consistent.

**A non-normative item (`normative: false`) obliges nothing**, so it carries an empty
`verification-method` and no justification, and needs no children. It states orientation or scope
whose falsifiable content is owned by other items, and is written in the indicative — a `shall` in a
non-normative item reads as an obligation nothing verifies. This is the one legal empty method; every
normative item has one.

## The V&V model: Doorstop proves linkage, the test suite proves correctness

**Doorstop does not run anything.** It proves that the graph is complete and current:

- **Validation** (completeness) — every `SYS` need has a child `SRS`, every `SRS` has a child `TST`,
  and no `SRS`/`TST` is orphaned without a parent. A gap fails the gate.
- **Verification linkage** — every active `TST` item's `references` must resolve: each entry to a
  real file in the repo, and, where the entry carries a `keyword`, to the line that keyword names.
  **The keyword is the test's declaration, not its name** — `func TestFoo` for Go, `test('a title'`
  or `` test(`a ${templated} title` `` for Playwright — so what resolves is the line that declares
  the test, and renaming it fails the gate as surely as a missing file does
  ([ADR 0005 rev 4](../decisions/0005-traceability-gating.md)). A bare name would not: Doorstop
  returns the *first* line matching the keyword anywhere in the file, and this repo's Go doc-comment
  convention repeats the function name in prose above it, so a bare name resolves to the comment and
  survives the test's deletion. **A parameterized test's cases are the limit of that**: where the
  `test(` call sits in a loop, the keyword is the template rather than any title the runner prints,
  so removing a row of the data array — or emptying it — leaves the entry resolving over fewer
  tests, or none. Evidence drift below is what catches that. **A reference to a whole-file check
  artifact no runner discovers omits `keyword`** — a `scripts/image/*.py` harness has no declaration
  to anchor on and no runner title to name, so the entry resolves to the file alone and drift is
  carried by its `sha`. Every reference to a runner-discovered test carries a keyword. No test file
  names a requirement ID: the trace runs one way, from the tree outward.
- **Evidence drift** — the `TST` document records a hash of each referenced file when the item is
  reviewed, and `tst/.req_sha_item_validator.py` compares it back on every validation run. Editing a
  referenced test — a comment counts — fails the gate for every item referencing it until
  `doorstop review <UID>` is run against each. That cost is deliberate and is
  [ADR 0005 rev 4](../decisions/0005-traceability-gating.md)'s.
- **Re-validation** — editing a parent item changes its fingerprint, flagging every child **suspect**
  until re-reviewed, and moving a child to a different parent unreviews the child. Among **active**
  items a silent divergence is impossible; an inactive item is evaluated for neither, and what is and
  is not covered there is [below](#pending-decomposition).

What Doorstop does **not** do is prove the referenced check passes: it proves a `TST` item *points at*
a real verifying file, and the corresponding [`just`](../../justfile) gate proves that check passes.
Both are required.

### Pending verifications

Where a requirement's verifying test does not exist yet, its `TST` item is committed **`active: false`** with a note describing the test to come. Inactive items are
excluded from reference and review checking; each is activated and given a real `references` entry as
its test lands. A `SRS` parent whose child is pending surfaces an informational `no item with UID:
TST00x` line during validation — Doorstop noting a stubbed verification, not a failure.

### Pending decomposition

The same idiom extends upward: a `SYS` or `SRS` item whose decomposition round has not yet arrived is
committed **`active: false`** with its full content and attributes. The strict gate errors on any *active* parent with no child links, so an item is
activated in the same change that writes its first child. Elsewhere `active: false` means retirement
(ADR 0005 rev 4); an item carrying a pending note is awaiting decomposition or its verifying
artifact, not retired.

### Retirement

A retired item keeps its ID and its file — an ID is permanent, and the item is the record of an
obligation that existed. It is `active: false`, its `rationale` states the ground on which the
obligation fell and names the ticket that withdrew it, and its `text` is rewritten to read as a
record rather than a demand. A `TST` item marks it in the header as `Retired:`, where a pending one
carries `Pending:`; a `SYS` or `SRS` item carries no prefix, that convention being the `TST` tier's
alone.

An **active** child left linked to a retired parent fails the gate: Doorstop reports `no item with
UID` for the parent it can no longer resolve, and exits non-zero whether or not `--error-all` is
passed. An unresolvable *child* reference from an active parent is only a warning, and the asymmetry
is the point — a missing parent breaks the trace, a missing child is a decomposition not yet written.
An **inactive** child is invisible to it, and that is the case needing the discipline: a child of a
retired item is retired in the same change or re-parented in it, whichever its own content supports.

Rewriting the parent's `text` and `rationale` is what moves its fingerprint, and every link stamped
against it goes suspect. Flipping `active` moves nothing: it is not among the values the stamp hashes.

**Activation is a review act.** Doorstop skips inactive items entirely, so a `reviewed` stamp on a
pending item carries no authority and edits to a pending item's own text are invisible to the gate.
One half of that is mechanical: check-reqs's `check_suspect_links` stage fails when a pending item's *parent* moves
after the link was reviewed, which is the drift Doorstop cannot see. The rest lands at activation —
the change that activates an item re-reads it in full and stamps its review (`doorstop review <UID>`)
then, never before, never scripted.

## Running the gate

Requires a local venv — siloed here beside the requirements it serves — with the pinned tool
([`pyproject.toml`](pyproject.toml), wise-ci's `check-reqs` at a pinned commit). Create it from the
repo root:

```sh
uv sync --project docs/requirements
```

Then:

```sh
just check-reqs   # wise-ci's check-reqs, running its seven stages in their fixed order
just verify       # runs check-reqs alongside the other repo gates
```

`just check-reqs` is the **same** invocation CI makes (job `requirements` in
[`checks.yml`](../../.github/workflows/checks.yml) runs the recipe itself), so the local run and the
gate are one spelling — a command added to the recipe is a command CI runs, with no second text to
fall behind. check-reqs's `doorstop` stage runs `doorstop --error-all --no-reformat`, which
promotes Doorstop's suspect / unreviewed / orphan / unresolved-reference warnings to errors, so the
process exits non-zero and the gate actually blocks — plain `doorstop` only warns.

**The `check_unreviewed` stage runs first because Doorstop writes.** Validating the tree stamps a
review fingerprint into any item that has none, and into any link carrying no stamp — whether or not a
person looked, and `--no-reformat` does not prevent it. An item authored in one commit would
otherwise be "reviewed" by whoever next ran the gate, which is the one thing the fingerprint exists
to prove. Failing first stops Doorstop before it can stamp. Clear it with `doorstop review <uid>`,
deliberately, never by re-running the gate.

Between them these commands assert that:

- no item carries a review fingerprint nobody wrote;
- every document the tree holds yields at least one item to check — a tier that yields nothing
  produces no finding, which reads exactly like a tier whose every item is reviewed;
- every parent link resolves and no item is orphaned or left suspect, **inactive items included**;
- every active `TST` item's `references` resolve — each entry to a real file, and, where it carries
  one, to the line its `keyword` names — and no referenced file has changed since the item was
  reviewed;
- every item carries a `verification-justification`;
- no item claims a verification method its own children do not support;
- no item's `text` names another item.

Each is a property of the specification, which is why they are stated here rather than in
[`../CI.md`](../CI.md) with the repository's checks — that document says they run and block, this one
says what they mean.

check-reqs's last stage is a report rather than a check: `report_proposed` prints the
`proposed` backlog per tier and exits zero whatever it counts. Making the backlog visible on a green
run is a property of the process rather than of the specification, so what that output asserts is
stated in [`../CI.md`](../CI.md) § Gate wiring.

The browsable, click-through traceability view of this tree (needtables, link graphs, matrices) is
built by the documentation site silo, [`../site/README.md`](../site/README.md) (ADR 0004 rev 2); this
directory is the requirements' canonical source and gate, not its presentation.

## Adding or changing requirements

Run all commands with the venv (`docs/requirements/.venv/bin/doorstop …`).

| Task | Command | The rule that comes with it |
|---|---|---|
| Add an item | `doorstop add SRS` | Creates the next `SRS0NN.yml`. Edit its `text` to a single "shall" statement; write a `header` summarising it |
| Link it up | `doorstop link SRS0NN SYS0MM` (child first, parent second) | Every `SRS` needs a `SYS` parent and every `TST` a `SRS` parent, or the gate flags an orphan. The link is written **unstamped** — see the next row |
| Stamp a new item | `doorstop review <UID>` **and** `doorstop clear <UID>` | Both, because they write different fields: `review` stamps the item body into `reviewed:`, `clear` writes the parent fingerprint into `links:`. `review` alone leaves the link reading `- SYS0MM: null` and check-reqs's `check_unreviewed` stage fails on that null, so an item added, linked and reviewed but never cleared reds the gate. Their order is immaterial — neither reads what the other writes, and the two sequences produce byte-identical files. The re-blessing row below is not a different order but a different obligation: a human re-reading a child after its parent moved |
| Point a `TST` at its check | add one `references` entry per verifying site: `{path: <repo-relative-file>, type: file, keyword: <the test's declaration>}`, then `doorstop review <UID>` to stamp the file hash | The path must resolve to a real tracked file and the keyword to the line declaring the test — `func TestXxx`, `test('<title>'` — never the bare name, which also matches the doc comment above it. A whole-file check artifact no runner discovers omits `keyword`, per ADR 0005 rev 4. **Doorstop cannot reference a file under a dot-directory** (e.g. anything in `.github/`) — see ADR 0002 rev 4; cite such wiring in the item's `text` instead |
| Re-bless a child after editing its parent | `doorstop clear <UID>`, then `doorstop review <UID>` | `clear` updates the stored parent fingerprint in the child's `links:`; `review` alone re-stamps the item but leaves the link suspect. Re-blessing is the human act of re-reading a downstream item after its parent moved — do not script it blindly |
| Re-bless an item after moving it to a different parent | `doorstop review <UID>` | `clear` is not enough: the item's parent UIDs are inside its own stamp, so the item itself is unreviewed. Read it against the parent it now has. `--error-all` reports it as `unreviewed changes` until you do |
| Baseline a round | — | Its `SYS`/`SRS` items land `accepted` + `active: true`, reviewed in the same change; only items awaiting decomposition or a verifying artifact stay `active: false` (see *Pending* above). "Active" and "reviewed" arrive together — an active-but-unreviewed item fails `--error-all` — so a domain's requirements are never left `proposed`/inactive, which would leave them un-baselined and outside the gate |

**One exception to re-blessing by hand, and it carries its own burden of proof: a bulk edit provable
as a single transform.** Where every changed item differs only by one mechanical substitution — a
corpus-wide rewrite of prose that cites other items, say — re-reading forty items decides nothing a
machine has not already settled, and scripting the re-stamp is legitimate. What makes it legitimate
is the proof, not the claim: apply the transform to the previous revision and diff the result
byte-for-byte against the new one. Every file must match, or the edit was not the single transform
you thought it was and the exception does not apply. A word-level or token-level comparison is not
enough — it cannot see a reflow, and reflow is how an unintended change hides.

### Traps

- **Neither `review` nor `clear` can reach an inactive item.** `Tree.find_item` is active-only, so
  `doorstop review TST019`<!-- Pending: no identity-based rejection, in the contract or on the wire -->
  and
  `doorstop clear TST019`<!-- Pending: no identity-based rejection, in the contract or on the wire -->
  both answer `no item with UID` while that item is pending. Stamping one takes a loop over
  `document._iter()`, which is the only route where a pending item's link goes suspect before
  activation.
- **`Item.clear()` writes a null stamp when the *parent* is also inactive**, which unstamps the link
  rather than re-stamping it — `_get_parent_uid_and_item` resolves through the active-only lookup and
  yields an unknown item, whose stamp is empty. The loop above reaches the child but not the parent,
  so assign `link.stamp = parent.stamp()` from the item found through `document._iter()` and save.
  check-reqs's `check_unreviewed` stage catches the null, so this fails loudly rather than silently; retiring a
  parent whose child is pending is when it arises.
- **A `keyword` that fails to resolve is reported by its `path`.** Doorstop builds
  `external reference not found: <path>` from the entry's path whichever half failed, so a deleted or
  renamed test reads as a missing file that is sitting right there. Check the keyword before the path.
- **Validation stamps what it touches, so never run it on the tree you care about.** `doorstop`'s
  validation pass re-blesses items as it goes, so a diagnostic run silently re-stamps the very items
  whose review state was in question. Copy the tree to a throwaway directory and validate there.
- **Inactive items are not rewritten by Doorstop**, so write their parent links in dict form,
  `- UID: null` (the form Doorstop itself stamps); a plain-string link breaks the docs-site needs
  generator.
- **Quote two-digit levels** (`level: '1.10'`) — unquoted, YAML parses a float and collapses it to
  `1.1`.
- **A document can never be empty** — "no items" is a gate error, so a tree reset must land in the
  same change as its first new items.
- **A new normative clause can contradict a decision already recorded in an existing item's
  `rationale`** — a deliberately-excluded case, a boundary an owner already fixed. `--error-all`
  cannot see this; grep the relevant items' rationales first. A clause that reverses a recorded
  decision is a finding: either the decision is reopened with its own review, or the clause does not
  land. (A phone-width `shall` slipped this way in the #38 round, against a scope decision recorded
  in `SYS002`<!-- The display's rendering keeps nothing from a viewer --> — caught only by
  independent review.)
- **Traceability is item-level; individual clauses are not checked.** An item that links two parents
  satisfies the orphan gate at item granularity, yet a single clause inside its `text` can be
  supported by *neither* parent — an orphan the gate cannot see. Prefer one obligation per parent;
  when an item must span parents, confirm by hand that every clause traces to one of them.
