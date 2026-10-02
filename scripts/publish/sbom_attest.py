#!/usr/bin/env python3
"""Generate and attest one SPDX SBOM per platform child of the pushed release image.

Reads the pushed index's platform children rather than a hardcoded pair, so a change to the
build's platform matrix needs no change here. Per child: the pinned syft scans the image at that
child's own digest and platform into an SPDX-JSON file, then `cosign attest` binds that file to the
child as an SPDX attestation (docs/CI.md § Publishing and provenance). The index itself is signed by
a separate `cosign sign --recursive` workflow step, which needs no per-child loop.

This is authored Python rather than a `run:` block with a loop, per ADR 0017 rev 9: a workflow
`run:` block carrying control flow is authored sh, and sh authors nothing here.

Inputs, read from the environment (matching this workflow's existing style for passing run-time
values into an invoked script):
  REF    the image reference, without a digest (e.g. ghcr.io/wisewareorg/wisekiosk)
  DIGEST the pushed index digest, sha256:<hex>
  SYFT   the path to the pinned syft binary, `download-syft`'s `cmd` output

`cosign` is invoked by bare name, on the PATH `cosign-installer` provides.

What this has been run against, in both directions: cases/publish-verify.md
"""

import os
import subprocess
import sys

from common import CheckError, read_children


def run(cmd):
    result = subprocess.run(cmd)
    if result.returncode != 0:
        print(f"sbom_attest: command failed ({result.returncode}): {' '.join(cmd)}", file=sys.stderr)
        sys.exit(1)


def main():
    ref = os.environ["REF"]
    digest = os.environ["DIGEST"]
    syft = os.environ["SYFT"]

    try:
        children = read_children(ref, digest)
    except CheckError as error:
        print(f"sbom_attest: {error}", file=sys.stderr)
        return 1
    if not children:
        print(f"sbom_attest: the pushed index {ref}@{digest} names no platform child", file=sys.stderr)
        return 1

    for child in children:
        arch = child["platform"].split("/", 1)[1]
        sbom_path = f"sbom-{arch}.spdx.json"
        run([syft, "scan", f"{ref}@{child['digest']}", "--platform", child["platform"],
             "-o", f"spdx-json={sbom_path}"])
        run(["cosign", "attest", "--yes", "--type", "spdxjson", "--predicate", sbom_path,
             f"{ref}@{child['digest']}"])

    print(f"attested {len(children)} platform child(ren): " + ", ".join(c["platform"] for c in children))
    return 0


if __name__ == "__main__":
    sys.exit(main())
