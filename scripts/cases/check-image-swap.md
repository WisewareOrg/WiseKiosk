# `check-image-swap`

The inputs `image_swap.py` has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s § *Deployment and bring-up*; how to run a case is
[`../README.md`](../README.md)'s.

Run against the two releases published on this repository, `v0.0.1` (prerelease), digest
`sha256:a233121b003e8125d978d97a89c2e02dffc1e1568a1a724b8319ee07da0e1fa7`, and `v0.1.0` (latest,
non-pre-release), digest `sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a`.
Script md5 `a4ba80b7d77150f1a4485acab97f600d` at `9fb833a ci(publish): swap two published digests
under the same mounts`. The script does not read pre-release status itself — that filter is the
`image-swap` job's, applied before the script ever runs — so `v0.0.1` stands in as an ordinary
digest for these rows. Each row invokes `image_swap.py` directly with the tags and digests shown,
reaching the real releases and the real images; nothing tracked is edited except where a row states
a seed.

**Re-observed after the health poll, configuration fetch and manifest-field resolution moved into
`scripts/bringup/common.py`, shared with `bring_up.py`.** Script md5 `39ddd99155742a32b350a55e206b6a77`
at `7617e79 refactor(bringup): factor the health-poll, config-fetch and manifest-read helpers into
common.py`. Every row but the nonexistent-digest one calls the moved code and was re-run; each
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
repository rather than seeded.

**Known gap.** The secret directory is not exercised: this check is config-only, the secret mount
being [#261 secret mount](https://github.com/tjwise99/WiseKiosk/issues/261)'s (owner ruling,
2026-09-04). "No builder invoked" holds by construction rather than by a row here: the harness
calls only `docker run`, `port`, `inspect`, `stop` and `rm`, and this is made observable by the
per-run `RepoDigests` assertion rather than by a seed that would need a builder to invoke.

**The `image-swap` job's own steps are unverified in CI.** Resolving the previous release's tag and
digest, and the job's `permissions` and trigger condition, are workflow YAML outside
`image_swap.py` and outside every row above; they are exercised for the first time by a real
release's job run.
