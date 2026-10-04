# `check-languages.py`

The inputs this check has been run against, in both directions. What it *asserts*, and why, is
[ADR 0017 rev 9](../../docs/decisions/0017-authored-language-set.md)'s;
how to run a case is [`../README.md`](../README.md)'s.

Exercised at `57fdd4e`, script md5 `cdf16aa8fefac64effc439dc29be1db9`.

Re-exercised under #10 frontend skeleton, script md5 `494d26dbfde1c1008de882ab11275f9b`, when
ADR 0017 rev 9 declared `woff2` and gave `html` a second disposition: every must-fail row below was
re-run against the widened set, and the row naming a second font extension was added there. A passing
run over this branch's head reports **240 tracked files**.

Re-exercised under #54 container image and publish, script md5 `d44a6cc60719298083fbeb2b6d455d40`,
when the container build's two extensionless inputs — `Dockerfile` and `.dockerignore` — entered
`NO_EXTENSION`: every must-fail row below was re-run against the widened set, and the row naming a
`Dockerfile` at an undeclared path was added there, the `.dockerignore` half of the same claim run
the same way. A passing run over this branch's head reports **315 tracked files**.

Re-exercised under #220 weather module, script md5 `b4d8923e913b7f02fa9f36341bedad70`, when a second
`.woff2` — the icon face — joined the bundled Inter one, and a `.txt` carrying the licence and
attribution both faces are redistributed under joined them in the same directory. **The declared set
did not widen**: `woff2` and `txt` were both already declared, so each is the *"new file with an
already-declared extension"* row below rather than a new claim, and no must-fail row's reasoning
moved. What changed in the script is those two extensions' justification prose — `woff2`'s named
Inter alone, and `txt`'s carried no disposition for legal text. Run in both directions over this
branch's head: the tree as it stands passes with both faces and the licence file tracked, and the
three rows those files are the input to — a seeded `scripts/rogue.rs`, a
`frontend/src/assets/rogue.ttf` copied from the face, and a `frontend/src/assets/rogue.license`
copied from the licence file — are each still rejected by name. The other must-fail rows were
**not** re-run — nothing they turn on moved.

Re-exercised under #309 wait-times implementation, script md5 `20977e05ab1ca23e9f9aeefb1f7f7b49`,
when ADR 0017 rev 9 declared `svg` an asset a build serves: every must-fail row below was re-run
against the widened set, and the row naming an undeclared `svg` extension turned into a must-pass
for the park-wait-times module's own icon set. A passing run over this branch's head reports **497
tracked files**.

| Direction | Case | Input |
|---|---|---|
| Must fail | A tracked file carries an extension outside the declared set | `scripts/rogue.rs` added to the tree |
| Must fail | A tracked file carries a *second* extension of a kind already declared | `frontend/src/assets/rogue.ttf`, a copy of the bundled font, added beside it — declaring `woff2` admits that extension and not the class |
| Must fail | A declared extension spelled in a different case | `scripts/rogue.PY`, a copy of a real check, added beside it |
| Must fail | A file with no extension and no declared entry for its exact path | `scripts/rogue-noext` added to the tree |
| Must fail | A declared no-extension basename reused at a path that is not declared | `scripts/decoy/LICENSE`, a copy of the real `LICENSE`, added under a subdirectory |
| Must fail | The container build's `Dockerfile` at a path other than the declared one | `frontend/Dockerfile`, a copy of the root `Dockerfile`, added beside the frontend package root — declaring the root path admits that path and not the basename |
| Must fail | A **new** `.sh` file, the language ADR 0017 rev 9 says authors nothing | `scripts/rogue.sh` added to the tree |
| Must fail | A **new** `.mjs` file, same reasoning | `scripts/rogue.mjs` added to the tree |
| Must fail | A `LEGACY`-grandfathered file deleted, leaving its entry stale | `scripts/check-eol.sh` removed from the tree, the entry left in `LEGACY` |
| Must fail | The population is empty | a freshly `git init`'d repository with nothing added, so `git ls-files` resolves nothing |
| Must fail | An untracked file, whatever its kind | `scripts/rogue.rs` present but never added — the untracked guard names it as unjudgeable |
| Must fail | An untracked file of a declared kind | an untracked, well-formed `.md` — visibility, not the declared set, is what fails |
| Must pass | The tree as it stands | — |
| Must pass | A new file with an already-declared extension | `scripts/new-check.py`, a placeholder script, added to the tree |
| Must pass | A rendered PNG figure a Markdown document embeds | `docs/design/display-design-study.md`'s two `.png` figures, in the tree as it stands |
| Must pass | The park-wait-times module's own icon set, `svg` among the declared extensions | the six `.svg` files under `frontend/src/modules/park_wait_times/icons/`, in the tree as it stands |
| Must pass | A second file in the newly declared Go manifest format | `backend/second.mod` added beside `backend/go.mod` |
| Must pass | A second file in the newly declared Go lockfile format | `backend/second.sum` added beside `backend/go.sum` |

**What the cases prove, beyond what the table shows on its own.**

- **The case-variant and basename-reuse rows are the same claim from two directions.** Matching is
  exact — on the extension's characters for a suffixed file, on the full repository-relative path for
  an extensionless one — so neither a spelling variant of a declared extension nor a declared
  no-extension name showing up somewhere else earns a pass by resemblance.
- **The new-`.sh`/new-`.mjs` rows are the point of the whole exercise, not incidental cases.**
  ADR 0017 rev 9 names POSIX sh and Node as authoring *nothing* — the one pair of languages the
  decision is most explicit about — so `sh`/`mjs` were kept out of `EXTENSIONS` and moved into the
  per-path `LEGACY` bucket instead, mirroring how ADR 0017 rev 9 itself treats the files that do not
  yet conform: "a disposition rather than an exemption," named one file at a time. A version of this
  check that allowlisted the extensions instead would pass both seeds here, which is exactly the
  failure mode these two rows exist to catch.
- **The stale-`LEGACY`-entry row is the other half of that mirroring.** ADR 0017 rev 9 says the
  disposition list it gives "ends" per file as each conversion lands, not that the population is
  grandfathered forever; `main()` fails an entry whose file is no longer tracked so the allowlist
  cannot silently outlive what it was declared for. Without this row, deleting `check-eol.sh` after
  its `pre-commit` conversion lands would leave a dead grant nobody is forced to notice.
- **The empty-population row is the population guard, not an accident of the fixture.** Run this check
  from a path that does not put it two directories below a git worktree root and `git ls-files` fails
  outright (`CalledProcessError`, exit 128 from git) rather than returning nothing — that failure mode
  was hit once while building this fixture, from a script placed at the fixture root instead of under
  `scripts/`, and is why the case above is seeded with the script correctly placed and a genuinely
  empty index, not a broken `cwd`.
- **The two Go-format rows prove a declared extension rather than a grandfathered path.** `mod` and
  `sum` entered `EXTENSIONS`, not `LEGACY`, so a *second* file in either format passes wherever it
  sits — which is the claim, the formats being the Go toolchain's rather than one file's. A
  path-grandfathered disposition would fail both rows.
- **The tree-as-it-stands row is what makes every other row mean something.** Everything else here is
  a targeted seed; this one is the population `just verify` actually runs the check against, and it is
  the row that would catch the allowlist itself being wrong rather than the check's logic.

**Known rejections.**

- A declared extension spelled with different case (`.PY` beside `.py`) is rejected rather than
  folded in. That is intentional, not an oversight of case-insensitive filesystems: a case variant is
  an undecided spelling exactly as much as a new extension is, and folding case would quietly widen
  the declared set past what ADR 0017 rev 9 names.

**Known gaps.**

- **The audience binding is not checked at all — only the set is.** `EXTENSIONS` is tree-wide and
  says nothing about where a file sits, while ADR 0017 rev 9's Decision table binds a language to an
  audience and states in as many words that TypeScript is product-only. So
  `scripts/check-rogue.ts`, a repository check authored in TypeScript, **passes**: seeded and run, it
  reports one file above the baseline of the tree it ran on and exits 0. This is the largest thing the
  check does not decide, and it is the reason `CONTRIBUTING.md` review checklist item 12,
  *Languages*, leads with audience rather than with the set. Closing it means binding each declared
  extension to the paths it may occupy, which is a second rule rather than a wider allowlist, and
  nothing has yet argued for one.
- **A grandfather entry is a cheaper bypass than a declared extension, and nothing prices it.** A
  path added to `LEGACY` is matched before any extension logic, so one line admits a file in any
  language at all — no new extension, and nothing in the run distinguishes it from a disposition the
  record actually granted. Seeded: `scripts/rogue.rs` with a `LEGACY` line and `EXTENSIONS` untouched
  reports one file above baseline and exits 0. Both this and a new `EXTENSIONS` key are the same
  residue for review, which is why item 12 names the grandfather list explicitly rather than saying
  *the declared set*.
- **Content is never read.** A no-extension or `LEGACY`-grandfathered file passes on its path alone,
  and an extension-bearing file passes on its suffix alone — nothing here notices `LICENSE` replaced
  with a shell script, a `justfile` replaced with something `just` could not read, or a `.py` file
  that is not valid Python. That is deliberate: this check
  asserts the tree's declared *shape*, and a mismatch between a file's declared kind and its actual
  content is the reviewer's, under `CONTRIBUTING.md` review checklist item 12, *Languages* — the same
  item ADR 0017 rev 9 names as the mechanism for a language decision this check cannot make.
- **A `LEGACY` entry is not re-verified against the record that grants it.** If a disposition changed
  under ADR 0017 rev 9 or under ADR 0016 rev 12 — say, a ticket number renumbered — nothing here would
  notice the comment in `LEGACY` had gone stale, the way `check-adr-revs.py` notices a stale
  citation elsewhere in this repository. The two checks do not overlap: this one is not itself prose
  citing an ADR by number in the form that check parses.
