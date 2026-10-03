# The `verify` job's release checks (`scripts/publish/sbom_attest.py`, `verify_metadata.py`, `verify_release.py`)

The inputs these checks have been run against, in both directions. What they *assert*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s § *Publishing and provenance*; how to run a case is
[`../README.md`](../README.md)'s. `scripts/publish/verify_permissions.py` (the no-write-scope gate)
has its own file, [`check-publish-permissions.md`](check-publish-permissions.md).

**Tools, pinned as run:** cosign v3.1.3 and syft v1.52.0, each named as a `with:` literal in both
the `draft` and `verify` jobs — Renovate's regex manager bumps every match of `cosign-release:`
and `syft-version:` in `publish.yml` in one PR, which is what keeps the two per-tool pins equal; a
hand edit drifting them apart surfaces as a package-set mismatch in `verify`'s SBOM regeneration
comparison rather than failing silently. `check-jsonschema==0.38.2`
(`pipx run`, pinned in `scripts/publish/verify_release.py` rather than in the workflow, since that is
where the invocation lives); `gh` and `docker buildx imagetools inspect` at whatever version the
runner or this host provides — neither is pinned, matching this repository's existing convention for
those two tools elsewhere.

## What is verified here, against measured and synthetic data

**`v0.2.1` is the first release whose `verify` job actually ran.** Its own run failed at the
signature step, on a string match that had never met real cosign v3.1.3 output; provenance, SBOM,
metadata and the attached-set check were all skipped as a result and never ran in that job
([#418 publish under immutable releases](https://github.com/WisewareOrg/WiseKiosk/issues/418#issuecomment-5961738214)).
The attached-set check's own string-match defect was found by hand against that same signed digest,
per that record. Both fixed matches — signature and attached-set — were then re-run by hand against
that digest with the fixed script; the rows below record them, along with the artifact-backed
`--step attached` input. The earlier releases, `v0.1.0` and `v0.0.1` (pre-release), were cut before
`verify` could run against a tag's ref at all and carry no signature or attestation; `v0.2.0`'s own
run signed, attested and SBOM'd its image before failing at asset upload, never reaching `verify` —
see the attached-set and metadata rows below, both run against `v0.1.0` for real.

Everything below that could not be run against a real published digest was exercised against real
tool output measured for this ticket (a throwaway image built from this repository's own
`Dockerfile`, pushed to a local `registry:2` container, scanned with the pinned syft and inspected
with `docker buildx imagetools inspect`, measured during implementation and not separately retained)
and against realistic synthetic fixtures built from that measured shape. The **reviewer-runnable
seeds** section below is what a reviewer runs against a real signed digest, such as `v0.2.1`'s.

**`sbom_attest.py` (draft job, per-child SBOM generation and attestation).** `read_children`
(`scripts/publish/common.py`) parsed a real two-entry `docker buildx imagetools inspect --format
'{{json .Manifest}}'` output (one real platform child plus a docker-generated attestation-manifest
descriptor at `platform: {"architecture":"unknown","os":"unknown"}`, produced when a test build did
not disable provenance/sbom) into `[{"digest": ..., "platform": "linux/amd64"}]` pairs — verified
against a mocked two-child fixture directly (`{'digest': 'sha256:aaa', 'platform': 'linux/amd64'},
{'digest': 'sha256:bbb', 'platform': 'linux/arm64'}`), matching the two-platform shape production
always builds (`platforms: linux/amd64,linux/arm64`) with `provenance: false, sbom: false`, which
never produces the spurious third entry.

**`verify_metadata.py` (annotations and labels) — exercised against the real, published two-platform
`v0.1.0` index**, `sha256:27ff2637…`, commit `4ee8b022…`:

| Direction | Case | Result |
|---|---|---|
| Must pass | The real image, `DOCKER_METADATA_ANNOTATIONS_LEVELS: index,manifest` (this repository's committed declaration), matching commit | `2 annotation level(s), 3 surface(s), 2 child config(s): the nine keys are present and bound to 4ee8b022…` |
| Must fail | The same real image, wrong `COMMIT` | seven named problems, including `no annotation surface was successfully checked` |
| Must fail | The same real image, `DOCKER_METADATA_ANNOTATIONS_LEVELS: index,bogus-level` | `unrecognised annotation level 'bogus-level'`, and `index` is still checked |
| Must fail | Two of nine keys set, seven missing, on `index`, `manifest` and `manifest-descriptor` levels (synthetic, single-platform pushed image) | lists the seven missing keys per surface, by name |
| Must fail | A single-platform index but `check_labels`'s branch expects more than one child (synthetic) | `.Image is a single config but the index has N platform child(ren)` — the degenerate single-platform `.Image` shape (no platform-keyed map) is asserted rather than silently mismatched |

**The genuinely multi-platform case is closed, not a gap.** The real `v0.1.0` index's
`imagetools inspect --format '{{json .Image}}'` returns a genuinely platform-keyed map (a top-level
`linux/amd64` key, each with its own `config`), confirmed by two independent routes:
one run of `imagetools inspect` directly against the digest, and a separate run of
`verify_metadata.py` end to end against the same digest, which got the `2 child config(s)` result in
the must-pass row above. `check_labels`'s platform-keyed branch is exercised against real data, not only
against the documented `buildx` behaviour it was originally written to.

**`verify_release.py`'s pure logic — unit-tested directly**, exercised here with synthetic data
rather than against a real signed digest:

| Function | Case | Result |
|---|---|---|
| `check_sbom_content` | A clean predicate: Go module present, one `distro=alpine-3.24.1` qualifier, describing package `versionInfo` matches the child digest, purl `arch=amd64` matches the platform | no problems |
| `check_sbom_content` | No package carries a `distro=` qualifier at all | `no package carries a purl 'distro=' qualifier` |
| `check_sbom_content` | Cross-wired binding: describing package's `versionInfo` is the *other* child's digest, everything else (Go module, distro) correct | fails only `describing package versionInfo '...' does not equal the child digest '...'` — the Go-module and distro assertions do not also fire, proving the binding fails by name rather than by accident (criterion 11) |
| `check_sbom_content` | Wrong distro name (`debian` against a Dockerfile whose final `FROM` is `alpine`) | `distro name 'debian' does not match the Dockerfile's final FROM image 'alpine'` |
| pair-set comparison (inline, `attested_pairs != regenerated_pairs`) | One package's `versionInfo` altered between the attested and regenerated sets (version drift) | set inequality detected, with the differing pair named on each side |
| `step_sbom`'s envelope-count guard | `cosign verify-attestation` prints two envelope lines (a re-run's second attestation, simulated) | `expected exactly one spdxjson attestation, cosign verify-attestation printed 2` — fails before extraction is attempted |
| `extract_predicate` | A synthetic DSSE envelope (`{"payload": base64(json)}`) matching `cosign verify-attestation`'s documented single-line output shape | round-trips to the original predicate exactly |
| `check_provenance_fields` | A realistic `--format json` entry matching the plan's measured shape (`buildSignerURI`, `sourceRepositoryURI`, `sourceRepositoryDigest`, `statement.subject[0].digest.sha256`, `predicateType`) | no problems |
| `check_provenance_fields` | Wrong `sourceRepositoryDigest` | fails only the `sourceRepositoryDigest` assertion |
| `check_provenance_fields` | Wrong `subject[0].digest.sha256` | fails only the subject-digest assertion |
| `check_provenance_fields` | `statement.subject: []` (an empty list, not a missing key) | fails the subject-digest assertion on `None`, rather than raising `IndexError` — the `[{}]` default only covers a *missing* key, so this is asserted separately |
| `check_provenance_fields` | Two entries instead of one | `expected exactly one attestation entry, got 2` |
| `step_signature`, `step_attached` | `read_children` returns `[]` without raising (a single-platform export, which `docker buildx build` refuses to annotate at the index level at all — measured, § below) | both fail closed with `no platform child to check`, rather than reporting `0 problem(s)` on zero children verified; `step_signature` still checks the index's own signature and `step_attached` still runs its index-level `cosign tree` check and its `$ARTIFACT_DIR` asset-list and notes reads, since none of the three depend on children |
| `go_module_name`, `dockerfile_final_from`, `purl_qualifier` | Run against this repository's own `backend/go.mod` and `Dockerfile`, and against two real purls measured with syft v1.51.1 against this repository's own two-platform image: the apk purl `pkg:apk/alpine/alpine-baselayout@3.7.2-r1?arch=x86_64&distro=alpine-3.24.1` (amd64 child) and the image-descriptor purl's `arch=` qualifier, `arch=amd64` (amd64 child) / `arch=arm64` (arm64 child) | `github.com/tjwise99/WiseKiosk/backend`; `('alpine', '3.24')`; `alpine-3.24.1` / `x86_64` from the apk purl — **a different vocabulary from the image-descriptor purl's own `amd64`/`arm64`**, which is what `check_sbom_content`'s binding assertion actually compares against (`child["platform"].split("/", 1)[1]`), never apk's `x86_64`/`aarch64` |

**Two provenance refusal texts are measured against the real `v0.1.0` index**, replacing an
earlier version of this file that asserted an unmeasured, incorrect shared literal (see *Reviewer-
runnable seeds* below for the fix this drove in `verify_release.py`) — `gh` 2.97.0:

| Condition | rc | stdout | stderr |
|---|---|---|---|
| A flipped (reversed-hex) digest against the real index | 1 | 0 bytes | `Error: failed to fetch remote image: GET https://ghcr.io/v2/tjwise99/wisekiosk/manifests/sha256:<flipped>: MANIFEST_UNKNOWN: manifest unknown` |
| The real digest, no attestation attached (true of `v0.0.1` and `v0.1.0`) | 1 | 0 bytes | `Error: HTTP 404: Not Found (https://api.github.com/repos/tjwise99/WiseKiosk/attestations/sha256:<digest>?...)` |
| A wrong `--signer-workflow` against a digest that *does* carry an attestation | 1 | 0 bytes | `Error: verifying with issuer "sigstore.dev"` — measured against the real, attested `v0.2.1` index, not `v0.1.0`'s (`v0.1.0` carries no attestation, like `v0.0.1`) |

**The vendored SPDX 2.3 schema** (`scripts/publish/spdx-schema-2.3.json`, from
https://github.com/spdx/spdx-spec/blob/v2.3/schemas/spdx-schema.json at tag `v2.3`) validated a real
syft-generated SBOM (`ok -- validation done`) and rejected the same document with `spdxVersion`
deleted (`'spdxVersion' is a required property`), via `check-jsonschema==0.38.0`.

## Reviewer-runnable seeds, against any real published digest

Each of these can be run directly against any signed, attested digest (`v0.2.1`'s,
`sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2`, or any later release's)
and is expected to fail for the reason given. None of these mutate the release; each either targets
a deliberately wrong identity/digest/workflow, or a throwaway copy pushed under a mismatched digest.

- **Signature, wrong identity.** `cosign verify --certificate-identity-regexp
  '^https://example\.invalid/' --certificate-oidc-issuer https://token.actions.githubusercontent.com
  <ref>@<digest>` — exits non-zero, printing `no matching CertificateIdentity found`. Measured
  against the real `v0.2.1` index (rows above).
- **Provenance, flipped digest.** `gh attestation verify oci://<ref>@sha256:<the real hex, reversed>
  --repo WisewareOrg/WiseKiosk --signer-workflow WisewareOrg/WiseKiosk/.github/workflows/publish.yml` —
  exits non-zero, printing the substring `MANIFEST_UNKNOWN: manifest unknown` on stderr, with an
  empty stdout — the registry refuses to resolve the reference before any attestation lookup runs.
  Measured against the real `v0.1.0` index and re-confirmed against `v0.2.1`'s; the digest is
  interpolated into the message, so only this substring is stable across releases, never the full
  line.
- **Provenance, wrong signer workflow, against a digest that genuinely carries an attestation.**
  `gh attestation verify oci://<ref>@<digest> --repo WisewareOrg/WiseKiosk --signer-workflow
  WisewareOrg/WiseKiosk/.github/workflows/checks.yml` — exits non-zero with an empty stdout, printing
  `Error: verifying with issuer "sigstore.dev"` on stderr. Measured against the real, attested
  `v0.2.1` index — the case `v0.0.1` and `v0.1.0`, both unattested, could not exercise. Deliberately
  weak on text: `verify_negative_no_json_success` asserts only the non-zero exit and the empty
  stdout, not this stderr line, since the six positive `--format json` field assertions carry the
  verdict for this check.
- **Provenance, registry-copy `--bundle-from-oci`, absent or wrong digest.** Push a throwaway copy of
  the image under a mismatched digest (or strip the bundle), never the real release, and run `gh
  attestation verify oci://<that ref>@<that digest> --repo tjwise99/WiseKiosk --signer-workflow
  tjwise99/WiseKiosk/.github/workflows/publish.yml --bundle-from-oci` — must fail.

The identity above is what these negatives were recorded against; the identity as the tree stands is
`--repo WisewareOrg/WiseKiosk --signer-workflow WisewareOrg/WiseKiosk/.github/workflows/publish.yml`.

- **SBOM, a second attestation on re-run.** `cosign attest` a second SPDX predicate onto the same
  child (simulating a re-run of `draft`) — `step_sbom`'s envelope-count guard must fail before
  extraction, rather than validating whichever envelope `cosign verify-attestation` prints first.
- **SBOM, version-drift predicate.** Take a real attested SPDX predicate, alter one package's
  `versionInfo`, feed it in place of the regenerated document — must fail the `(name, versionInfo)`
  pair comparison (not the binding assertion).
- **SBOM, cross-wired predicate.** Feed one child's real SBOM predicate in place of the other's — the
  two children's pair sets differ only in the describing package's own `(name, versionInfo)` entry, so
  this must fail the binding assertion specifically, not the pair comparison.
- **`verify_permissions.py` fail-closed guards**, against throwaway `publish.yml` copies (not the
  committed one): `packages: write` added under `verify`; the `verify` job renamed. Both covered fully
  in [`check-publish-permissions.md`](check-publish-permissions.md).
- **`verify_metadata.py`'s unknown-level guard**, against a throwaway `publish.yml` copy declaring
  `DOCKER_METADATA_ANNOTATIONS_LEVELS: index,bogus-level` — already exercised above against a real
  pushed image; reproducible against any digest.

**A defect this case-writing process found and fixed, rather than only recorded.** An earlier version
of the flipped-digest and wrong-signer-workflow seeds both asserted a single shared literal, `Error:
verifying with issuer "sigstore.dev"`, taken from the plan rather than measured. Reproduced against
the real `v0.1.0` index, neither refusal prints that text — a flipped digest fails registry
resolution with `MANIFEST_UNKNOWN` before any attestation lookup, and the plain no-attestation case
gives an HTTP 404 from GitHub's API — so `--step provenance` would have failed on every release, for
a defect in the checker rather than in the release.
Fixed in `verify_release.py` by asserting the two texts actually measured above, leaving the
wrong-signer-workflow case unasserted on text — the discarded shared literal turned out to be the
genuine text for that case once a real attested digest made it observable (row above).

**Fallibility is recorded once here, against throwaway copies; no standing meta-gate re-tests it**
([`check-arch.md`](check-arch.md) states that convention).

## Known gaps

Each needs a deliberately broken release and is unseeded, since seeding a broken release was
skipped for #269 publish verification: a missing release asset; a missing
attestation (no SBOM, no signature, or no provenance attached to a child); a dropped annotation key;
an empty annotation or label value; a wrong `.revision` on one surface only (the others correct).

## The cosign v3.1.3 output, and the artifact-backed `--step attached` input

Rows for the two string-match fixes and the artifact-backed `--step attached` input, recorded
against cosign v3.1.3's actual output on the signed `v0.2.1` digest,
`sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2`
([#418 publish under immutable releases](https://github.com/WisewareOrg/WiseKiosk/issues/418#issuecomment-5961738214)).
Each row states the case, how it is seeded or run, and the expected outcome.

Script md5 `e65399b90d097624f5e6ba5332065c5c`. The rows below ran against md5
`94b53f8646ff2531283370241fa24c2d`, which differs from the current file only in its module
docstring; the script's logic, which every row ran against, has not changed.

| Case | Seed / run | Expected outcome | Evidence |
|---|---|---|---|
| Signature step, wrong identity: the real cosign v3.1.3 refusal is recognised | `cosign verify --certificate-identity-regexp '^https://example\.invalid/' --certificate-oidc-issuer https://token.actions.githubusercontent.com ghcr.io/wisewareorg/wisekiosk@sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2` against the signed index | cosign refuses (non-zero exit), printing `no matching attestations: failed to verify certificate identity: no matching CertificateIdentity found, ...`; `step_signature`'s negative check matches on `no matching CertificateIdentity found` and raises no problem for it — combined with the positive per-surface `cosign verify` calls (already observed passing against this digest), `--step signature` prints `verify_release --step signature: ok` | Confirmed, byte for byte: `cosign verify` exited 1 and printed `Error: no matching attestations: failed to verify certificate identity: no matching CertificateIdentity found, last error: expected SAN value to match regex "^https://example\.invalid/", got "https://github.com/WisewareOrg/WiseKiosk/.github/workflows/publish.yml@refs/tags/v0.2.1"` (repeated on stderr); `python3 verify_release.py --step signature` (`REF`/`DIGEST` as above) printed `verify_release --step signature: ok` |
| Signature step, wrong identity: a refusal for a different reason is still reported as unexpected | the same wrong-identity regexp, against the index's own digest with its hex reversed (`sha256:<flipped>`) — the flipped-digest seed this file's provenance rows already use | `cosign verify` finds no signature tag for that digest and fails before any identity check runs, so its output does not contain `no matching CertificateIdentity found`; `step_signature` reports `signature: refused for an unexpected reason: <cosign's own output>` | Confirmed: `cosign verify` against `ghcr.io/wisewareorg/wisekiosk@sha256:2a59e9e00df55ca6675db4f9163deb7d7726db94856278caefc945fdc9c93255` (the real digest's hex reversed) exited 10, printing `Error: no signatures found` / `error during command execution: no signatures found` — no `no matching CertificateIdentity found` substring; calling `cosign_verify` directly through `verify_release.py` and evaluating `step_signature`'s negative branch against that output produced `['signature: refused for an unexpected reason: Error: no signatures found\nerror during command execution: no signatures found']` |
| `--step attached`, per-surface headings present | `cosign tree` against the signed index and each platform child | the index's output carries `sigstore.dev/cosign/sign/v1` and `slsa.dev/provenance/v1`; each child's output carries `sigstore.dev/cosign/sign/v1` and `spdx.dev/Document`; the heading checks append no problems | Confirmed against the signed index and both platform children (`sha256:53e3862a…`, linux/amd64; `sha256:0fbbdd31…`, linux/arm64): the index's `cosign tree` output carries exactly `sigstore.dev/cosign/sign/v1` and `slsa.dev/provenance/v1` (no `spdx.dev/Document`); each child's carries exactly `sigstore.dev/cosign/sign/v1` and `spdx.dev/Document` (no `slsa.dev/provenance/v1`); `python3 verify_release.py --step attached` (`ARTIFACT_DIR` built from `v0.2.1`'s own assets and notes) printed `verify_release --step attached: ok` |
| `--step attached`, the index missing either of its expected headings has each reported | `ARTIFACT_DIR` built from `v0.2.1`'s own assets and notes, run with `DIGEST` pointed at `v0.1.0`'s unattested index instead (`sha256:27ff2637…589a`), so `cosign tree` on it shows neither expected heading | `step_attached` reports both: `sigstore.dev/cosign/sign/v1` and `slsa.dev/provenance/v1` missing on the index | Confirmed, verbatim: `python3 verify_release.py --step attached` (`REF=ghcr.io/wisewareorg/wisekiosk`, `DIGEST=sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a`) printed `attached: cosign tree ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a did not show 'sigstore.dev/cosign/sign/v1': 📦 Supply Chain Security Related artifacts for an image: ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a\nNo Supply Chain Security Related Artifacts found for image ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a,\n start creating one with simply running$ cosign sign <img>` and `attached: cosign tree ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a did not show 'slsa.dev/provenance/v1': 📦 Supply Chain Security Related artifacts for an image: ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a\nNo Supply Chain Security Related Artifacts found for image ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a,\n start creating one with simply running$ cosign sign <img>`, exit 1. A seventh problem the same run printed, `attached: release notes do not contain 'Image: ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a'`, is the expected consequence of pairing `v0.2.1`'s artifact with `v0.1.0`'s digest, not part of this case |
| `--step attached`, a platform child missing either of its expected headings has each reported | the same run: `cosign tree` on `v0.1.0`'s two unattested children shows neither expected heading | `step_attached` reports both, per child: `sigstore.dev/cosign/sign/v1` and `spdx.dev/Document` missing | Confirmed, verbatim, from the same run above: `attached: cosign tree linux/amd64 did not show 'sigstore.dev/cosign/sign/v1': 📦 Supply Chain Security Related artifacts for an image: ghcr.io/wisewareorg/wisekiosk@sha256:c0f72f6eab4cb5ae67a850705df46a12d973b580a767bbb1a160055f059d2120\nNo Supply Chain Security Related Artifacts found for image ghcr.io/wisewareorg/wisekiosk@sha256:c0f72f6eab4cb5ae67a850705df46a12d973b580a767bbb1a160055f059d2120,\n start creating one with simply running$ cosign sign <img>`; `attached: cosign tree linux/amd64 did not show 'spdx.dev/Document': 📦 Supply Chain Security Related artifacts for an image: ghcr.io/wisewareorg/wisekiosk@sha256:c0f72f6eab4cb5ae67a850705df46a12d973b580a767bbb1a160055f059d2120\nNo Supply Chain Security Related Artifacts found for image ghcr.io/wisewareorg/wisekiosk@sha256:c0f72f6eab4cb5ae67a850705df46a12d973b580a767bbb1a160055f059d2120,\n start creating one with simply running$ cosign sign <img>`; `attached: cosign tree linux/arm64 did not show 'sigstore.dev/cosign/sign/v1': 📦 Supply Chain Security Related artifacts for an image: ghcr.io/wisewareorg/wisekiosk@sha256:641a50c1a2bf659b0304a8d6e3f818d5851b5909d2c3b00b8adbb631dc901f58\nNo Supply Chain Security Related Artifacts found for image ghcr.io/wisewareorg/wisekiosk@sha256:641a50c1a2bf659b0304a8d6e3f818d5851b5909d2c3b00b8adbb631dc901f58,\n start creating one with simply running$ cosign sign <img>`; `attached: cosign tree linux/arm64 did not show 'spdx.dev/Document': 📦 Supply Chain Security Related artifacts for an image: ghcr.io/wisewareorg/wisekiosk@sha256:641a50c1a2bf659b0304a8d6e3f818d5851b5909d2c3b00b8adbb631dc901f58\nNo Supply Chain Security Related Artifacts found for image ghcr.io/wisewareorg/wisekiosk@sha256:641a50c1a2bf659b0304a8d6e3f818d5851b5909d2c3b00b8adbb631dc901f58,\n start creating one with simply running$ cosign sign <img>` |
| `--step attached`, assets and notes read from `$ARTIFACT_DIR` | `$ARTIFACT_DIR/assets/` holds the release's own `compose.yaml` and `config.example.json`; `$ARTIFACT_DIR/notes.md` holds the release's own notes (carrying `Image: ghcr.io/wisewareorg/wisekiosk@sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2`); no `gh release view` call is made | the asset-list and notes checks each append no problems | Confirmed: `ARTIFACT_DIR` built from `gh release download v0.2.1` (into `assets/`) and `gh release view v0.2.1 --json body -q .body` (into `notes.md`); `python3 verify_release.py --step attached` printed `verify_release --step attached: ok` with no `gh release view` call made by the script |
| `--step attached`, an extra asset in `$ARTIFACT_DIR/assets/` is reported | the same directory with an extra file added, e.g. `extra.txt` | `attached: release assets are ['compose.yaml', 'config.example.json', 'extra.txt'], expected exactly ['compose.yaml', 'config.example.json']` | Confirmed, byte for byte: with `extra.txt` added to a copy of the same `assets/` directory, `python3 verify_release.py --step attached` printed `verify_release --step attached: 1 problem(s):` then `  attached: release assets are ['compose.yaml', 'config.example.json', 'extra.txt'], expected exactly ['compose.yaml', 'config.example.json']`, exit 1 |

## `v0.2.1`'s CI run, and what replaces it

`v0.2.1`'s own `verify` job is the first to have run this job's checks against a signed and attested
release. It failed at the signature step, on one of the two stale string matches the
rows above fix; provenance, SBOM, metadata and the attached-set check were skipped as a result,
never running in that job — the attached-set match was found and fixed by hand, not from this run
([#418 publish under immutable releases](https://github.com/WisewareOrg/WiseKiosk/issues/418#issuecomment-5961738214)).
The fixed script was re-run by hand against that same signed digest (recorded above), not by a CI
job, since `verify` runs ahead of `publish` rather than re-running against an already-locked release.
