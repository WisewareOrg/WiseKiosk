#!/usr/bin/env python3
"""Every service in the committed deployment recipe declares restart policy `unless-stopped`.

The value is the assertion rather than the key's presence: `unless-stopped` and not `always`,
because the latter also overrides a deliberate manual stop (docs/DEPLOYMENT.md § The deployment
recipe, ADR 0020 rev 5).

That one key and no other. Every other value in the recipe is a sample default an operator is
expected to weigh and change, so gating one would assert a recommendation as an obligation —
docs/CI.md § Deployment and bring-up, which is also why this is not a recipe linter.

No dependencies: Python stdlib only, plain text scanning (no YAML parser), matching
scripts/check-repo-silo.py's idiom.

What this has been run against, in both directions: cases/check-restart-policy-py.md
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
RECIPE = "deploy/compose.yaml"
POLICY = "unless-stopped"

# A mapping key and its indentation. A list item, a blank line, a comment and a continuation
# all miss.
KEY = re.compile(r"^(?P<indent>[^\S\n]*)(?P<name>[A-Za-z0-9_.-]+):(?P<value>[^\S\n].*)?$")
# Read for the guard below rather than by the walk, and deliberately sharing no assumption with it:
# a layout the walk cannot follow yields no service, and a check that examined nothing would
# otherwise print success.
RESTART = re.compile(r"^[^\S\n]*restart:")

problems = []


def scalar(text):
    """A scalar as written, without a trailing comment or surrounding quotes."""
    value = re.sub(r"[^\S\n]#.*$", "", text or "").strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        value = value[1:-1]
    return value


def mapping_keys(lines):
    """Every mapping key in the recipe, as (line number, indent, name, value)."""
    found = []
    for number, line in enumerate(lines, start=1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        match = KEY.match(line)
        if match:
            found.append(
                (number, len(match.group("indent")), match.group("name"), match.group("value"))
            )
    return found


def block(keys, index):
    """The keys nested under keys[index], up to the next key at or above its indent."""
    indent = keys[index][1]
    nested = []
    for entry in keys[index + 1 :]:
        if entry[1] <= indent:
            break
        nested.append(entry)
    return nested


path = ROOT / RECIPE
if not path.is_file():
    problems.append(f"{RECIPE} is absent, so this read no recipe")
    keys = []
    lines = []
else:
    lines = path.read_text(encoding="utf-8").split("\n")
    keys = mapping_keys(lines)

services = [index for index, entry in enumerate(keys) if entry[1] == 0 and entry[2] == "services"]
if path.is_file() and not services:
    problems.append(
        f"{RECIPE} declares no top-level 'services' key, or check-restart-policy.py cannot read "
        f"this layout — either way no service was examined"
    )

checked = 0
reached = set()
for index in services:
    nested = block(keys, index)
    if not nested:
        problems.append(f"{RECIPE} declares 'services' with nothing under it")
        continue
    depth = min(entry[1] for entry in nested)
    for offset, entry in enumerate(nested):
        if entry[1] != depth:
            continue
        name = entry[2]
        checked += 1
        # The service's own keys, not everything beneath it: a `restart` nested deeper belongs to
        # some key the service contains, and reading it as the service's policy would pass a recipe
        # that declares none. It is reported by the guard below instead.
        own = block(nested, offset)
        depth_own = min((item[1] for item in own), default=None)
        declared = [item for item in own if item[1] == depth_own and item[2] == "restart"]
        reached.update(item[0] for item in declared)
        if not declared:
            problems.append(
                f"{RECIPE}: service '{name}' declares no restart policy, so a deployment stopped "
                f"by a host reboot stays stopped"
            )
            continue
        for item in declared:
            value = scalar(item[3])
            if value != POLICY:
                problems.append(
                    f"{RECIPE}:{item[0]}: service '{name}' declares restart '{value}', "
                    f"expected '{POLICY}'"
                )

# Every restart key in the file was reached by the walk above. One that was not sits where this
# read no service's policy, and the walk's verdict says nothing about it.
for number, line in enumerate(lines, start=1):
    if line.lstrip().startswith("#") or not RESTART.match(line):
        continue
    if number not in reached:
        problems.append(
            f"{RECIPE}:{number}: a 'restart' key sits outside every service's own keys, so nothing "
            f"here judged it"
        )

if problems:
    print(f"check-restart-policy: the deployment recipe ({len(problems)}):", file=sys.stderr)
    for problem in problems:
        print("  " + problem, file=sys.stderr)
    sys.exit(1)
print(f"{checked} service(s) in {RECIPE} declare restart '{POLICY}'.")
