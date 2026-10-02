#!/usr/bin/env python3
"""Verify a published release's signature, provenance, SBOM, or attached artifact set.

One `--step` per invocation, so the `verify` job keeps one workflow step per check, each a single
command whose failure text comes from here and stays unique to its cause
(docs/CI.md § Publishing and provenance):

  signature   cosign verify on the index and each platform child, plus a negative identity
  provenance  gh attestation verify against GitHub's attestation store and against the registry
              copy of the same bundle, plus two negatives
  sbom        cosign verify-attestation per child, asserting exactly one SPDX attestation exists,
              schema-validated, content-asserted against backend/go.mod and the Dockerfile,
              regenerated and compared, and bound to the child under verification
  attached    cosign tree on the index and each child, and the release's asset list and notes

This is authored Python rather than `run:` blocks with loops and multi-field jq assertions, per
ADR 0017 rev 9: a workflow `run:` block carrying control flow is authored sh, and sh authors
nothing here. Each per-child loop and every content assertion below is a plain Python `for`/`if`,
not shell.

Inputs, read from the environment (matching this workflow's existing style for passing run-time
values into an invoked script) — only what each `--step` needs:
  REF     the image reference, without a digest (e.g. ghcr.io/wisewareorg/wisekiosk); every step
  DIGEST  the index digest, sha256:<hex>; every step
  COMMIT  the release commit, github.sha; --step provenance only
  TAG     the release tag, github.event.release.tag_name; --step attached only
  SYFT    the path to the pinned syft binary, `download-syft`'s `cmd` output; --step sbom only

`cosign` and `gh` are invoked by bare name, on the PATH the workflow's setup steps provide. The
platform children are re-derived independently in each step by `read_children`, matching
sbom_attest.py and verify_metadata.py, rather than shared through a `run:` block's own output.

What this has been run against, in both directions: cases/publish-verify.md
"""

import argparse
import base64
import json
import os
import re
import subprocess
import sys
from pathlib import Path

from common import CheckError, read_children

ROOT = Path(__file__).resolve().parent.parent.parent
SCHEMA_PATH = Path(__file__).resolve().parent / "spdx-schema-2.3.json"

# Pinned separately from the syft/cosign action inputs because this invocation lives in Python
# rather than in publish.yml; the Renovate customManagers entry for it points at this file and
# matches this literal spec string.
CHECK_JSONSCHEMA_PIN = "check-jsonschema==0.38.2"

IDENTITY_REGEXP = r"^https://github\.com/WisewareOrg/WiseKiosk/\.github/workflows/publish\.yml@refs/tags/v"
OIDC_ISSUER = "https://token.actions.githubusercontent.com"
SIGNER_WORKFLOW = "WisewareOrg/WiseKiosk/.github/workflows/publish.yml"
GH_REPO = "WisewareOrg/WiseKiosk"


def fail(problems, message):
    problems.append(message)


def run(cmd, **kwargs):
    return subprocess.run(cmd, capture_output=True, text=True, **kwargs)


# --- signature ----------------------------------------------------------------------------------

def cosign_verify(target, identity_regexp):
    result = run(["cosign", "verify", "--certificate-identity-regexp", identity_regexp,
                  "--certificate-oidc-issuer", OIDC_ISSUER, target])
    return result.returncode, result.stdout + result.stderr


def step_signature(ref, digest, problems):
    try:
        children = read_children(ref, digest)
    except CheckError as error:
        fail(problems, str(error))
        return
    if not children:
        fail(problems, "signature: no platform child to check")

    targets = [f"{ref}@{digest}"] + [f"{ref}@{child['digest']}" for child in children]
    for target in targets:
        returncode, output = cosign_verify(target, IDENTITY_REGEXP)
        if returncode != 0:
            fail(problems, f"signature: cosign verify {target} did not verify: {output.strip()}")

    returncode, output = cosign_verify(f"{ref}@{digest}", r"^https://example\.invalid/")
    if returncode == 0:
        fail(problems, "signature: cosign verify accepted a wrong identity regexp, expected refusal")
    elif "none of the expected identities matched" not in output:
        fail(problems, f"signature: refused for an unexpected reason: {output.strip()}")


# --- provenance ---------------------------------------------------------------------------------

def gh_attestation_verify(target, signer_workflow, bundle_from_oci=False, as_json=False):
    cmd = ["gh", "attestation", "verify", target, "--repo", GH_REPO,
           "--signer-workflow", signer_workflow]
    if bundle_from_oci:
        cmd.append("--bundle-from-oci")
    if as_json:
        cmd.extend(["--format", "json"])
    result = run(cmd)
    return result.returncode, result.stdout, result.stderr


def check_provenance_fields(entries, digest, commit, problems):
    if len(entries) != 1:
        fail(problems, f"provenance: expected exactly one attestation entry, got {len(entries)}")
        return
    verification = entries[0].get("verificationResult", {})
    statement = verification.get("statement", {})
    certificate = verification.get("signature", {}).get("certificate", {})

    digest_hex = digest.split(":", 1)[1]
    subject = statement.get("subject") or [{}]
    subject_digest = subject[0].get("digest", {}).get("sha256")
    if subject_digest != digest_hex:
        fail(problems, f"provenance: subject digest {subject_digest!r} does not match the index digest {digest_hex!r}")

    predicate_type = statement.get("predicateType")
    if predicate_type != "https://slsa.dev/provenance/v1":
        fail(problems, f"provenance: predicateType {predicate_type!r} is not 'https://slsa.dev/provenance/v1'")

    build_signer_uri = certificate.get("buildSignerURI", "")
    if not re.match(IDENTITY_REGEXP, build_signer_uri):
        fail(problems, f"provenance: buildSignerURI {build_signer_uri!r} does not match {IDENTITY_REGEXP}")

    source_repository_uri = certificate.get("sourceRepositoryURI")
    if source_repository_uri != "https://github.com/WisewareOrg/WiseKiosk":
        fail(problems, f"provenance: sourceRepositoryURI {source_repository_uri!r} is not "
                       f"'https://github.com/WisewareOrg/WiseKiosk'")

    source_repository_digest = certificate.get("sourceRepositoryDigest")
    if source_repository_digest != commit:
        fail(problems, f"provenance: sourceRepositoryDigest {source_repository_digest!r} does not "
                       f"match the release commit {commit!r}")


# The digest is interpolated into the message this substring is drawn from.
MANIFEST_UNKNOWN_TEXT = "MANIFEST_UNKNOWN: manifest unknown"


def verify_negative_manifest_unknown(problems, label, target, signer_workflow):
    returncode, stdout, stderr = gh_attestation_verify(target, signer_workflow)
    combined = stdout + stderr
    if returncode == 0:
        fail(problems, f"provenance: {label}: gh attestation verify accepted, expected refusal")
    elif MANIFEST_UNKNOWN_TEXT not in combined:
        fail(problems, f"provenance: {label}: refused for an unexpected reason: {combined.strip()}")


# Asserts only a non-zero exit and an empty stdout — no message literal.
def verify_negative_no_json_success(problems, label, target, signer_workflow):
    returncode, stdout, stderr = gh_attestation_verify(target, signer_workflow)
    if returncode == 0:
        fail(problems, f"provenance: {label}: gh attestation verify accepted, expected refusal")
    elif stdout.strip():
        fail(problems, f"provenance: {label}: refused but printed to stdout: {stdout.strip()}")


def step_provenance(ref, digest, commit, problems):
    target = f"oci://{ref}@{digest}"

    returncode, stdout, stderr = gh_attestation_verify(target, SIGNER_WORKFLOW, as_json=True)
    if returncode != 0:
        fail(problems, f"provenance: gh attestation verify failed: {(stderr or stdout).strip()}")
    else:
        try:
            entries = json.loads(stdout)
        except json.JSONDecodeError as error:
            fail(problems, f"provenance: --format json output did not parse as JSON: {error}")
        else:
            check_provenance_fields(entries, digest, commit, problems)

    verify_negative_no_json_success(
        problems, "wrong signer workflow", target,
        "WisewareOrg/WiseKiosk/.github/workflows/checks.yml",
    )
    flipped_hex = digest.split(":", 1)[1][::-1]
    verify_negative_manifest_unknown(
        problems, "flipped digest", f"oci://{ref}@sha256:{flipped_hex}", SIGNER_WORKFLOW,
    )

    returncode, stdout, stderr = gh_attestation_verify(target, SIGNER_WORKFLOW, bundle_from_oci=True, as_json=True)
    if returncode != 0:
        fail(problems, f"provenance: the registry copy of the provenance bundle (--bundle-from-oci) "
                       f"did not verify: {(stderr or stdout).strip()}")


# --- sbom ----------------------------------------------------------------------------------

def purl_qualifier(purl, name):
    match = re.search(rf"[?&]{re.escape(name)}=([^&]+)", purl or "")
    return match.group(1) if match else None


def package_purl(package):
    for entry in package.get("externalRefs", []):
        if entry.get("referenceType") == "purl":
            return entry.get("referenceLocator")
    return None


def extract_predicate(envelope_line):
    envelope = json.loads(envelope_line)
    payload = envelope["payload"]
    padded = payload + "=" * (-len(payload) % 4)
    statement = json.loads(base64.b64decode(padded))
    return statement["predicate"]


def go_module_name():
    text = (ROOT / "backend" / "go.mod").read_text(encoding="utf-8")
    match = re.search(r"^module\s+(\S+)", text, re.MULTILINE)
    return match.group(1) if match else None


def dockerfile_final_from():
    text = (ROOT / "Dockerfile").read_text(encoding="utf-8")
    froms = re.findall(r"^FROM\s+(\S+)", text, re.MULTILINE)
    if not froms:
        return None, None
    image_tag = froms[-1].split("@", 1)[0]
    if ":" not in image_tag:
        return image_tag, None
    name, tag = image_tag.split(":", 1)
    return name, tag


def check_sbom_content(platform, predicate, child, go_module, base_name, base_tag, problems):
    packages = predicate.get("packages", [])

    if not any(package.get("name") == go_module for package in packages):
        fail(problems, f"sbom {platform}: no package named {go_module!r} (backend/go.mod's module line)")

    distros = set()
    for package in packages:
        distro = purl_qualifier(package_purl(package), "distro")
        if distro:
            distros.add(distro)
    if not distros:
        fail(problems, f"sbom {platform}: no package carries a purl 'distro=' qualifier")
    elif len(distros) != 1:
        fail(problems, f"sbom {platform}: purl 'distro=' qualifiers disagree: {sorted(distros)}")
    else:
        distro = next(iter(distros))
        distro_name, _, distro_version = distro.partition("-")
        if distro_name != base_name:
            fail(problems, f"sbom {platform}: distro name {distro_name!r} does not match the "
                           f"Dockerfile's final FROM image {base_name!r}")
        elif distro_version != base_tag and not distro_version.startswith(f"{base_tag}."):
            fail(problems, f"sbom {platform}: distro version {distro_version!r} does not carry the "
                           f"Dockerfile's final FROM tag {base_tag!r} as a dotted-component prefix")

    describing = next(
        (package for package in packages if package.get("primaryPackagePurpose") == "CONTAINER"), None
    )
    if describing is None:
        fail(problems, f"sbom {platform}: no describing image package (primaryPackagePurpose CONTAINER)")
        return
    if describing.get("versionInfo") != child["digest"]:
        fail(problems, f"sbom {platform}: describing package versionInfo "
                       f"{describing.get('versionInfo')!r} does not equal the child digest "
                       f"{child['digest']!r}")
    expected_arch = child["platform"].split("/", 1)[1]
    arch = purl_qualifier(package_purl(describing), "arch")
    if arch != expected_arch:
        fail(problems, f"sbom {platform}: describing package purl arch={arch!r} does not equal "
                       f"the platform architecture {expected_arch!r}")


def step_sbom(ref, digest, syft, problems):
    try:
        children = read_children(ref, digest)
    except CheckError as error:
        fail(problems, str(error))
        return
    if not children:
        fail(problems, "sbom: no platform child to check")
        return

    go_module = go_module_name()
    base_name, base_tag = dockerfile_final_from()

    attested = {}
    for child in children:
        platform = child["platform"]
        result = run(["cosign", "verify-attestation", "--type", "spdxjson",
                      "--certificate-identity-regexp", IDENTITY_REGEXP,
                      "--certificate-oidc-issuer", OIDC_ISSUER,
                      f"{ref}@{child['digest']}"])
        if result.returncode != 0:
            fail(problems, f"sbom {platform}: cosign verify-attestation did not verify: {result.stderr.strip()}")
            continue
        # cosign attest appends rather than replaces.
        envelope_lines = [line for line in result.stdout.strip().splitlines() if line.strip()]
        if len(envelope_lines) != 1:
            fail(problems, f"sbom {platform}: expected exactly one spdxjson attestation, "
                           f"cosign verify-attestation printed {len(envelope_lines)}")
            continue
        try:
            predicate = extract_predicate(envelope_lines[0])
        except (KeyError, ValueError, json.JSONDecodeError) as error:
            fail(problems, f"sbom {platform}: could not extract the predicate: {error}")
            continue

        if predicate.get("spdxVersion") != "SPDX-2.3":
            fail(problems, f"sbom {platform}: extraction path wrong — spdxVersion is "
                           f"{predicate.get('spdxVersion')!r}, expected 'SPDX-2.3'")
            continue

        arch = platform.split("/", 1)[1]
        predicate_path = ROOT / f"predicate-{arch}.spdx.json"
        predicate_path.write_text(json.dumps(predicate), encoding="utf-8")
        schema_result = run(["pipx", "run", CHECK_JSONSCHEMA_PIN,
                             "--schemafile", str(SCHEMA_PATH), str(predicate_path)])
        if schema_result.returncode != 0:
            fail(problems, f"sbom {platform}: SPDX schema validation failed: "
                           f"{(schema_result.stdout + schema_result.stderr).strip()}")

        check_sbom_content(platform, predicate, child, go_module, base_name, base_tag, problems)
        attested[child["digest"]] = (child, predicate)

    for child in children:
        if child["digest"] not in attested:
            continue
        _, predicate = attested[child["digest"]]
        platform = child["platform"]
        arch = platform.split("/", 1)[1]
        regenerated_path = ROOT / f"regenerated-{arch}.spdx.json"
        result = subprocess.run([syft, "scan", f"{ref}@{child['digest']}", "--platform", platform,
                                  "-o", f"spdx-json={regenerated_path}"])
        if result.returncode != 0:
            fail(problems, f"sbom {platform}: regeneration with syft failed")
            continue
        regenerated = json.loads(regenerated_path.read_text(encoding="utf-8"))
        attested_pairs = {(p.get("name"), p.get("versionInfo")) for p in predicate.get("packages", [])}
        regenerated_pairs = {(p.get("name"), p.get("versionInfo")) for p in regenerated.get("packages", [])}
        if attested_pairs != regenerated_pairs:
            fail(problems, f"sbom {platform}: regeneration mismatch on (name, versionInfo) pairs — "
                           f"only in the attestation: {sorted(attested_pairs - regenerated_pairs)}, "
                           f"only in the regeneration: {sorted(regenerated_pairs - attested_pairs)}")


# --- attached set -------------------------------------------------------------------------------

def step_attached(ref, digest, tag, problems):
    try:
        children = read_children(ref, digest)
    except CheckError as error:
        fail(problems, str(error))
        children = []
    if not children:
        fail(problems, "attached: no platform child to check")

    result = run(["cosign", "tree", f"{ref}@{digest}"])
    combined = result.stdout + result.stderr
    if "Signatures for an image tag" not in combined:
        fail(problems, f"attached: cosign tree {ref}@{digest} did not show 'Signatures for an image tag': "
                       f"{combined.strip()}")

    for child in children:
        result = run(["cosign", "tree", f"{ref}@{child['digest']}"])
        combined = result.stdout + result.stderr
        if "Signatures for an image tag" not in combined:
            fail(problems, f"attached: cosign tree {child['platform']} did not show "
                           f"'Signatures for an image tag': {combined.strip()}")
        if "Attestations for an image tag" not in combined:
            fail(problems, f"attached: cosign tree {child['platform']} did not show "
                           f"'Attestations for an image tag': {combined.strip()}")

    result = run(["gh", "release", "view", tag, "--json", "assets"])
    if result.returncode != 0:
        fail(problems, f"attached: gh release view {tag} --json assets failed: {result.stderr.strip()}")
    else:
        try:
            names = sorted(asset["name"] for asset in json.loads(result.stdout).get("assets", []))
        except (json.JSONDecodeError, KeyError) as error:
            fail(problems, f"attached: gh release view --json assets did not parse: {error}")
        else:
            expected = sorted(["compose.yaml", "config.example.json"])
            if names != expected:
                fail(problems, f"attached: release assets are {names}, expected exactly {expected}")

    result = run(["gh", "release", "view", tag, "--json", "body"])
    if result.returncode != 0:
        fail(problems, f"attached: gh release view {tag} --json body failed: {result.stderr.strip()}")
    else:
        try:
            body = json.loads(result.stdout).get("body", "")
        except json.JSONDecodeError as error:
            fail(problems, f"attached: gh release view --json body did not parse: {error}")
        else:
            expected_line = f"Image: {ref}@{digest}"
            if expected_line not in body:
                fail(problems, f"attached: release notes do not contain {expected_line!r}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--step", required=True, choices=["signature", "provenance", "sbom", "attached"])
    args = parser.parse_args()

    ref = os.environ["REF"]
    digest = os.environ["DIGEST"]
    problems = []

    if args.step == "signature":
        step_signature(ref, digest, problems)
    elif args.step == "provenance":
        step_provenance(ref, digest, os.environ["COMMIT"], problems)
    elif args.step == "sbom":
        step_sbom(ref, digest, os.environ["SYFT"], problems)
    elif args.step == "attached":
        step_attached(ref, digest, os.environ["TAG"], problems)

    if problems:
        print(f"verify_release --step {args.step}: {len(problems)} problem(s):", file=sys.stderr)
        for problem in problems:
            print("  " + problem, file=sys.stderr)
        return 1
    print(f"verify_release --step {args.step}: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
