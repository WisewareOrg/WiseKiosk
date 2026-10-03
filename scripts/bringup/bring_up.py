#!/usr/bin/env python3
"""The documented bring-up procedure reaches a serving deployment from a release's own files.

docs/DEPLOYMENT.md § Bring-up: the procedure is three commands, run in the directory holding the
release's two assets. This runs the first `sh` fence after the doc's `## Bring-up` heading,
unedited, inside `<release-dir>`, which already holds `compose.yaml` and `config.example.json`.
`WISEKIOSK_IMAGE` is exported to the block, so `deploy/compose.yaml`'s
`image: ${WISEKIOSK_IMAGE:-ghcr.io/wisewareorg/wisekiosk:latest}` resolves to this release's
digest rather than the default tag (ADR 0020 rev 5).

Serving is three assertions, all required (docs/CI.md § Deployment and bring-up): the running
container's own image resolves to `<digest>` — what the documented procedure brought up is this
release's image, however the recipe names it — the compose service's container reaches Docker
health status `healthy` within a deadline derived from the image's own declared healthcheck
(interval, retries and start period substituted with Docker's defaults where the image declares
none), and `GET /config.json` returns `<release-dir>`'s own `config.example.json` byte for byte —
the assertion health status alone cannot make, since `/healthz` is configuration-blind by design.

Usage: bring_up.py [--doc PATH] <release-dir> <digest>
"""

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path

import common

ROOT = Path(__file__).resolve().parent.parent.parent
DEFAULT_DOC = ROOT / "docs" / "DEPLOYMENT.md"

SERVICE = "kiosk"

HEADING = re.compile(r"^##\s+Bring-up\s*$")
FENCE_OPEN = re.compile(r"^```sh\s*$")
FENCE_CLOSE = re.compile(r"^```\s*$")


def fail(problem):
    print(f"bring-up: {problem}", file=sys.stderr)
    return 1


def extract_block(doc_path):
    """The first `sh` fence after the `## Bring-up` heading, as its literal lines."""
    lines = doc_path.read_text(encoding="utf-8").split("\n")
    heading_at = next((i for i, line in enumerate(lines) if HEADING.match(line)), None)
    if heading_at is None:
        raise common.HarnessError(f"{doc_path} carries no '## Bring-up' heading")
    fence_at = next(
        (i for i in range(heading_at + 1, len(lines)) if FENCE_OPEN.match(lines[i])), None
    )
    if fence_at is None:
        raise common.HarnessError(f"{doc_path} carries no 'sh' fence after '## Bring-up'")
    for end in range(fence_at + 1, len(lines)):
        if FENCE_CLOSE.match(lines[end]):
            return lines[fence_at + 1 : end]
    raise common.HarnessError(f"{doc_path}'s bring-up fence never closes")


def run_block(block, directory, env):
    """Every line of the documented procedure, unedited, through `sh -c`."""
    for line in block:
        completed = subprocess.run(["sh", "-c", line], cwd=directory, env=env)
        if completed.returncode != 0:
            raise common.HarnessError(
                f"`{line}` exited {completed.returncode} — the documented procedure did not "
                f"complete"
            )


def compose(*arguments, directory):
    return subprocess.run(
        ["docker", "compose", *arguments], cwd=directory, capture_output=True, text=True
    )


def compose_container(directory):
    completed = compose("ps", "-q", SERVICE, directory=directory)
    container = completed.stdout.strip().split("\n")[0] if completed.stdout.strip() else ""
    if completed.returncode != 0 or not container:
        raise common.HarnessError(
            f"`docker compose ps -q {SERVICE}` found no container ({completed.stderr.strip()})"
        )
    return container


def compose_address(directory):
    completed = compose("port", SERVICE, "8080", directory=directory)
    if completed.returncode != 0 or not completed.stdout.strip():
        raise common.HarnessError(
            f"`docker compose port {SERVICE} 8080` exited {completed.returncode} "
            f"({completed.stderr.strip()})"
        )
    return completed.stdout.strip().split("\n")[0]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--doc", type=Path, default=DEFAULT_DOC)
    parser.add_argument("release_dir", type=Path)
    parser.add_argument("digest")
    args = parser.parse_args()

    try:
        block = extract_block(args.doc)
        image_ref = f"{common.IMAGE}@{args.digest}"
        env = {**os.environ, "WISEKIOSK_IMAGE": image_ref}
        run_block(block, args.release_dir, env)
        container = compose_container(args.release_dir)
        common.assert_running_image(container, image_ref)
        common.wait_healthy(container, common.healthcheck_deadline(image_ref))
        address = compose_address(args.release_dir)
        example = (args.release_dir / "config.example.json").read_bytes()
        common.assert_config_served(address, example)
    except common.HarnessError as error:
        return fail(str(error))

    print(f"the documented bring-up procedure at {args.digest} reaches a serving deployment")
    return 0


if __name__ == "__main__":
    sys.exit(main())
