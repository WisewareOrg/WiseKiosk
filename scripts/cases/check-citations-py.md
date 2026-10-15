# `check-citations.py`

The inputs this check has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s; how to run a case is [`../README.md`](../README.md)'s.

| Direction | Input |
|---|---|
| Must fail | a citation naming no item |
| Must fail | a citation carrying the wrong header for a real item |
| Must fail | an `ADR` number naming no file |
| Must fail | a **lowercase** identifier, cited or not |
| Must fail | a mixed-case identifier |
| Must fail | an untracked `.md` — the untracked guard names it as unreadable, whatever it contains |
| Must pass | an identifier or ADR number inside a fenced code block |
| Must pass | an untracked `.md` under `.claude/` — outside this gate's population, so outside its guard |
| Must pass | a word merely containing an identifier |
| Must pass | a correct uppercase citation with its verbatim header |

The case rows exist because a lowercase citation carrying a fabricated header on a *real* item passed
clean — the exact failure the check exists to catch, invisible rather than reported. The owner ruled
on 2026-08-02 that a mis-cased identifier is malformed.

**Two wrapping rules, and they differ.** The `ADR` pattern admits exactly one line break between the
word and its number; the header normaliser admits any number, including a blank-line paragraph break.
Neither is a false-resolve — CommonMark reads the comment as one either way — and the difference is
between two readers rather than between a document and its code.
