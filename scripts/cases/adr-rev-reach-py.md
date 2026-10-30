# `adr-rev-reach.py`

The inputs this check has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s; how to run a case is [`../README.md`](../README.md)'s.

Not a check — it reports and exits zero, and sits outside `just verify`. Its cases are still recorded
here, because what it must and must not list is the same kind of claim every row above makes.
Exercised against the sweep that motivated it, `52bb933` on the branch squashed as `197d075`.

| Direction | Case | Input | Reported |
|---|---|---|---|
| Must list | The sweep that hid two false claims | `52bb933^ 52bb933` — three ADRs revved, the tree re-pinned | 55 citations, including both lines whose claim the same commit falsified |
| Must not list | A sentence the author rewrote | `52bb933^ 8f43ccb` — the same sweep with the two fixes folded in | neither line; an edited line falls out of the pairing rather than being tested and passed |
| Must not list | An administrative rev | an ADR revved with a changelog line and nothing else | classified *body unchanged*; none of its citations appear |
| Must not list | A rev the sweep merely passed through | an ADR whose only body edit is re-pinning its own citations of the revved record | classified *body unchanged*, so the sweep does not cascade into the records it crosses |
| Must not list | No rev in range | `8f43ccb 62c0a48` | `no ADR revved between these trees; nothing to re-read`, exit 0 |
| Must list | A pin-only line outside Markdown | the sweep that took the authored-language-set decision to its fifth revision, whose 41 citations sit 28 in a Python docstring and its comments, 12 in Markdown, 1 hand-edited | all 40 pin-only lines, `scripts/check-languages.py` among them |
| Must not list | A line outside Markdown whose sentence the author rewrote | the same sweep with `scripts/check-languages.py:153` reworded alongside its pin | 39, that line absent — the pairing rule holds identically outside Markdown |
| Must name | A tracked file not decodable as text at either tree | a binary committed at the base and carried to the head across that sweep | named *not decodable as text*, never passed over |
| Must name | A tracked file with no content at one tree | `docs/CI.md` deleted from disk but not from the index, head defaulted to the worktree | named *tracked, but no content at this tree* — a distinct reason, in the same run as the binary above |

The two *body unchanged* rows are the pair that matters, and they are why substance is compared with
the head rev line dropped, the *Revisions* section dropped, and every rev token in the remainder
masked. Without the masking, every record a sweep crosses differs textually while asserting what it
asserted before, and its own citations join the list — the report grows with the sweep rather than
with the work. The two rows were seeded together over a clean `main` and re-run: 30 citations
reported, none of them from either seeded ADR.

**The list shrinks only on an edit.** A reviewer who reads a sentence and finds it still true leaves
it untouched, so it is listed again next run. That is not a defect to close: reaching zero would
reward editing a sentence cosmetically to clear it.

**The population is every tracked file, and was not always.** As first written this walked Markdown
alone. The sweep that took the authored-language-set decision to its fifth revision is what exposed
it: 28 of its 41 citations sit in a Python docstring and its comments, all of them held to the
current rev by `check-adr-revs`, none of them visible
here — and the summary line still read as the whole population. Ablation on one fixture, the two
scripts differing only in that restriction: **12 citations reported before, 40 after**, and the
binary file named only by the second. A reporter whose population is narrower than the gate it audits
does not under-report loudly; it reports a smaller number in the same sentence.

**Why the two unjudged reasons are kept apart.** The default invocation — the `pre-push` hook, and
`just rev-reach` with no head ref — compares against the worktree, where `git ls-files` lists a file
deleted from disk but not yet staged. Reporting that as undecodable bytes would tell a contributor
mid-rebase that a UTF-8 document is binary, and the block they learn to distrust is the one this
tool exists to make worth reading.

**Known gaps.**

- **A citation re-pinned and re-wrapped in the same change is not listed.** Alignment is a `difflib`
  `equal` block over the masked lines, so a reflowed line is a `replace` and falls out of the pairing
  exactly as a rewritten one does — the tool cannot tell a mechanical wrap from an author who had the
  sentence open. Seeded and observed: an ADR revved with a real body edit, one citing line re-pinned
  and reflowed onto two lines, reported as *no citation was re-pinned without its sentence being
  edited*. Reachable here, because re-wrapping is what a prose pass does and this repository's own
  citations are wrap-sensitive. Closing it means pairing on something other than line identity, which
  is a larger change than the tool has earned; a sweep that also reflows should be split into two
  commits, and the pin-only one is what to run this against.
- **Its population is complete only while `check-adrs` is green** — a citation a sweep missed
  fails that gate and never reaches this tool.
- **Line granularity is an approximation.** A claim can rot several lines below the citation it hangs
  off. The line printed is a handle to the paragraph a reader opens, not the extent of what they read.
