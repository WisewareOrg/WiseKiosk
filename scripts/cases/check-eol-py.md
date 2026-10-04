# `check-eol.py`

The inputs this check has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s; how to run a case is [`../README.md`](../README.md)'s.

`git grep -lIP '\r$' -- .`, inverted, run as the `check-eol` local hook in
[`.pre-commit-config.yaml`](../../.pre-commit-config.yaml). It is the authored remainder beside the
adopted `mixed-line-ending` hook, scoped to what that hook cannot reach
([ADR 0016 rev 12](../../docs/decisions/0016-maintained-tools-for-standard-artifacts.md)) — the gap is
demonstrated in the two rows marked *(gap)*, each run against `mixed-line-ending` itself, which
passed where this fails.

| Direction | Input |
|---|---|
| Must fail | a tracked file containing CRLF, in `.txt` and in `.md` |
| Must fail | a committed CRLF file with nothing staged *(gap: `mixed-line-ending` is handed only the staged set, so the same state passed it)* |
| Must fail | a uniformly-CRLF file *(gap: `mixed-line-ending` counts one ending kind as unmixed and passed the same file; it fails only a genuinely mixed one, which was seeded separately to prove it can fail)* |
| Must fail | the search failing rather than finding nothing — run outside a repository, where git exits 128 and the status is propagated |
| Must fail | an untracked CRLF file — reported by the untracked guard as unsearchable, not judged by the search |
| Must fail | an untracked all-LF file — same guard: visibility, not content, is what fails |
| Must fail | an untracked file beside a tracked CRLF file — one run reports both: the guard accumulates alongside the search rather than short-circuiting it |
| Must pass | an all-LF tree |
| Must pass | a binary file containing CR — excluded by `-I` |
| Must pass | the untracked all-LF file from above, once tracked — the guard is silent and the search judges it |
| Must pass | CR appearing mid-line, where the line still ends in LF |
| Must pass | a repository with no tracked file — the retired population guard, exercised to confirm the ruling's side, not to guard: ADR 0016 rev 12's owner ruling is that an empty scan may report success |

**What it does not catch: a file whose `.gitattributes` sets the `binary` attribute.** That attribute
both exempts the file from CRLF→LF normalisation when it is added *and* makes `-I` skip it during the
grep, so genuinely CRLF-terminated text commits and survives a fresh clone unseen. A plain `-text`
does not do this; only the full `binary` macro. `.gitattributes` declares `binary` on the image, font
and PDF globs — the attribute used as intended — so the reachable case is a *text* glob given it. The
owner ruled on 2026-08-02 not to gate that, so what holds is that **the LF invariant holds for files
git treats as text, and `.gitattributes` decides which those are.**

The check is not made redundant by git's own normalisation: a CRLF blob forced into history via
`hash-object`/`update-index`, bypassing the add-time filter, is still caught after a fresh checkout.
