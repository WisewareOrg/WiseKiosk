#!/usr/bin/env python3
"""Every tracked file's extension is in a declared set.

[ADR 0017 rev 9](../docs/decisions/0017-authored-language-set.md) binds two authored languages to
their audience — Go, TypeScript, the Svelte component format and CSS for what ships, Python (standard
library only) for what checks the repository — and rules that everything else in the tree is
*derived*: a toolchain's own required input format, not a language anyone authors in. Documentation,
and the assets a build serves, are carved out of the decision entirely — CSS excepted, which also
carries a documentation disposition beside its authored one, both named where the extension is
declared below.

This asserts that every extension actually present is one of those three kinds, declared and
attributed below — never inferred from what happens to already be in the tree. A file type nobody has
decided about **fails**, which is the opposite of a denylist: the unbounded set ADR 0017 rev 9 rejected
would pass silently on a new extension, and only a bounded, declared set can fail on one.

`sh` and `mjs` are **not** declared extensions: ADR 0017 rev 9 names POSIX sh and Node as authoring
nothing at all, so an allowlisted `.sh`/`.mjs` extension would be the one hole that lets past exactly
the violation this check exists to catch. The check scripts already written in them are real, but each
carries **a disposition rather than an exemption** — ADR 0017 rev 9's own words for this, applied here
the same way it applies there — so each is declared individually, by its exact repository-relative
path, in `LEGACY`, citing the record that ends it. A *new* `.sh` or `.mjs` file, anywhere not in that
list, fails — and an entry whose file is no longer tracked fails too, so the list cannot silently
outlive the conversion it was grandfathering.

A file with no extension is judged the same way, by its exact repository-relative path in
`NO_EXTENSION` — `justfile`, `LICENSE` and `.gitignore` serve nothing in common, and a shared
no-extension bucket would wave through the next one just as silently as an unbounded extension set
would.

Population is `git ls-files`. An empty population fails rather than passing: a run that resolved no
tracked file judged nothing, and this repository's whole review checklist turns on that not reading
as a clean tree.

What this has been run against, in both directions: cases/check-languages-py.md
"""

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# Extension (without the leading dot, exact case) -> which kind it is and what it serves.
EXTENSIONS = {
    # Authored — what ships (ADR 0017 rev 9 Decision table).
    "go": "authored — Go, what ships to a user or an operator (ADR 0001 rev 1, ADR 0017 rev 9)",
    "ts": "authored — TypeScript, what ships (ADR 0017 rev 9)",
    "svelte": "authored — the Svelte component format, what ships "
    "(ADR 0018 rev 1, ADR 0017 rev 9)",
    "css": "authored — the shared product stylesheet, what ships (the display styling contract, ADR 0017 rev 9); "
    "also documentation — an asset the Sphinx docs-site build serves, a Furo theme override "
    "(ADR 0017 rev 9)",
    # Authored — what checks the repository (ADR 0017 rev 9 Decision table).
    "py": "authored — Python, standard library only, what checks the repository (ADR 0017 rev 9)",
    # Derived — a toolchain's own required input format (ADR 0017 rev 9 Decision).
    "yml": "derived — a toolchain's own required input format: GitHub Actions workflow YAML, "
    "Doorstop item and silo config YAML (ADR 0017 rev 9)",
    "yaml": "derived — a toolchain's own required input format: pre-commit's "
    ".pre-commit-config.yaml (ADR 0016 rev 12), the boundary schema both code generators "
    "read and oapi-codegen's own configuration (ADR 0008 rev 6, ADR 0017 rev 9)",
    "mod": "derived — the Go toolchain's own manifest format: backend/go.mod "
    "(ADR 0017 rev 9)",
    "sum": "derived — the Go toolchain's own lockfile format: backend/go.sum "
    "(ADR 0017 rev 9)",
    "json": "derived — a toolchain's own required input format: npm's package.json and "
    "package-lock.json, Claude Code's settings.json, commitlint's .commitlintrc*.json; also "
    "derived — data an authored check reads and does not itself author: the vendored SPDX 2.3 "
    "schema scripts/publish/spdx-schema-2.3.json the SBOM check validates against",
    "likec4": "derived — LikeC4's own model format, named in ADR 0017 rev 9 "
    "(ADR 0003 rev 4)",
    "mmd": "derived — generated Mermaid output of the LikeC4 export/splice toolchain "
    "(scripts/splice-arch-diagrams.py), never hand-authored",
    "txt": "derived — a toolchain's own required input format (pip's requirements-dev.txt); also "
    "derived — data an authored check reads and does not itself author: the secret-pattern "
    "enumeration scripts/image/layer_secret_scan.py matches image layers against, rather "
    "than restating it (docs/TESTING.md); also documentation — legal text, not an authored "
    "program: the licence and attribution the bundled faces are redistributed under, beside "
    "them in frontend/src/assets/ (ADR 0017 rev 9)",
    "regex": "derived — data an authored check reads and does not itself author: the "
    "single-source-of-truth branch pattern check-branch.py and branch-shape.py "
    "each read instead of restating it (docs/CI.md)",
    # Documentation, and the assets a build serves — ADR 0017 rev 9 states this decision does not
    # reach them.
    "md": "documentation — not an authored program; ADR 0017 rev 9 does not reach documentation",
    "png": "documentation — a rendered figure a Markdown document embeds and an asset the Sphinx "
    "docs-site build serves (ADR 0017 rev 9: 'the assets a documentation build serves')",
    # Two dispositions under one extension, the shape `.css` already takes.
    "html": "derived — Vite's own required input format: frontend/index.html is the single entry "
    "the build reads and rewrites (ADR 0018 rev 1, ADR 0017 rev 9); also documentation — an asset "
    "the Sphinx docs-site build serves (ADR 0017 rev 9)",
    "woff2": "an asset a build serves — not an authored program: the two bundled faces the display "
    "styling contract states, Inter for every readable character and the icon face the weather "
    "module's glyphs are drawn from, both self-hosted because the display page reaches no "
    "origin but the backend's (ADR 0017 rev 9)",
    "gif": "an asset a build serves — not an authored program: the render tier's exempt-imagery "
    "fixture, a same-origin file so the served img-src 'self' admits it the way a data: URI "
    "cannot (ADR 0017 rev 9)",
    "svg": "an asset a build serves — not an authored program: the park-wait-times module's own "
    "icon set, original stylised line-glyph artwork rather than program text (ADR 0017 rev 9)",
}

# Exact repository-relative path -> which kind it is and what it serves, for a file with no
# extension. Matched by full path, not by basename: these files serve nothing in common, and a
# basename match would let a same-named file anywhere else in the tree pass unjudged.
NO_EXTENSION = {
    "justfile": "derived — invoking `just`, named explicitly in ADR 0017 rev 9",
    "LICENSE": "documentation — legal text, not an authored program",
    "Dockerfile": "derived — the container build's own required input format, named explicitly in "
    "ADR 0017 rev 9 (ADR 0021 rev 4)",
    ".gitignore": "derived — git's own required input format",
    ".gitattributes": "derived — git's own required input format",
    ".editorconfig": "derived — EditorConfig's own required input format",
    ".dockerignore": "derived — the container build's own required input format: what its context "
    "excludes",
    ".github/CODEOWNERS": "derived — GitHub's own required input format",
}


# Exact repository-relative path -> the record giving that file its disposition. `sh` and `mjs` are
# NOT declared extensions: ADR 0017 rev 9 says they author nothing, so a *new* file in either must
# fail. The files below predate that decision and each carries "a disposition rather than an
# exemption" in ADR 0017 rev 9's own words — so they are grandfathered one at a time, and `main()`
# fails an entry here whose file is no longer tracked, so the list empties itself as each conversion
# lands rather than accumulating dead grants.
LEGACY: dict[str, str] = {}


def fail(problems):
    print(f"{len(problems)} problem(s):", file=sys.stderr)
    for problem in problems:
        print(f"  {problem}", file=sys.stderr)
    return 1


def tracked():
    """Every tracked path, from git rather than a walk — the set CI is asked to judge."""
    listing = subprocess.run(
        ["git", "ls-files", "-z"], cwd=ROOT, capture_output=True, check=True
    ).stdout
    return [name.decode() for name in listing.split(b"\0") if name]


def untracked():
    """Untracked, non-ignored paths — outside the population above, so reported rather than
    silently unjudged."""
    listing = subprocess.run(
        ["git", "ls-files", "--others", "--exclude-standard", "-z"],
        cwd=ROOT, capture_output=True, check=True,
    ).stdout
    return [name.decode() for name in listing.split(b"\0") if name]


def main():
    problems = []
    files = tracked()
    if not files:
        return fail(["no tracked file was resolved — an empty population is not a clean run"])

    problems.extend(
        f"{name}: untracked, so this check cannot judge it — git add or gitignore it"
        for name in untracked()
    )

    judged = 0
    for name in files:
        # A file grandfathered by path is judged first: sh and mjs are not declared extensions, so
        # the named legacy files pass here and a new one in either language falls through and fails.
        if name in LEGACY:
            judged += 1
            continue
        suffix = Path(name).suffix
        if not suffix:
            if name not in NO_EXTENSION:
                problems.append(f"{name}: no extension, and no declared entry for this exact path")
            else:
                judged += 1
            continue
        extension = suffix[1:]
        if extension not in EXTENSIONS:
            problems.append(
                f"{name}: extension .{extension} is not in the declared set — decide whether it is "
                f"authored or derived (ADR 0017 rev 9), or grandfather this path with its disposition"
            )
        else:
            judged += 1

    stale = sorted(set(LEGACY) - set(files))
    problems.extend(
        f"{name}: grandfathered as legacy but no longer tracked — its disposition landed, so the "
        f"entry goes with it" for name in stale
    )

    if problems:
        return fail(sorted(problems))
    print(f"{judged} tracked files, every extension in the declared set")
    return 0


if __name__ == "__main__":
    sys.exit(main())
