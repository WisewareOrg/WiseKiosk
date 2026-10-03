# `check-image-swap`

The inputs `image_swap.py` has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s § *Deployment and bring-up*; how to run a case is
[`../README.md`](../README.md)'s.

Run against the two releases published on this repository, `v0.0.1` (prerelease), digest
`sha256:a233121b003e8125d978d97a89c2e02dffc1e1568a1a724b8319ee07da0e1fa7`, and `v0.1.0` (latest,
non-pre-release), digest `sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a`.
Script md5 `a4ba80b7d77150f1a4485acab97f600d`. The script does not read pre-release status itself — that filter is the
`image-swap` job's, applied before the script ever runs — so `v0.0.1` stands in as an ordinary
digest for these rows. Each row invokes `image_swap.py` directly with the tags and digests shown,
reaching the real releases and the real images; nothing tracked is edited except where a row states
a seed.

**Re-observed after the health poll, configuration fetch and manifest-field resolution moved into
`scripts/bringup/common.py`, shared with `bring_up.py`.** Script md5 `39ddd99155742a32b350a55e206b6a77`.
Every row but the nonexistent-digest one calls the moved code and was re-run; each
reported byte-identical text and near-identical timing, noted per row.

| Direction | Case | Input |
|---|---|---|
| Must pass | The two releases, oldest first | `v0.0.1 sha256:a233…e1fa7 v0.1.0 sha256:27ff…d589a` — `v0.0.1 (0.0.1) and v0.1.0 (0.1.0) each serve their mounted configuration under the same mount arguments, with no builder invoked` in 69.2s; re-observed post-factoring, 69.8s |
| Must pass | The same two releases, order swapped (versions still differ) | `v0.1.0 sha256:27ff…d589a v0.0.1 sha256:a233…e1fa7` — passes identically, in 66.1s; re-observed post-factoring, 69.1s |
| Must fail | The same release given as both A and B | `v0.1.0 sha256:27ff…d589a v0.1.0 sha256:27ff…d589a` — `` v0.1.0 and v0.1.0 both report version '0.1.0' — the swap changed nothing `` — re-observed post-factoring, byte-identical |
| Must fail | B's digest does not exist | `v0.1.0 sha256:27ff…d589a v0.1.0 sha256:00…00` — `` `docker run ghcr.io/tjwise99/wisekiosk@sha256:00…00` exited 125 (…failed to resolve reference…: not found) `` — nothing pulled, nothing built. Fails in `start_container`, before any moved code runs; not re-run |
| Must fail | B's mount omitted for the second run | seeded: `start_container`'s `--volume` argument dropped for the B call only — `` /config.json answered 404, expected 200 `` in 69.1s; re-observed post-factoring against a freshly reapplied seed, byte-identical, 66.9s |

**Real-world confirmation of the no-previous-release path.** At the time these rows were run, this
repository carries exactly one non-pre-release (`v0.1.0`); `gh release list --exclude-pre-releases
--json tagName --jq` with the current release's tag filtered out returns nothing, which the
`image-swap` job's `previous` step writes to `$GITHUB_OUTPUT` as an empty `tag`. The swap step's
own `if: steps.previous.outputs.tag != ''` condition is then false, so the step — and
`image_swap.py` with it — is skipped rather than run and passing — confirmed against the live
repository rather than seeded. The `previous` step's `--exclude-drafts` was added after these rows
ran; its effect is recorded in [`publish-draft.md`](publish-draft.md)'s `--exclude-drafts` row.

## The release-dir interface's new side

Row for `image_swap.py`'s interface, `image_swap.py <previous-tag> <previous-digest> <tag>
<release-dir> <digest>` — the side under test takes its files from `<release-dir>` directly rather
than from `gh release download`, which lets this check run against a release still only a draft,
with no published tag to download from; it still runs containers by digest directly (`docker run`),
so `deploy/compose.yaml`'s `WISEKIOSK_IMAGE` variable plays no part here
([#418 publish under immutable releases](https://github.com/WisewareOrg/WiseKiosk/issues/418#issuecomment-5961738214)).
States the case, how it is seeded or run, and the expected outcome. `<release-dir>` is hand-built: a
`deploy/compose.yaml` plus `v0.2.1`'s published `config.example.json`.

| Case | Seed / run | Expected outcome | Evidence |
|---|---|---|---|
| The published previous release swapped against the release-dir side under test | `image_swap.py v0.1.0 sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a v0.2.1 <release-dir> sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2` — the previous side's configuration comes from `gh release download v0.1.0`, as before; the side under test's comes from `<release-dir>` directly, with no `gh release download` call for it | both sides run their own digest directly, with no builder invoked; each serves its own mounted configuration; v0.1.0 (0.1.0) and v0.2.1 (0.2.1) report different versions, each equal to its own tag | Confirmed, byte for byte: `v0.1.0 (0.1.0) and v0.2.1 (0.2.1) each serve their mounted configuration under the same mount arguments, with no builder invoked` in 68.3s. v0.1.0's image resolves at `ghcr.io/wisewareorg/wisekiosk@sha256:27ff2637…589a`. Script md5 `e9da905f15f70dc9b4c0e35b483965a8` |

**Known gap.** The secret directory is not exercised: this check is config-only, the secret mount
being [#261 secret mount](https://github.com/WisewareOrg/WiseKiosk/issues/261)'s (owner ruling,
2026-09-04). "No builder invoked" holds by construction rather than by a row here: the harness
calls only `docker run`, `port`, `inspect`, `stop` and `rm`, and this is made observable by the
per-run `RepoDigests` assertion rather than by a seed that would need a builder to invoke.

**The `image-swap` job's own steps are unverified in CI.** Resolving the previous release's tag and
digest, and the job's `permissions` and trigger condition, are workflow YAML outside
`image_swap.py` and outside every row above; they are exercised for the first time by a real
release's job run.
