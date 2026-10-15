# The Conventional-Commit gate: `commitlint`, commit-msg stage

The inputs the adopted gate has been run against, in both directions. The gate replaces
`check-commit-msg.sh` and `conventional-commit.regex`
(ADR 0016 rev 13; #107 adopt commitlint), and the rows below are that check's recorded cases re-run
against the adopted tool — including the must-pass rows, since an adopted tool may reject legal
input the authored check accepted.

**Tool, pinned as run:** `@commitlint/cli` 21.2.2 with `@commitlint/config-conventional` 21.2.2,
installed by pre-commit 4.6.2 from the `additional_dependencies` pins in
[`.pre-commit-config.yaml`](../../.pre-commit-config.yaml), reading
[`.commitlintrc.json`](../../.commitlintrc.json). A case fails if the hook exits non-zero.

**Run a case** from the repository root, against the same hook definition CI uses:

```sh
printf '%s\n' "<message>" > "$f"
scripts/.venv/bin/pre-commit run commitlint --hook-stage commit-msg --commit-msg-filename "$f"
```

| Direction (retired check) | Input | Result |
|---|---|---|
| Must fail | `some plain prose` | fail (`type-empty`, `subject-empty`) |
| Must fail | `Fix: a thing` | fail (`type-case`, `type-enum`) |
| Must fail | `feat a thing` | fail (`type-empty`, `subject-empty`) |
| Must fail | `feat(): a thing` | **pass — gap, see note** |
| Must fail | `feat(CI): a thing` | fail (`scope-case`) |
| Must fail | `feat: ` | fail (`header-trim`, `subject-empty`) |
| Must fail | `feats: a thing` | fail (`type-enum`) |
| Must pass | `fix: a thing` | pass |
| Must pass | `feat(ci): a thing` | pass |
| Must pass | `feat(ci)!: a thing` | pass |
| Must pass | `fixup! feat: x` | pass (`defaultIgnores`) |
| Must pass | `squash! feat: x` | pass (`defaultIgnores`) |
| Must pass | `Merge branch 'x'` | pass (`defaultIgnores`) |

Three notes the rows cannot carry alone:

- **The uppercase-scope row is restored, not given up.** `config-conventional` carries no
  `scope-case` rule, so `feat(CI): x` passed where the retired regex refused it; the base
  configuration adds `scope-case: [2, always, lower-case]`, which restores the refusal with every
  must-pass row above — the hyphenated scope included — re-run and still passing.
- **One must-fail row is given up: empty scope.** `feat(): x` passes, and no one-line rule closes
  it: the parser hands a scopeless header and an empty-parens header the same empty scope, so
  `scope-empty: [2, never]` — the obvious rule — also rejects the must-pass `fix: a thing` (both
  measured). The gap stays recorded here, deferred to the owner.
- **Legal input the adopted tool rejects**, each measured: `fix: Sentence case subject` fails
  `subject-case`, `fix: a thing.` fails `subject-full-stop`, and a 105-character header fails
  `header-max-length` (100) — the tightening ADR 0016 rev 13 accepts, titles adapting rather than
  the tool being configured around them.
