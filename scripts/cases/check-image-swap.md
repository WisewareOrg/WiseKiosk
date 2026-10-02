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

**Decision 3 and decision 4 specified before implementation.** The rows below specify
`image_swap.py`'s own candidate selection, probe classification and argument-count behavior, ahead
of the code that implements them — this is the red half of the pair, and each row stays red until
the implementation fills in its observed result. Each row is marked **runnable now** (the inputs
and the code it exercises, `argparse` and `common.imagetools_inspect`, already exist) or **pending
the first org non-pre-release** (`ghcr.io/wisewareorg/wisekiosk` answers 403 — confirmed by hand
2026-10-02 — until something is first pushed there, which the row needs). A runnable-now row's
Input cell states the command and the outcome it is expected to produce; the implementation run
replaces "expected" with what that row's own run printed.

| Direction | Case | Input |
|---|---|---|
| Must fail | Exactly 1 positional | `v0.1.0` — **runnable now**; expected: argparse's own usage error to stderr, exit 2 |
| Must fail | Exactly 3 positionals | `v0.1.0 sha256:27ff…d589a v0.0.1` — **runnable now**; expected: argparse's own usage error to stderr, exit 2 |
| Must pass | 2-form, `TAG` with no older non-pre-release | `v0.1.0 sha256:27ff…d589a` — **runnable now**; confirmed 2026-10-02 via `gh release list --exclude-pre-releases --json tagName,publishedAt`, which names only `v0.1.0` itself, so no non-pre-release has a `publishedAt` earlier than it and the candidate list is empty before any probe runs; expected stdout `` image-swap: skipped — no non-pre-release older than v0.1.0 has an image in ghcr.io/wisewareorg/wisekiosk (checked: none) ``, exit 0 |
| Must fail | 2-form, nonexistent `TAG` | `v9.9.9 sha256:00…00` — **runnable now**; `v9.9.9` is not a release of this repository (confirmed 2026-10-02 via the same `gh release list` above); expected: exit 1, an `image-swap: `-prefixed message to stderr (the plan does not specify its exact wording) |
| Must pass | Probe classification: an existing old-namespace tag | `common.imagetools_inspect("ghcr.io/tjwise99/wisekiosk:0.1.0", ".Manifest.Digest")` — **runnable now**; `docker buildx imagetools inspect` against that reference returned exactly this digest when run by hand 2026-10-02; expected: `(value, None)` with `value` equal to `sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a` — found |
| Must pass | Probe classification: a nonexistent old-namespace tag | `common.imagetools_inspect("ghcr.io/tjwise99/wisekiosk:9.9.9", ".Manifest.Digest")` — **runnable now**; `docker buildx imagetools inspect` against that reference printed `` ghcr.io/tjwise99/wisekiosk:9.9.9: not found `` when run by hand 2026-10-02; expected: `(None, stderr)` with `stderr` containing `not found` (case-insensitive) — absent |
| Must pass | 4-form, unchanged | `v0.0.1 sha256:a233…e1fa7 v0.1.0 sha256:27ff…d589a` — **runnable now**; expected: identical to this table's first row above, since decision 4 leaves the 4-positional form's behavior unchanged |
| Must pass | Decision 3 walks past an absent candidate to a found one | **pending the first org non-pre-release** — needs two non-pre-releases under `ghcr.io/wisewareorg/wisekiosk`, the newer absent and the next older found; not yet real, since the namespace carries no image until the first org publish |
| Must pass | Decision 3 swaps against the found candidate | **pending the first org non-pre-release** — the swap itself, run against the TAG under test and the found older candidate; same prerequisite as the row above |

**Decision 3's fail-closed path has no row above.** The "anything else" branch — another error from
`imagetools_inspect`, or a value that does not match `^sha256:[0-9a-f]{64}$` — has no real input
that reaches it without a stub standing in for the registry or the probe, which no row above does.
It stays unverified until a real failure of that shape is observed, or the plan accepts a stub for
it.
