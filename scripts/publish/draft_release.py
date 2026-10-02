#!/usr/bin/env python3
"""Create or reuse a draft release for one tag, so `publish.yml` can attach assets and amend notes
before anything is immutable.

GitHub locks a release's assets and tag the moment it publishes (ADR 0020 rev 4): a workflow
triggered by the publish of a release can therefore never upload to the release that triggered it.
The fix is draft-first — `publish.yml` creates or reuses this tag's draft release before any
registry write, fills it over its own later steps, then publishes it last. This script owns only
the draft's existence and its `prerelease` flag.

Looks the tag up in the full release list — drafts included, for a token with write access —
rather than inferring absence from a `gh release view` error, so "absent" is never a guess from a
failed read:

  no release carries the tag       create a draft, `--generate-notes`, titled the tag
  exactly one release, a draft     reuse it, re-syncing `--prerelease` to this run's value
  exactly one release, published   exit 1: already immutable, nothing left to do here
  more than one release            exit 1: ambiguous, resolved by hand

Inputs, read from the environment (matching this workflow's existing style for passing run-time
values into an invoked script):
  TAG         the tag this release is cut on, e.g. v1.2.3
  PRERELEASE  'true' or 'false'

`gh` reads `GH_TOKEN`/`GH_REPO` itself, as it is run by hand elsewhere in this tree. Any `gh`
failure is surfaced as `gh`'s own error text, unaltered — no text of this script's own authorship
stands in for it.

This is authored Python rather than a `run:` block with branching, per ADR 0017 rev 9: a workflow
`run:` block carrying control flow is authored sh, and sh authors nothing here.

What this has been run against, in both directions: cases/publish-draft.md
"""

import json
import os
import subprocess
import sys


def run(cmd):
    return subprocess.run(cmd, capture_output=True, text=True)


def fail(result):
    print((result.stderr or result.stdout).strip(), file=sys.stderr)
    sys.exit(result.returncode or 1)


def main():
    tag = os.environ["TAG"]
    prerelease = os.environ["PRERELEASE"]

    lookup = run(["gh", "api", "--paginate", "repos/{owner}/{repo}/releases"])
    if lookup.returncode != 0:
        fail(lookup)
    try:
        releases = json.loads(lookup.stdout)
    except json.JSONDecodeError as error:
        print(f"draft_release: `gh api --paginate repos/{{owner}}/{{repo}}/releases` output did "
              f"not parse as JSON: {error}", file=sys.stderr)
        return 1

    matches = [release for release in releases if release.get("tag_name") == tag]

    if len(matches) > 1:
        print(f"more than one release carries tag {tag}; resolve by hand", file=sys.stderr)
        return 1

    if not matches:
        create = run(["gh", "release", "create", tag, "--draft", "--verify-tag",
                      "--title", tag, "--generate-notes", f"--prerelease={prerelease}"])
        if create.returncode != 0:
            fail(create)
        print(f"draft release {tag} created, prerelease={prerelease}")
        return 0

    if not matches[0].get("draft"):
        print(f"release {tag} is already published and immutable; re-cut under a new version",
              file=sys.stderr)
        return 1

    edit = run(["gh", "release", "edit", tag, f"--prerelease={prerelease}"])
    if edit.returncode != 0:
        fail(edit)
    print(f"draft release {tag} reused, prerelease={prerelease}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
