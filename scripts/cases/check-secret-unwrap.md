# `check-secret-unwrap.py`

The inputs this check has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md) § *Module and framework structure*'s and
[ADR 0023 rev 2](../../docs/decisions/0023-secret-output-containment.md)'s; how to run a case is
[`../README.md`](../README.md)'s.

Each case is a `git archive 7f48366` copy of the tracked tree — the backend as it stood before this
check existed — with the check copied in fresh over it, seeded in place, `git add -A` and committed so the
population `git ls-files` returns is the seeded one, then `python3 scripts/check-secret-unwrap.py` run
inside the copy. **The script under test is blob `1c34a974ba6a797d01480e262e3a7f1639155c09`**
(`git hash-object scripts/check-secret-unwrap.py`), md5 `d3153f6b407f2c42ae6f3ad35c390058`, asserted
before every run: a `git archive` fixture otherwise carries the *fixture commit's* copy, so a case
would exercise a script that predates the fix it is testing. Every seed is confirmed to have landed
with `git diff --quiet HEAD` before the result is read, a clean diff read as the case failing.

The `7f48366` tree holds ten tracked non-test Go files under `backend/`, one declaration of the unwrap
method and one reference to it; the counts in the pass rows below are against that baseline.

| Direction | Case | Input |
|---|---|---|
| Must fail | A second non-test call site | `func leak(s secret.Secret) string { return s.Reveal() }` appended to `internal/upstream/proxy.go` — 2 references, both named with file, line and text |
| Must fail | A second non-test site as a method **value**, no call parens | `return s.Reveal` appended to the same file — an unwrap aliased behind a variable is counted, not escaped |
| Must fail | A second non-test site with the selector split across a line break | `return s.` / `Reveal()` on two lines — the match runs over the whole file, so the split spelling is one reference rather than none |
| Must fail | The one legitimate call site removed | `resolved.Reveal()` in `internal/router/router.go` replaced with `""` — 0 references, reported as the site being gone rather than as a clean tree |
| Must fail | The method renamed, every site renamed with it | `Reveal` → `Unwrap` across `secret.go` and `router.go` — refused on the *declaration* being absent, before the reference count is read |
| Must fail | No tracked non-test Go file under `backend/` | every one `git rm`-ed — the empty population is refused rather than counted |
| Must fail | Those files deleted with plain `rm`, still tracked | each named as tracked-but-absent, so a broken worktree cannot read as a zero count |
| Must pass | A test file gaining two more unwrap calls | two `s.Reveal()` added to `secret_test.go` — `_test.go` is exempt by decision, and the count is unmoved at 1 |
| Must pass | The legitimate unwrap respelled across a line break | `resolved.` / `Reveal()` on two lines — legal Go the whole-file match still reads as the one site |
| Must pass | The tree as it stands | 1 reference over 10 files carrying 1 declaration |

**The two `Must pass` respellings are the point of the pair.** A count-based check has two ways to be
wrong that a green run hides equally: rejecting a legal spelling of the one site, and failing to see a
second site spelled unusually. The line-break rows sit on both sides of exactly that — the same
spelling is a pass in the legitimate site and a fail as a second one.

**Failure at the declaration guard runs before the count**, which is what makes the rename row
distinct from the removal row: both leave zero references, and only the declaration guard tells *the
check is keyed on a name nobody uses any more* apart from *someone deleted the unwrap*. Without it a
rename would turn the check into one that passes over every tree there is.

**Known gaps**, each measured on the same fixture rather than inferred:

- **An untracked second call site passes.** `internal/upstream/leak.go` written but never `git add`-ed
  — the run reports the one site and exits 0. The population is `git ls-files`; this is the exception
  [`docs/CI.md`](../../docs/CI.md) § *Repository shape* lists, the same hole every `git ls-files`-based
  gate here carries.
- **A second site reached by reflection passes.** `reflect.ValueOf(s).MethodByName("Reveal").Call(nil)`
  appended to `internal/upstream/proxy.go`, tracked and committed — the run reports the one site and
  exits 0. The match is on a selector; a name that reaches the method as a string is outside it. This
  is inherent to a textual check and is why ADR 0023 rev 2 composes the structural half with a
  behavioural canary rather than resting on either.
- **A mention in a comment or a string literal counts.** Not measured as a row because it fails rather
  than passes — the direction that costs a false red, not a false green.
- **The gate says nothing about what the one site does with the value.** It counts unwraps; redaction
  through every formatting path is the `secret` package's own tests'.
