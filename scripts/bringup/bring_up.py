#!/usr/bin/env python3
"""The documented bring-up procedure reaches a serving deployment from a published release.

docs/DEPLOYMENT.md § Bring-up: the procedure is three commands, run in the directory holding the
release's two assets. This runs the first `sh` fence after the doc's `## Bring-up` heading,
unedited, in a fixed working directory (`bring-up/` at the repository root) holding the two assets
`gh release download` fetches for the given release tag.

Before the block runs, `ghcr.io/wisewareorg/wisekiosk:latest` is asserted to resolve to the given
digest — the one assertion in the tree that `latest` moved to the release this run was handed, and
the poll that waits out registry propagation.

Serving is two assertions, both required (docs/CI.md § Deployment and bring-up): the compose
service's container reaches Docker health status `healthy` within a deadline derived from the
image's own declared healthcheck (interval, retries and start period substituted with Docker's
defaults where the image declares none), and `GET /config.json` returns the downloaded
`config.example.json` byte for byte — the assertion health status alone cannot make, since
`/healthz` is configuration-blind by design.

Usage: bring_up.py [--doc PATH] <tag> <digest>
"""

import argparse
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path

import common

ROOT = Path(__file__).resolve().parent.parent.parent
DEFAULT_DOC = ROOT / "docs" / "DEPLOYMENT.md"
WORKDIR = ROOT / "bring-up"

SERVICE = "kiosk"

HEADING = re.compile(r"^##\s+Bring-up\s*$")
FENCE_OPEN = re.compile(r"^```sh\s*$")
FENCE_CLOSE = re.compile(r"^```\s*$")

# How long `latest` is given to resolve to the digest this run was handed, and how often it is
# asked — the harness's own constant (docs/CI.md § Deployment and bring-up).
PROPAGATION_TIMEOUT = 60.0
PROPAGATION_INTERVAL = 1.0


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


def materialise(directory):
    """A fixed, empty working directory — what an operator starts a bring-up from."""
    shutil.rmtree(directory, ignore_errors=True)
    directory.mkdir()


def download_assets(tag, directory):
    completed = subprocess.run(
        [
            "gh",
            "release",
            "download",
            tag,
            "--pattern",
            "compose.yaml",
            "--pattern",
            "config.example.json",
            "--dir",
            str(directory),
        ],
        capture_output=True,
        text=True,
    )
    if completed.returncode != 0:
        raise common.HarnessError(
            f"`gh release download {tag}` exited {completed.returncode} "
            f"({completed.stderr.strip()})"
        )


def resolved_latest_digest():
    """`latest`'s current manifest digest, or None with the detail on a failed resolution."""
    return common.imagetools_inspect(f"{common.IMAGE}:latest", ".Manifest.Digest")


def assert_latest_propagated(expected_digest):
    """`latest` resolves to the digest this run was handed, within the poll bound."""
    deadline = time.monotonic() + PROPAGATION_TIMEOUT
    seen, detail = None, None
    while time.monotonic() < deadline:
        seen, detail = resolved_latest_digest()
        if seen == expected_digest:
            return
        time.sleep(PROPAGATION_INTERVAL)
    reason = f" — {detail}" if detail else ""
    raise common.HarnessError(
        f"{common.IMAGE}:latest resolved to {seen!r} after {PROPAGATION_TIMEOUT:.0f}s, expected "
        f"{expected_digest}{reason} — latest has not propagated to the digest this release "
        f"published"
    )


def run_block(block, directory):
    """Every line of the documented procedure, unedited, through `sh -c`."""
    for line in block:
        completed = subprocess.run(["sh", "-c", line], cwd=directory)
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
    parser.add_argument("tag")
    parser.add_argument("digest")
    args = parser.parse_args()

    try:
        block = extract_block(args.doc)
        materialise(WORKDIR)
        download_assets(args.tag, WORKDIR)
        assert_latest_propagated(args.digest)
        run_block(block, WORKDIR)
        container = compose_container(WORKDIR)
        image_ref = f"{common.IMAGE}@{args.digest}"
        common.wait_healthy(container, common.healthcheck_deadline(image_ref))
        address = compose_address(WORKDIR)
        example = (WORKDIR / "config.example.json").read_bytes()
        common.assert_config_served(address, example)
    except common.HarnessError as error:
        return fail(str(error))

    print(f"the documented bring-up procedure at {args.tag} reaches a serving deployment")
    return 0


if __name__ == "__main__":
    sys.exit(main())
