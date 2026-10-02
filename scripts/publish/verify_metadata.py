#!/usr/bin/env python3
"""Every declared OCI annotation level carries the nine keys, bound to the release commit.

Reads `DOCKER_METADATA_ANNOTATIONS_LEVELS`'s value from `.github/workflows/publish.yml`'s text
rather than restating the literal here, and maps each declared level through a closed table
(docs/CI.md § Publishing and provenance):

  index               -> the index manifest's own annotations
  manifest            -> each platform child's own manifest annotations, fetched by its digest
  manifest-descriptor -> each child's descriptor annotations, as recorded inside the index

Any other level fails. Every mapped surface must carry the nine `org.opencontainers.image.*` keys
non-empty, and `.revision` must equal the release commit on every surface. Labels are read from
each child's image config, via one `imagetools inspect --format '{{json .Image}}'` call on the
index digest, which returns a map keyed by platform string when the index has more than one
platform child.

Fail-closed: fails unless it inspected at least one level, at least one surface per level, and at
least one child config — an unrecognised level, an empty declaration, or an unreadable surface each
fail rather than being skipped.

Inputs are read from the environment rather than argv, matching this workflow's existing style for
passing run-time values into an invoked script:
  REF     the image reference, without a digest (e.g. ghcr.io/wisewareorg/wisekiosk)
  DIGEST  the index digest, sha256:<hex>
  COMMIT  the release commit, github.sha

The platform children are re-derived here rather than passed in, matching sbom_attest.py and
verify_release.py — each script computes them independently from the same index digest, per ADR
0017 rev 9: no shared step output built by a `run:` block loop or filter.

What this has been run against, in both directions: cases/publish-verify.md
"""

import os
import re
import sys
from pathlib import Path

from common import CheckError, imagetools_inspect, read_children

ROOT = Path(__file__).resolve().parent.parent.parent
WORKFLOW = ".github/workflows/publish.yml"

NINE_KEYS = [
    "org.opencontainers.image.title",
    "org.opencontainers.image.description",
    "org.opencontainers.image.url",
    "org.opencontainers.image.source",
    "org.opencontainers.image.version",
    "org.opencontainers.image.created",
    "org.opencontainers.image.revision",
    "org.opencontainers.image.licenses",
    "org.opencontainers.image.documentation",
]
REVISION_KEY = "org.opencontainers.image.revision"

LEVELS_LINE = re.compile(r"DOCKER_METADATA_ANNOTATIONS_LEVELS:\s*(\S+)")

problems = []
surfaces_checked = 0


def fail(message):
    problems.append(message)


def declared_levels():
    path = ROOT / WORKFLOW
    if not path.is_file():
        fail(f"{WORKFLOW} is absent, so this read no declaration")
        return []
    match = LEVELS_LINE.search(path.read_text(encoding="utf-8"))
    if not match:
        fail(f"{WORKFLOW}: no DOCKER_METADATA_ANNOTATIONS_LEVELS declaration was found")
        return []
    levels = [level.strip() for level in match.group(1).split(",") if level.strip()]
    if not levels:
        fail(f"{WORKFLOW}: DOCKER_METADATA_ANNOTATIONS_LEVELS declares no level")
    return levels


def check_surface(name, annotations, commit):
    global surfaces_checked
    if not annotations:
        fail(f"{name}: carries no annotations")
        return
    missing = [key for key in NINE_KEYS if not annotations.get(key)]
    if missing:
        fail(f"{name}: missing or empty key(s) {missing}")
        return
    revision = annotations[REVISION_KEY]
    if revision != commit:
        fail(f"{name}: '.revision' is {revision!r}, expected the release commit {commit!r}")
        return
    surfaces_checked += 1


def check_labels(ref, digest, children, commit):
    try:
        image = imagetools_inspect(ref, digest, format="{{json .Image}}")
    except CheckError as error:
        fail(f"labels: {error}")
        return 0
    if not isinstance(image, dict):
        fail(f"labels: {ref}@{digest}: unexpected .Image shape {type(image).__name__}")
        return 0

    if "config" in image:
        # A single real platform: .Image is the bare struct rather than a platform-keyed map.
        if len(children) != 1:
            fail(
                f"labels: {ref}@{digest}: .Image is a single config but the index has "
                f"{len(children)} platform child(ren)"
            )
            return 0
        entries = {children[0]["platform"]: image}
    else:
        entries = image

    child_checked = 0
    for child in children:
        entry = entries.get(child["platform"])
        if entry is None or "config" not in entry:
            fail(f"labels: {child['platform']} ({child['digest']}): no config found in .Image")
            continue
        labels = entry["config"].get("Labels") or {}
        missing = [key for key in NINE_KEYS if not labels.get(key)]
        if missing:
            fail(f"labels {child['platform']} ({child['digest']}): missing or empty key(s) {missing}")
            continue
        revision = labels[REVISION_KEY]
        if revision != commit:
            fail(
                f"labels {child['platform']} ({child['digest']}): '.revision' is {revision!r}, "
                f"expected the release commit {commit!r}"
            )
            continue
        child_checked += 1

    if child_checked == 0:
        fail("labels: no child config was checked")
    return child_checked


def main():
    ref = os.environ["REF"]
    digest = os.environ["DIGEST"]
    commit = os.environ["COMMIT"]

    try:
        children = read_children(ref, digest)
    except CheckError as error:
        fail(str(error))
        children = []
    if not children:
        fail(f"the pushed index {ref}@{digest} names no platform child")

    levels = declared_levels()

    index_raw_cache = {}

    def index_raw():
        if digest not in index_raw_cache:
            try:
                index_raw_cache[digest] = imagetools_inspect(ref, digest, raw=True)
            except CheckError as error:
                fail(str(error))
                index_raw_cache[digest] = None
        return index_raw_cache[digest]

    for level in levels:
        if level == "index":
            raw = index_raw()
            if raw is not None:
                check_surface(f"index {digest}", raw.get("annotations"), commit)
        elif level == "manifest":
            for child in children:
                try:
                    raw = imagetools_inspect(ref, child["digest"], raw=True)
                except CheckError as error:
                    fail(str(error))
                    continue
                check_surface(f"manifest {child['digest']}", raw.get("annotations"), commit)
        elif level == "manifest-descriptor":
            raw = index_raw()
            if raw is None:
                continue
            descriptors = {entry.get("digest"): entry for entry in raw.get("manifests", [])}
            for child in children:
                descriptor = descriptors.get(child["digest"])
                if descriptor is None:
                    fail(f"manifest-descriptor {child['digest']}: not found among the index's manifests")
                    continue
                check_surface(
                    f"manifest-descriptor {child['digest']}", descriptor.get("annotations"), commit
                )
        else:
            fail(f"{WORKFLOW}: unrecognised annotation level {level!r}")

    if levels and surfaces_checked == 0:
        fail("no annotation surface was successfully checked")

    child_configs_checked = check_labels(ref, digest, children, commit) if children else 0

    if problems:
        print(f"verify_metadata: {len(problems)} problem(s):", file=sys.stderr)
        for problem in problems:
            print("  " + problem, file=sys.stderr)
        return 1

    print(
        f"{len(levels)} annotation level(s), {surfaces_checked} surface(s), "
        f"{child_configs_checked} child config(s): the nine keys are present and bound to {commit}."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
