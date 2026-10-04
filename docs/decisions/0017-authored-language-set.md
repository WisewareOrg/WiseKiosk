# 0017 — Authored language follows the artifact's audience: Go and TypeScript ship, Python checks the repository

**Status:** accepted
**Decided:** 2026-08-16 (CSS's product-stylesheet disposition added; the audience rule itself taken
2026-08-04 in #60 authored-language set, taken after
[ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md) settled which authored checks survive at all)
**Rev:** 9

## Revisions

- **rev 9** — 2026-09-13 — `scripts/check-languages.py` declares `svg` an asset a build serves rather
  than an authored program: the park-wait-times module's icon set
  (`frontend/src/modules/park_wait_times/icons/`) is original, stylised line-glyph artwork, the same
  kind of shipped, hand-drawn visual asset `woff2` and `gif` already carry a disposition for here,
  rather than program text a comment-discipline gate would read. The audience rule, the derivation
  rule and the authored language set are unchanged, so the `Decided` date does not move (#309
  wait-times implementation).
- **rev 8** — 2026-08-18 — the carve-out for what a build serves is stated over *a* build rather
  than a documentation build, and `scripts/check-languages.py` declares `woff2` under it: the
  frontend skeleton bundles the Inter variable font [the display styling
  contract](../contracts/display-styling-contract.md) states, which is a third-party binary nobody
  here authors and which no existing kind reached. `html` gains a second disposition beside its
  docs-site one, `frontend/index.html` being Vite's own required entry format rather than a
  documentation asset — the two-dispositions-under-one-extension shape `.css` already carries. The
  same change discharges `scripts/validate-tree.sh`'s disposition — the wrapper is deleted and sh
  now authors nothing here — so the paragraphs describing that deletion as pending are rewritten to
  record that it landed. The audience rule, the derivation rule and the authored language set are
  unchanged, so the `Decided` date does not move (#10 frontend skeleton).
- **rev 7** — 2026-08-17 — the Go toolchain's manifest and lockfile formats join the Decision's
  illustrative list of derived formats, and `scripts/check-languages.py` declares `mod` and `sum`
  beside them: the boundary codegen's Go module root introduces both, and an entry added to the
  declared set arrives with the rev that decided it. The audience rule, the derivation rule and the
  language set itself are unchanged, so the `Decided` date does not move (#7 boundary-contract
  codegen).
- **rev 6** — 2026-08-16 — `.css` gains a second disposition alongside the existing docs-site Furo
  override: [the display styling contract](../contracts/display-styling-contract.md) introduces
  `frontend/src/app.css` as a shared product stylesheet, which is authored CSS for what ships rather
  than a documentation asset. The Decision table's first row gains CSS beside Go, TypeScript and the
  Svelte component format; `scripts/check-languages.py`'s `.css` entry is updated to name both
  dispositions. This changes what the table's first row names, so the `Decided` date moves with it;
  the rest of the language set, the audience rule and the derivation rule are unchanged (#154 display
  design).
- **rev 5** — 2026-08-13 — `scripts/check-languages.py` gates the declared extension set, which
  falsifies two claims rev 4 made: *"Nothing here is gated"*, and the rejected alternative *Gate the
  set with a check* refusing one on the ground that "one residual obligation does not earn a gate of
  its own". That alternative keeps its reasoning and gains the condition that changed. What the gate
  does **not** reach is stated rather than assumed — the audience binding, content behind a declared
  path, a program embedded in a derived format, and a declared entry added without a rev — and review
  checklist item 12 narrows to exactly that. The disposition list here is still a snapshot: the
  check holds a second copy and nothing compares the two. The language set, the audience rule and the
  derivation rule are unchanged (#145 prose pass).
- **rev 4** — 2026-08-13 — three claims the tree contradicted: a count of Python check scripts and the
  *reads-the-tree* split drawn from it, a line count for the one sh file, and a closed ticket carrying
  the argument that this disposition ends rather than grandfathers. Each replaced by the rule it was
  an instance of; the language set, the derivation rule and every rejected alternative are unchanged
  (#145 prose pass).
- **rev 3** — 2026-08-08 — drops the append-only placement constraint on the review checklist: a
  citation carries the question's name beside its number, so a renumber no longer strands one. The
  language set is unchanged (#129 retire the desk configuration validator).
- **rev 2** — 2026-08-05 — names revving rather than amendment as the route a later decision takes; the language set is unchanged (#118 ADR revisions).
- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

The product's languages were decided. Go is [ADR 0001 rev 1](0001-backend-language-go.md); the frontend is
[ADR 0018 rev 1](0018-frontend-svelte-vite-static-spa.md); the boundary types are generated into both by
[ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md). The languages the *repository* is written in were
never decided at all. Programs are authored here in POSIX sh, in Node ESM and in Python, none of
those by anyone's choice, and the next contributor could add a fourth without meeting an argument.

Two things make an unbounded set expensive rather than merely untidy.

**Each language is a standing obligation, not a one-off.** The comment-discipline gate `CI.md`
describes (#59 comment-discipline gate) decides which comments belong to a language's documentation
facility, which costs a classifier arm and a fixture suite per language, and its coverage registry
fails on a language that has neither. A set nobody bounded is an obligation nobody can size.

**One of the candidates is already owed and the other is not.** Python is unconditional here for
reasons that have nothing to do with checks: Doorstop ([ADR 0002 rev 4](0002-requirements-management-doorstop.md))
and Sphinx ([ADR 0004 rev 2](0004-docs-site-sphinx-needs.md)) both require it, and CI installs it for two jobs
whatever the check scripts are written in. Nothing obliges Node in the repository-checks layer at
all — all seven `.mjs` scripts import only `node:fs`, `node:child_process` and `node:path`, and five
carry a header comment recording that they deliberately scan plain text rather than reach for a
parser, three of them naming the YAML parser they avoid. Consolidating the checks on Python therefore
adds no toolchain and takes Node out of that layer; consolidating them on Node would keep both there
indefinitely. Node does not leave the repository either way — LikeC4
([ADR 0003 rev 4](0003-architecture-as-code-likec4.md)) and Vite
([ADR 0018 rev 1](0018-frontend-svelte-vite-static-spa.md)) make it unconditional as an invoked toolchain. The
asymmetry is about which layer owes which interpreter, not about which language is present.

Symmetrically, a Python check that reads the requirements tree imports `doorstop` or `yaml` and so
pays no interpreter this repository was not already installing. The Node dependency is habit; the
Python one is the Doorstop silo. A Python check that reads something else — the decisions directory,
say — imports neither and still pays nothing, which is the asymmetry rather than an exception to it.

## Decision

**Language follows the artifact's audience.** What this decides is the languages a contributor
authors **programs** in.

| Artifact | Authored in |
|---|---|
| What ships to a user or an operator — the kiosk, and the tools an operator runs | Go, TypeScript, the Svelte component format ([ADR 0018 rev 1](0018-frontend-svelte-vite-static-spa.md)), and CSS for the shared product stylesheet ([the display styling contract](../contracts/display-styling-contract.md)) |
| What checks this repository | Python, standard library only |

**CSS carries a second, unrelated disposition beside the row above: a Furo theme override that is
documentation rather than a program**, an asset the Sphinx docs-site build serves and outside this
decision's reach on that ground alone
([`../../scripts/check-languages.py`](../../scripts/check-languages.py) records both dispositions
under the one extension). A product stylesheet is different: it ships to an operator, styling what
they see, and is authored the same way the table's other product formats are.

**Everything else present in the tree is derived, not enumerated.** A toolchain's own required input
format — its configuration, its model, the items it stores — is part of invoking that toolchain
rather than authoring in it. The workflow YAML, the Doorstop item files under `docs/requirements/`,
the `.likec4` models, a `lychee` TOML, a `commitlint` configuration, the Go toolchain's `go.mod` and
`go.sum`, and the Dockerfile a container build will need are each that, and so is
`docs/site/conf.py`, which is Python because Sphinx's configuration format is Python. The
`justfile` is the same thing for `just`, which is why [`../CI.md`](../CI.md)'s no-shebang rule
matters here: it is what keeps a recipe a list of commands rather than a shell script wearing a
recipe's clothes.

**A program embedded in a derived format is still an authored program**, and the rule above reaches
it: a workflow `run:` block or a hook `entry:` carrying control flow is authored sh whatever file it
sits in. Only the `justfile` leg of this has a check behind it; an embedded program is one of the
things the gate below cannot see, and so the reviewer's, under
[`../../CONTRIBUTING.md`](../../CONTRIBUTING.md) review checklist item 12, *Languages*. It is the one
place where the argument that a new language arrives as a new file extension, the loudest thing in a
diff, does not hold at all.

Nothing above needs revising when a tool arrives with a format not yet seen here — which is the
point, since [ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md) adopts four such tools.

**Documentation, and the assets a build serves, are not authored programs and this decision does not
reach them.** Which build is not the distinguishing fact — a rendered figure the docs site serves and
a bundled font the product serves are both third-party or generated material that nobody here
authors, and a carve-out naming only the documentation build would have refused the second for a
reason that never applied to it.

**Node is an invoked toolchain and never an authoring language.** LikeC4
([ADR 0003 rev 4](0003-architecture-as-code-likec4.md)) and Vite are invoked; so is whatever an adopted hook
provisions for itself. The distinction is who wrote the code being run.

**POSIX sh authors nothing.** The rule that admitted it — sh only where no interpreter can be
assumed — had `.githooks/` as its entire population, and
[ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md) retires those for `pre-commit`, which is
itself Python. A rule with no subject is not kept for the one file that predates it.

**TypeScript is product-only.** Extending it to repository tooling would leave the runtime, the
toolchain and the classifier arm exactly as they are, so the exclusion of JavaScript would become a
rule about file extensions.

**Scripts that read the requirements tree run in the Doorstop silo venv and may use its libraries.**
That is [`../CI.md`](../CI.md)'s tooling-siloed-with-the-feature rule reaching this decision, not a
carve-out from it: the silo is what makes `doorstop` and `yaml` available, and it is why a check that
reads the tree is a different artifact from one that reads the repository.

**The artifacts that did not conform when this was decided each carry a disposition rather than an
exemption:** `scripts/check-branch.sh` converts (#109 check-branch conversion); the four index, silo
and splice checks in Node convert (#110 Node check conversion); `scripts/check-verify-ci-parity.mjs`
waits on #101 CI invoking just recipes directly, because that ticket may delete it (#111 parity-check
conversion); and `scripts/validate-tree.sh` was **deleted rather than converted**, retiring the
pending-TST-tier exception, which dropped the wrapper and restored the bare `doorstop` call it stood
in for. Tree validation was not what went; the exception was.

**That deletion was conditional, and the condition is discharged.** The wrapper was to go when the
first `TST` item activated and the exception it stood in for stopped being needed — which happened
under #10 frontend skeleton, in the same change that activated the first seven verification items, so
sh now authors nothing in this tree and `LEGACY` in
[`../../scripts/check-languages.py`](../../scripts/check-languages.py) is empty. Until then sh
remained an authored language here for exactly one file — a classifier arm and its fixtures, which is
the price the grandfathering alternative was rejected for paying. The disposition was deletion rather
than conversion because converting a script another change deletes is the waste this decision's own
sequencing avoids elsewhere. The difference from grandfathering is that it ended on a condition the
tree decided rather than on anyone's intention.

That list is a snapshot taken on the decision date, not a standing inventory — no check compares it
against the tree, and the rule above is what governs anything written after it.
[`../../scripts/check-languages.py`](../../scripts/check-languages.py) keeps a second copy of it, of
the files above written in a language that authors nothing plus the ones
[ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md) disposes of in the same languages —
the remainder of that record's list carries a declared extension and needs no entry. That copy is
grandfathered by exact path and fails once a path stops being tracked, so a disposition that lands
takes its grant with it. **The two copies are not compared with each other**, so a conversion drops
its entry from the check's list and leaves this one saying that conversion is pending.

**The declared kinds are gated; the audience binding is not.**
[`../../scripts/check-languages.py`](../../scripts/check-languages.py) fails any tracked file whose
extension is outside the declared set, or whose exact path is undeclared where it has no extension,
and fails a run resolving no tracked file rather than reporting a clean tree. What it holds is that
*set*, tree-wide. It judges a file by path and extension and never opens one, so the table above —
which binds a language to an audience — is outside it: a repository check authored in TypeScript
satisfies the gate on an extension this record declares for what ships, and TypeScript being
product-only is not a claim any run makes. Outside it too are the content behind a declared path, a
program embedded in a derived format, and whether an entry added to the declared set — or to the
grandfather list beside it, which needs no new extension and is the cheaper of the two — arrived with
the rev that decided it. A language outside this set is still a decision with a rejected alternative, so it
arrives as a rev of this record or an ADR superseding it, and the reviewer is the mechanism for
everything the gate leaves —
[`../../CONTRIBUTING.md`](../../CONTRIBUTING.md) review checklist item 12, *Languages*, added with this record.
#59 comment-discipline gate's coverage registry is **not** this decision's enforcement: it exists so
that gate cannot go silently blind on a language it has no arm for, and it would fail-closed on an
unregistered language whatever this record said.

## Alternatives considered

- **Leave it unbounded — whatever language suits the job.** The status quo, and its argument is real:
  every script here is short, each was written in whatever fit, and no defect has been traced to the
  spread. Rejected because the cost is not in the writing. It is one classifier arm and fixture suite
  per language for #59 comment-discipline gate, a toolchain per language in CI and in any hook, and a
  reader who must know four languages to review a check. Those are paid per language, forever, by
  whoever comes next.
- **Bound the set but grandfather the existing sh and Node scripts.** The honest-sounding compromise,
  and the one the ticket expected: they work, they have recorded cases, and rewriting working gates
  to satisfy a policy is churn. Rejected on arithmetic. #59's registry resolves per *tracked file*, so
  a single grandfathered `.sh` still buys sh a classifier arm and its fixtures — the language's full
  price with none of the bound. Grandfathering saves the rewrite and forfeits the reason for it.
- **A carve-out for `scripts/splice-arch-diagrams.mjs`**, which runs only inside `check-arch` and so
  is siloed with the LikeC4 toolchain it serves — the same reasoning that admits the Doorstop venv
  above. Rejected by the arithmetic in the line above: it saves 79 lines of conversion and gives up
  the bound, because the language stays in the registry either way. The Doorstop exception survives
  the same test only because it is about *libraries a silo provides*, not about a second authoring
  language.
- **Enumerate every permitted format instead of deriving them.** Rejected: an enumeration would need
  revising the first time an adopted tool brings a configuration format, and
  [ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md) adopts four tools that bring TOML and a
  JavaScript-or-JSON-or-YAML configuration between them. An ADR revved one ticket after it is written
  was written too early.
- **Gate the set with a check.** Machine-decidable, and [ADR 0011 rev 2](0011-requirement-or-convention.md)
  routes a machine-decidable convention to a check — which is why this was the drafted answer.
  Rejected: a new authored language usually arrives as a new file extension in a diff's file list,
  which is the loudest thing a review sees — the embedded case named in the Decision above is the
  exception, and it is a residue rather than the population — and
  [ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md) had just ruled that one residual obligation
  does not earn a gate of its own. The two work in sequence rather than in parallel — the first sizes
  the case as a residue, which is the condition under which the second reaches it at all. The
  checklist question is what 0011 requires so the obligation is not a dead letter; a gate on top of it
  would be ceremony.
  **Adopted at rev 5**, for less than it was refused on. What the arithmetic priced was a gate bought
  against one residual obligation; built, it also makes the declared set an artifact in the tree, each
  entry naming what it serves and which record grants it, and gives the grandfather list a
  fails-when-stale property no reviewer reading a diff can supply. It adds no toolchain this
  repository was not already installing. The rejection's own reasoning survives adoption rather than
  being overturned by it — a new extension is the loud case, and gating it is what leaves review a
  residue small enough to state, which is why item 12 narrows rather than retires.
- **Keep every product style rule inside a `.svelte` `<style>` block, authoring no standalone
  `.css`.** [the display styling contract](../contracts/display-styling-contract.md) had this as a live
  option — CSS inside a component's `<style>` block, root-component globals included, was already
  sanctioned without touching this record at all. Rejected there in favour of a shared token
  stylesheet; the reasoning for that choice is that record's, not restated here. What follows for
  this record is only the consequence: had the alternative been chosen, CSS would need no new
  disposition and this rev would not exist.

## Consequences

- **822 lines of working check convert** across #109, #110 and #111 — 145 in sh, 677 in Node. Each is
  verified by re-running the cases [`../../scripts/README.md`](../../scripts/README.md) records for
  the original, in both directions, because a conversion that is verified by inspection is a rewrite
  with a clean-looking diff.
- **`check-branch` loses `curl` and `jq` for `urllib` and `json`.** [ADR 0006 rev 6](0006-process-gates.md)'s
  *plain sh + curl + jq — no toolchain* property, already corrected by
  [ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md) when `commitlint` and `pre-commit` were
  adopted, ends completely. What replaces it is a weaker but real property: the interpreter the gates
  need is one this repository already owes for its requirements tree and its documentation site.
- **A check that wants logic already written in TypeScript must reimplement it in Python or shell out
  to the frontend toolchain.** No check does; a configuration-schema check that wanted to
  reuse the validation engine of [ADR 0007 rev 2](0007-config-validation-allocation.md) would be the first, and
  it would be a real cost when it arrives.
- **#59 comment-discipline gate's registry gets a bounded population** — the authored languages above,
  plus formats that arrive derived rather than chosen, each of which needs an arm or a recorded
  exclusion under that gate's own rule. Bounded is the claim, not small: what the bound buys is that
  the population changes only by an ADR.
- **No requirement item states any of this.** The decision constrains the repository, and
  [ADR 0011 rev 2](0011-requirement-or-convention.md) makes a repository constraint a check or a checklist
  question, never a tree item.
- **The review checklist gains a question, *Languages***, appended to its *Code* section rather than
  placed beside the dependency question it most resembles. Inserting it there would have renumbered
  the questions below it, against [ADR 0002 rev 4](0002-requirements-management-doorstop.md),
  [ADR 0003 rev 4](0003-architecture-as-code-likec4.md) and
  [ADR 0016 rev 12](0016-maintained-tools-for-standard-artifacts.md), which cite one by number with
  nothing gating the citation. That constraint does not bind a later change:
  [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md) requires a citation to carry the question's name
  beside its number, so a renumbered citation still names what it meant.
- **`.css` now carries two dispositions under one declared extension.**
  [`../../scripts/check-languages.py`](../../scripts/check-languages.py) judges by extension alone
  and never opens a file, so it cannot tell a docs-site override from a product stylesheet apart; its
  one `css` entry names both rather than splitting the extension, the same shape the audience binding
  already takes for every other row in the Decision table above.

**Premise that would reopen this:** an artifact appears that neither audience covers — something that
must run where neither Python nor a shipped toolchain is reachable — or Python stops being
unconditional here, which means both Doorstop ([ADR 0002 rev 4](0002-requirements-management-doorstop.md)) and
Sphinx ([ADR 0004 rev 2](0004-docs-site-sphinx-needs.md)) leaving. A single script that would have been shorter
in another language is not that premise; it is the argument this decision rejected. Absent either, do
not relitigate.
