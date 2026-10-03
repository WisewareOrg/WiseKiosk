#!/usr/bin/env python3
"""The previous release and the release under test answer the same mount arguments, one after the
other.

docs/CI.md § Deployment and bring-up: the previous digest runs under a mounted configuration and an
ephemeral published port; is asserted healthy, serving that configuration, and reporting its own
version; is stopped and removed; the digest under test runs under byte-identical mount arguments and
is asserted the same way — with no builder invoked at either step. Version is read from the OCI
`org.opencontainers.image.version` annotation on the manifest each digest names, and the two
versions must differ, each equalling its own release tag with the leading `v` stripped. The
previous side's configuration comes from `gh release download`; the side under test's comes from
`<release-dir>` directly, which lets this check run against a release still only a draft, with no
published tag to download from.

Usage: image_swap.py <previous-tag> <previous-digest> <tag> <release-dir> <digest>
"""

import argparse
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

import common

MOUNT_TARGET = "/srv/kiosk/config.json"
VERSION_ANNOTATION = "org.opencontainers.image.version"


def fail(problem):
    print(f"image-swap: {problem}", file=sys.stderr)
    return 1


def download_config(tag, directory):
    completed = subprocess.run(
        [
            "gh",
            "release",
            "download",
            tag,
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


def start_container(image_ref, config_path):
    completed = subprocess.run(
        [
            "docker",
            "run",
            "--detach",
            "--pull",
            "always",
            "--volume",
            f"{config_path}:{MOUNT_TARGET}:ro",
            "--publish",
            "127.0.0.1::8080",
            image_ref,
        ],
        capture_output=True,
        text=True,
    )
    if completed.returncode != 0:
        raise common.HarnessError(
            f"`docker run {image_ref}` exited {completed.returncode} ({completed.stderr.strip()})"
        )
    return completed.stdout.strip()


def container_address(container):
    completed = subprocess.run(
        ["docker", "port", container, "8080"], capture_output=True, text=True
    )
    if completed.returncode != 0 or not completed.stdout.strip():
        raise common.HarnessError(
            f"`docker port {container} 8080` exited {completed.returncode} "
            f"({completed.stderr.strip()})"
        )
    return completed.stdout.strip().split("\n")[0]


def resolve_version(image_ref):
    """The `org.opencontainers.image.version` annotation on the manifest image_ref names."""
    annotations, detail = common.imagetools_inspect(image_ref, ".Manifest.Annotations")
    if annotations is None:
        raise common.HarnessError(
            f"`docker buildx imagetools inspect {image_ref}` failed ({detail})"
        )
    version = (annotations or {}).get(VERSION_ANNOTATION)
    if not version:
        raise common.HarnessError(
            f"{image_ref}'s manifest carries no {VERSION_ANNOTATION} annotation"
        )
    return version


def run_and_assert(tag, digest, release_dir=None):
    """Runs image@digest under the shared mount, asserts it, tears it down, returns its version.

    The mounted configuration is `release_dir`'s own `config.example.json` when given, otherwise
    `gh release download`'s, against `tag`.
    """
    image_ref = f"{common.IMAGE}@{digest}"
    with tempfile.TemporaryDirectory() as directory:
        directory = Path(directory)
        if release_dir is None:
            download_config(tag, directory)
            example_path = directory / "config.example.json"
        else:
            example_path = Path(release_dir) / "config.example.json"
        config_path = directory / "config.json"
        shutil.copy(example_path, config_path)
        container = None
        try:
            container = start_container(image_ref, config_path)
            common.assert_running_image(container, image_ref)
            common.wait_healthy(container, common.healthcheck_deadline(image_ref))
            address = container_address(container)
            common.assert_config_served(address, config_path.read_bytes())
            version = resolve_version(image_ref)
            expected_version = tag[1:] if tag.startswith("v") else tag
            if version != expected_version:
                raise common.HarnessError(
                    f"{image_ref} reports version {version!r}, expected {expected_version!r} "
                    f"from tag {tag}"
                )
            return version
        finally:
            if container is not None:
                subprocess.run(["docker", "stop", container], capture_output=True, text=True)
                subprocess.run(["docker", "rm", container], capture_output=True, text=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("previous_tag")
    parser.add_argument("previous_digest")
    parser.add_argument("tag")
    parser.add_argument("release_dir")
    parser.add_argument("digest")
    args = parser.parse_args()

    try:
        version_a = run_and_assert(args.previous_tag, args.previous_digest)
        version_b = run_and_assert(args.tag, args.digest, release_dir=args.release_dir)
        if version_a == version_b:
            raise common.HarnessError(
                f"{args.previous_tag} and {args.tag} both report version {version_a!r} — the swap "
                f"changed nothing"
            )
    except common.HarnessError as error:
        return fail(str(error))

    print(
        f"{args.previous_tag} ({version_a}) and {args.tag} ({version_b}) each serve their mounted "
        f"configuration under the same mount arguments, with no builder invoked"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
