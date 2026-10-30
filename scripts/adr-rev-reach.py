#!/usr/bin/env python3
"""The citations a rev bump re-pinned without touching the sentence they hang off.

wise-ci's `check-adrs`'s `check-adr-revs.py` holds every citation to its ADR's current rev, and says
in its own docstring that whether a citation pinning the current rev still means what the ADR says is
decided by nothing there. A sweep that rewrites `rev N` to `rev N+1` across the tree satisfies that
gate while re-deciding nothing, and the green result then certifies a reach that did not happen.

This enumerates the work that sweep leaves. It decides nothing about a claim and fails on no diff:
what it produces is a list of file, line and sentence for a reader to judge, under
`CONTRIBUTING.md` review checklist question 21, *Rev reach*.

Reported. For each ADR whose head rev moved between the base ref and the head tree, and whose body
outside its head-rev line and its *Revisions* section also changed: every line in the tracked set
that is identical across the two trees once every `rev N` token is masked, and different before
masking. That is a line whose sole edit was the pin. The tracked set is every file, not the Markdown
in it: it mirrors
[wise-ci's `check-adrs`](https://github.com/WisewareOrg/wise-ci/tree/main/check-adrs)'s own
population, not restated here, so a sweep reaches every file that gate reads. A file not decodable
as text at either tree is named in the output rather than passed over, with which of the two reasons
applies, since a shrunken population reported as a whole one is the failure this tool exists to
catch.

Not reported, each deliberately:

- A citing line the change also edited. The author had the sentence open; a sweep did not write it.
  Alignment comes from a difflib `equal` block over the masked lines, so an edited line falls out of
  the pairing rather than being tested and passed.
- Every citation of an ADR whose body did not move — a rev that only records its own text, or one
  whose only edit was re-pinning the ADRs it cites. Masking is applied to the body comparison for
  that second case, so a sweep does not cascade into the records it passes through.
- A citation whose pin did not move, which `check-adrs` fails on independently, and one added by
  the change, which is not a re-pin.
- A changelog line and its continuations in `docs/decisions/`, which pin a rev deliberately.

Not decidable here, and the reason this reports rather than fails: whether the claim survived. A rev
that does not touch what a citation asserts is legal and ordinary — most of a sweep is that — so a
gate failing every pin-only edit would fail closed on legal input, and the exemption list it would
grow is where a bypass gets spelled ([ADR 0011 rev 2](../docs/decisions/0011-requirement-or-convention.md)
routes a judgement obligation to the review checklist instead).

What this has been run against, in both directions: cases/adr-rev-reach-py.md
"""

import difflib
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DECISIONS = "docs/decisions"

ADR_FILE = re.compile(r"^(\d{4})-[a-z0-9-]+\.md$")
HEAD_REV = re.compile(r"^\*\*Rev:\*\* *(\d+) *$", re.M)
# The one accepted citation spelling, in prose and inside a link title alike; anything else is
# `check-adrs`'s to reject, and a tree it has passed carries none.
CITATION = re.compile(r"\bADR (\d{4}) rev (\d+)\b")
REV_TOKEN = re.compile(r"\brev \d+\b")

SECTION = re.compile(r"^## +(.+?) *$")
CHANGELOG = re.compile(r"^- \*\*rev (\d+)\*\* — ")
CONTINUATION = re.compile(r"^ {2,}\S")


def fail(message):
    print(f"adr-rev-reach: {message}", file=sys.stderr)
    return 2


def git(*arguments):
    return subprocess.run(
        ["git", *arguments], cwd=ROOT, capture_output=True, check=True
    ).stdout


def default_base():
    """The merge base with `main`, which is the commit a branch's whole diff is read against."""
    try:
        return git("merge-base", "main", "HEAD").decode().strip()
    except subprocess.CalledProcessError:
        return None


def resolved(ref):
    """A ref this can read a tree from, reported by name rather than by a traceback."""
    if ref is None:
        return True
    try:
        git("rev-parse", "--verify", "--quiet", f"{ref}^{{commit}}")
        return True
    except subprocess.CalledProcessError:
        return False


def listing(ref):
    """Every tracked path at a ref, or in the worktree where the ref is None."""
    if ref is None:
        return [name.decode() for name in git("ls-files", "-z").split(b"\0") if name]
    return [
        name.decode()
        for name in git("ls-tree", "-r", "-z", "--name-only", ref).split(b"\0")
        if name
    ]


def read(ref, path):
    """One path's text at a ref as (text, why-not), the two reasons kept apart.

    A tracked path missing from disk and a tracked path holding bytes are different facts about the
    population, and the worktree reaches the first: `git ls-files` lists a file deleted but not yet
    staged, which a reader told it was undecodable would read as a wrong answer rather than a rebase.
    """
    try:
        raw = (ROOT / path).read_bytes() if ref is None else git("show", f"{ref}:{path}")
    except (OSError, subprocess.CalledProcessError):
        return None, "tracked, but no content at this tree"
    try:
        return raw.decode("utf-8"), None
    except UnicodeDecodeError:
        return None, "not decodable as text"


def masked(line):
    """A line with every rev pin blanked, so two spellings of one sentence compare equal."""
    return REV_TOKEN.sub("rev ~", line)


def revisions_lines(text):
    """The indices of the changelog lines and their continuations in an ADR's *Revisions* section."""
    exempt, inside, entry = set(), False, False
    for index, line in enumerate(text.split("\n")):
        heading = SECTION.match(line)
        if heading:
            inside, entry = heading.group(1) == "Revisions", False
            continue
        if not inside:
            continue
        if CHANGELOG.match(line):
            entry = True
            exempt.add(index)
        elif entry and CONTINUATION.match(line):
            exempt.add(index)
        else:
            entry = False
    return exempt


def substance(text):
    """An ADR's text with what a rev necessarily rewrites removed, and its own pins masked.

    What is left is the part a citation of this ADR can be a claim about. The head rev line and the
    *Revisions* section move on every rev by construction, so comparing them would report every rev
    as substantive; masking the pins drops a sweep through this record's own citations, which is the
    change this tool exists to say nothing follows from.
    """
    kept, inside = [], False
    for index, line in enumerate(text.split("\n")):
        heading = SECTION.match(line)
        if heading:
            inside = heading.group(1) == "Revisions"
        if inside or HEAD_REV.match(line):
            continue
        kept.append(masked(line))
    return "\n".join(kept)


def latest_revision_line(text):
    """The changelog line for the head rev, which is what the bumper said the rev did."""
    heads = HEAD_REV.findall(text)
    if len(heads) != 1:
        return None
    wanted, inside, collected = heads[0], False, []
    for line in text.split("\n"):
        heading = SECTION.match(line)
        if heading:
            if collected:
                break
            inside = heading.group(1) == "Revisions"
            continue
        if not inside:
            continue
        entry = CHANGELOG.match(line)
        if entry:
            if collected:
                break
            if entry.group(1) == wanted:
                collected.append(line)
        elif collected and CONTINUATION.match(line):
            collected.append(line.strip())
        elif collected:
            break
    return " ".join(collected) or None


def adrs(ref, problems):
    """Each ADR number at a ref, with the rev its head declares and its substantive text."""
    found = {}
    for path in listing(ref):
        directory, _, name = path.rpartition("/")
        if directory != DECISIONS:
            continue
        number = ADR_FILE.match(name)
        if not number:
            continue
        text, why_not = read(ref, path)
        if text is None:
            problems.append(f"{path} at {ref or 'the worktree'}: {why_not}")
            continue
        heads = HEAD_REV.findall(text)
        if len(heads) != 1:
            problems.append(
                f"{path} declares {len(heads)} `**Rev:** N` lines at {ref or 'the worktree'}, "
                "expected 1 — `check-adrs` is what fails on this"
            )
            continue
        found[number.group(1)] = (int(heads[0]), substance(text), text, path)
    return found


def revved(base, head, problems):
    """Each ADR whose head rev moved, split on whether its substance moved with it."""
    before, after = adrs(base, problems), adrs(head, problems)
    # The mirror of `check-adrs`'s `check-adr-revs.py`'s guard, at both ends: a ref this read no
    # ADR from is a ref this tool has not read, and reporting no work over it is the same output
    # as a branch with none.
    for ref, population in ((base, before), (head, after)):
        if not population:
            problems.append(
                f"no ADR declared a rev at {ref or 'the worktree'} — the head format, that ref, or "
                "this parser is wrong"
            )
    substantive, administrative = {}, {}
    for number, (rev, body, text, path) in sorted(after.items()):
        if number not in before or before[number][0] == rev:
            continue
        entry = (before[number][0], rev, path, latest_revision_line(text))
        if before[number][1] == body:
            administrative[number] = entry
        else:
            substantive[number] = entry
    return substantive, administrative


def pin_only(base, head, moved, problems):
    """Every line whose sole edit was a pin of one of `moved`, as number → (path, line, text)."""
    work, unreadable = {number: [] for number in moved}, []
    if not moved:
        return work, unreadable
    at_head = set(listing(head))
    for path in sorted(at_head & set(listing(base))):
        (old, why_old), (new, why_new) = read(base, path), read(head, path)
        if old is None or new is None:
            # Named rather than skipped: the population this walks is the population
            # `check-adrs` holds to a rev, and a file dropped from it silently would shrink
            # the reach reported below without shrinking the sweep that produced it.
            unreadable.append((path, why_old or why_new))
            continue
        old_lines, new_lines = old.split("\n"), new.split("\n")
        if not CITATION.search(old) and not CITATION.search(new):
            continue
        exempt = revisions_lines(new) if path.startswith(f"{DECISIONS}/") else set()
        matcher = difflib.SequenceMatcher(
            None, [masked(line) for line in old_lines], [masked(line) for line in new_lines]
        )
        for tag, old_start, old_end, new_start, _ in matcher.get_opcodes():
            # Only an `equal` block: it is what makes the pairing sound, and it is exactly the
            # condition "the same sentence, whatever its pins". Every other opcode is a line the
            # change wrote.
            if tag != "equal":
                continue
            for offset in range(old_end - old_start):
                old_line = old_lines[old_start + offset]
                new_line = new_lines[new_start + offset]
                if old_line == new_line or new_start + offset in exempt:
                    continue
                was = CITATION.findall(old_line)
                is_now = CITATION.findall(new_line)
                if len(was) != len(is_now):
                    problems.append(
                        f"{path}:{new_start + offset + 1}: the citations on this line differ in "
                        "count while the line is otherwise identical — this parser is wrong"
                    )
                    continue
                for (number, before_rev), (_, after_rev) in zip(was, is_now):
                    if before_rev != after_rev and number in work:
                        work[number].append(
                            (path, new_start + offset + 1, new_line.strip())
                        )
    return work, sorted(unreadable)


def report(substantive, administrative, work, unreadable):
    items = 0
    for number, (was, is_now, path, note) in substantive.items():
        lines = work[number]
        items += len(lines)
        print(f"\nADR {number} rev {was} → {is_now}, and its body moved with it — {path}")
        if note:
            print(f"  {note}")
        if not lines:
            print("  no citation was re-pinned without its sentence being edited")
            continue
        print(f"  re-pinned, sentence untouched — read each against the diff above:")
        for citing, line, text in lines:
            print(f"    {citing}:{line}: {text}")
    for number, (was, is_now, path, _) in administrative.items():
        print(
            f"\nADR {number} rev {was} → {is_now}, body unchanged — {path}\n"
            "  nothing to re-read: the rev rewrote no claim a citation can be about"
        )
    if unreadable:
        print("\ntracked but unjudged here, each with why:")
        for path, why_not in unreadable:
            print(f"  {path}: {why_not}")
    total = len(substantive) + len(administrative)
    print(
        f"\n{total} ADR(s) revved; {items} citation(s) re-pinned by a sweep and not otherwise read"
        if total
        else "\nno ADR revved between these trees; nothing to re-read"
    )


def main(argv):
    base = argv[1] if len(argv) > 1 else default_base()
    head = argv[2] if len(argv) > 2 else None
    if len(argv) > 3:
        return fail("usage: adr-rev-reach.py [<base-ref> [<head-ref>]]")
    if base is None:
        return fail("no base ref given and `git merge-base main HEAD` resolved nothing")
    for ref in (base, head):
        if not resolved(ref):
            return fail(f"{ref!r} names no commit")
    problems = []
    substantive, administrative = revved(base, head, problems)
    work, unreadable = pin_only(base, head, substantive, problems)
    if problems:
        for problem in sorted(problems):
            print(f"adr-rev-reach: {problem}", file=sys.stderr)
        return 2
    print(f"citations re-pinned between {base} and {head or 'the worktree'}:")
    report(substantive, administrative, work, unreadable)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
