#!/usr/bin/env python3
"""Tooling is siloed with the feature it serves: a depth-1 listing of the
repository root holds no manifest or environment directory, and the root
renovate.json parses as JSON and extends the WisewareOrg/wise-renovate
preset, tag-pinned. See docs/CI.md § Repository shape.

No dependencies: Python stdlib only, plain text scanning (no YAML parser).

What this has been run against, in both directions: cases/check-repo-silo-py.md
"""

import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

problems = []


def listing(directory):
    try:
        return sorted((ROOT / directory).iterdir(), key=lambda path: path.name)
    except OSError:
        return None


ROOT_FORBIDDEN = [
    re.compile(r"^package\.json$"),
    re.compile(r"^go\.mod$"),
    re.compile(r"^pyproject\.toml$"),
    re.compile(r"^requirements.*\.txt$"),
    re.compile(r"^\.venv$"),
]

for entry in listing("."):
    if any(pattern.search(entry.name) for pattern in ROOT_FORBIDDEN):
        problems.append(
            f"'{entry.name}' sits at the repository root — silo it with the feature it serves"
        )

# The WisewareOrg/wise-renovate preset, tag-pinned: renovate.json must exist at the root, parse as
# JSON, and extend it. See docs/CI.md § Repository shape.
RENOVATE_CONFIG = ROOT / "renovate.json"
RUNNER_PRESET_PREFIX = "github>WisewareOrg/wise-renovate"

if not RENOVATE_CONFIG.is_file():
    problems.append("renovate.json is missing at the repository root")
else:
    try:
        renovate_config = json.loads(RENOVATE_CONFIG.read_text(encoding="utf-8"))
    except json.JSONDecodeError as error:
        problems.append(f"renovate.json does not parse as JSON: {error}")
    else:
        extends = renovate_config.get("extends")
        preset_entries = [
            entry
            for entry in (extends if isinstance(extends, list) else [])
            if isinstance(entry, str)
            and (entry == RUNNER_PRESET_PREFIX or entry.startswith(RUNNER_PRESET_PREFIX + "#"))
        ]
        if not preset_entries:
            problems.append(
                f"renovate.json's 'extends' does not include the {RUNNER_PRESET_PREFIX} preset"
            )
        elif not any("#" in entry for entry in preset_entries):
            problems.append(
                f"renovate.json extends {RUNNER_PRESET_PREFIX} unpinned — no '#tag'"
            )

# No recipe carries a shebang, read from just's own dump. See docs/CI.md § Repository shape.
just_dump = json.loads(
    subprocess.run(
        ["just", "--dump", "--dump-format", "json"], cwd=ROOT, capture_output=True, check=True
    ).stdout
)
recipes = just_dump.get("recipes")
# A module's recipes sit outside `recipes`, so a script hidden in one would pass unseen. A dump
# shape this does not recognise is reported too: `--dump` is a debug surface, not a stable schema,
# and a missing field would otherwise read as an affirmative "no script here".
modules = just_dump.get("modules")
if not isinstance(modules, dict):
    problems.append(
        "the justfile dump carries no `modules` map, so this cannot tell whether a module is declared"
    )
elif modules:
    problems.append(
        f"the justfile declares module(s) {', '.join(modules)}, whose recipes this does not read"
    )
# Asserted before the loop, which passes vacuously over an empty set and would then agree that a
# justfile this could not read holds no script.
if not recipes:
    problems.append("the justfile dump names no recipe, so nothing here judged it")
for name, recipe in (recipes or {}).items():
    if not isinstance(recipe.get("shebang"), bool):
        problems.append(
            f"the dump for recipe '{name}' carries no `shebang` field, so this judged nothing about it"
        )
        continue
    if recipe["shebang"]:
        problems.append(
            f"justfile recipe '{name}' is a shell script — move it under scripts/ and call it from a one-line recipe"
        )

if problems:
    print(f"check-repo-silo: tooling is not siloed ({len(problems)}):", file=sys.stderr)
    for problem in problems:
        print("  " + problem, file=sys.stderr)
    sys.exit(1)
print(
    "Repository root holds no manifest, no justfile recipe is a shell script, and renovate.json "
    "extends the pinned WisewareOrg/wise-renovate preset."
)
