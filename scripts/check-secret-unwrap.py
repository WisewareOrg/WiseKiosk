#!/usr/bin/env python3
"""Exactly one non-test reference to the secret type's unwrap method exists in the backend.

ADR 0023 rev 2 confines a secret to a type that cannot be emitted, and rests the structural half of
that on one property a reader cannot check by reading a diff: the sole call site that unwraps it to
the raw value is guarded by a lint. This is that lint. What it asserts is docs/CI.md § Module and
framework structure.

The population is the tracked, non-`_test.go` Go files under `backend/`. Test files are exempt by
decision, not by oversight: `secret_test.go` is what proves the redaction paths, and it cannot do that
without unwrapping. Tracked rather than on-disk, so an untracked file is invisible to a local run; CI
checks out every committed file, so the gate binds there — docs/CI.md § Repository shape lists this
as an exception.

The match reaches a method *value* (`f := s.Reveal`) as well as a call, so an unwrap aliased behind a
variable is counted rather than escaping the count, and it runs over the whole file rather than line
by line, so the selector split across a line break Go accepts is one reference rather than none. It is
textual: a mention inside a comment or a string literal counts too, which fails rather than passes.

**An empty population fails, and so does a tree declaring no `Reveal` at all.** Both would otherwise
report zero references and read exactly like a clean tree — a check keyed on a name finds nothing once
that name is gone, which is the fail-open this whole file exists to avoid.

What this has been run against, in both directions: cases/check-secret-unwrap.md
"""

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# A reference to a Reveal member: `s.Reveal()`, and `s.Reveal` handed on as a method value. Matched
# over the whole file rather than line by line, so the selector split across a line break Go accepts
# is one match rather than none.
REFERENCE = re.compile(r"\.\s*Reveal\b")
# The method's own declaration: a receiver's closing paren, then the name, then the parameter list.
DECLARATION = re.compile(r"\)\s*Reveal\s*\(")


def population():
    """The tracked, non-test Go files under backend/."""
    # check=True: a git failure (exit 128 outside a repository) raises rather than reading as an
    # empty listing, so a run that could not ask the question cannot report a clean tree.
    listing = subprocess.run(
        ["git", "ls-files", "-z", "--", "backend"],
        cwd=ROOT,
        capture_output=True,
        check=True,
    ).stdout
    names = (name.decode() for name in listing.split(b"\0") if name)
    return sorted(n for n in names if n.endswith(".go") and not n.endswith("_test.go"))


def main():
    files = population()
    if not files:
        print(
            "no tracked, non-test Go file under backend/ — the population this check reads is "
            "empty, so a zero count here is evidence of nothing.",
            file=sys.stderr,
        )
        return 1

    missing = [name for name in files if not (ROOT / name).is_file()]
    if missing:
        print(
            "tracked but absent from the working tree, so this check cannot read them — "
            "`git rm` each, or restore it:",
            file=sys.stderr,
        )
        for name in missing:
            print(f"  {name}", file=sys.stderr)
        return 1

    references = []
    declarations = []
    for name in files:
        text = (ROOT / name).read_text()
        lines = text.splitlines()
        for match in REFERENCE.finditer(text):
            number = text.count("\n", 0, match.start()) + 1
            references.append((name, number, lines[number - 1].strip()))
        declarations += [name for _ in DECLARATION.finditer(text)]

    if not declarations:
        print(
            "no Reveal method is declared in the non-test backend tree. This check is keyed on that "
            "name and reports zero references over any tree that does not use it, so a rename must "
            "reach this check too — rename it here, or retire the check with the decision it "
            "enforces (ADR 0023 rev 2).",
            file=sys.stderr,
        )
        return 1

    if len(references) != 1:
        print(
            f"{len(references)} non-test reference(s) to the secret type's unwrap method; "
            "ADR 0023 rev 2 obliges exactly one:",
            file=sys.stderr,
        )
        for name, number, text in references:
            print(f"  {name}:{number}: {text}", file=sys.stderr)
        if not references:
            print(
                "\nThe one legitimate unwrap site is gone. Restore it, or amend ADR 0023 rev 2 and "
                "this check together.",
                file=sys.stderr,
            )
        else:
            print(
                "\nEvery secret must reach the outbound request as the confined type; unwrapping it "
                "anywhere else defeats the containment. Remove the added site, or amend "
                "ADR 0023 rev 2 and this check together.",
                file=sys.stderr,
            )
        return 1

    name, number, _ = references[0]
    print(
        f"{name}:{number} is the only reference to the secret type's unwrap method, over "
        f"{len(files)} tracked non-test Go file(s) under backend/ carrying "
        f"{len(declarations)} declaration(s) of it."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
