# `check-bringup`

The inputs `bring_up.py` has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s § *Deployment and bring-up*; how to run a case is
[`../README.md`](../README.md)'s.

Run against release `v0.1.0` (latest, non-pre-release), digest
`sha256:27ff2637c52fe1e389a23693bb0c5fd8dcce175e9219c49006884ad46d3e589a`. Script md5
`d637a9fe8679bd38dbde24fc9821f97e` at `ca7d796 feat(ci): parse and run the documented bring-up
procedure`. A seed edits a scratch copy of `docs/DEPLOYMENT.md`'s fenced block and is run with
`bring_up.py --doc <copy> v0.1.0 <digest>`, which reaches the real release's assets and the real
image; the tracked `docs/DEPLOYMENT.md` and `deploy/compose.yaml` are never edited. The
registry-propagation poll bound, 60s, is the harness's own constant; no document specifies one.

**#140 image-swap check factored the health poll, the configuration fetch and the manifest-field
resolution this script calls into `scripts/bringup/common.py`, shared with `image_swap.py`; no
step or assertion below changed.** Script md5 of `bring_up.py` after that factoring:
`bb845037be5d9775cbdfaa3b8e892402`, `common.py`: `7c3513a0a267c04ef064608002a173a9`, both at
`7617e79 refactor(bringup): factor the health-poll, config-fetch and manifest-read helpers into
common.py`. The must-pass row and the `latest`-mismatch row were re-run against the factored
script — both reported byte-identical text, in 32.4s and 63s respectively — because both pass
through the moved `imagetools_inspect`/health-poll/config-fetch code; the other four rows fail on
`compose_container` or `run_block`, neither of which moved, and were not re-run.

**Host port 8080 was held for the whole of this branch's work** by an unrelated, longer-running
task's container, which was not this branch's to stop; the recipe hardcodes that port. The
recipe-as-committed row was run in CI instead
([run 34000590390, job `bring-up-ci-confirm`](https://github.com/WisewareOrg/WiseKiosk/actions/runs/34000590390/job/101398633749),
Docker 29.6.2, Compose v2 (5.1.4)), container start to reported success in 31s, well inside the
120s deadline the image's declared healthcheck derives — kept below as corroboration of the local
run that replaced it. The owner authorized stopping that container to record the remaining three
rows (2026-09-06); it was stopped, the four port-dependent rows below were run against the real
recipe locally, this harness's own compose project was torn down completely
(`docker compose down --volumes`, confirmed against `docker ps`/`docker network ls`), and the other
container was restarted afterward. One row's observation differs from what was predicted before
that ruling, noted where it does — the reasoning from source was directionally right (a defect
downstream of the removed line) but wrong about which line fails first.

| Direction | Case | Input |
|---|---|---|
| Must pass | The block as committed | run locally — `the documented bring-up procedure at v0.1.0 reaches a serving deployment` in 32.4s; corroborated by the CI run above (31s) |
| Must fail | `latest` resolving to a digest other than the one this run was handed | passed 64 zeros as the expected digest — ``ghcr.io/tjwise99/wisekiosk:latest resolved to 'sha256:27ff2637…589a' after 60s, expected sha256:000…0 — latest has not propagated to the digest this release published`` |
| Must fail | The `docker compose up -d` line removed | seeded via `--doc`; no container ever starts — `` `docker compose ps -q kiosk` found no container () `` |
| Must pass | `docker compose up -d` spelled `docker compose up --detach` | run locally via `--doc` on the same flag, long form (discovery brief §6) — reaches a serving deployment identically, in 33.0s |
| Must fail | The `cp config.example.json config.json` line removed | run locally via `--doc` — **differs from the prediction this row carried before the port freed**: rather than a directory bind-mount surfacing at `/config.json`, the *next* line fails first, since there is nothing for it to act on — `` `chmod 644 config.json` exited 1 `` (`chmod: cannot access 'config.json': No such file or directory`); no container ever starts |
| Must fail | `chmod 644` spelled `chmod 600` | run locally via `--doc` — matches the prediction: the image's user (uid 10001) does not own the file `cp` creates, a 600 file denies it read access, and `staticserve.go`'s `open()` (`backend/internal/staticserve/staticserve.go:55-66`) folds that permission error into the same `found=false` path a missing file takes — `` `/config.json answered 404, expected 200` `` |

## WI-2 rows: the release-dir interface and the running digest (pending re-run)

Rows for `bring_up.py`'s new interface, `bring_up.py [--doc PATH] <release-dir> <digest>` — the tag
argument and `assert_latest_propagated`'s poll are gone, replaced by `WISEKIOSK_IMAGE` and a
post-run check of the running container's own image digest against `<digest>`
([#418 verify before the release locks](https://github.com/WisewareOrg/WiseKiosk/issues/418#issuecomment-5961738214)).
Each row states the case, how it is seeded or run, and the expected outcome; Evidence is filled once
each is re-run. `<release-dir>` is hand-built for both rows: this branch's `deploy/compose.yaml`
(carrying `image: ${WISEKIOSK_IMAGE:-ghcr.io/wisewareorg/wisekiosk:latest}`) plus `v0.2.1`'s published
`config.example.json`, against digest
`sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2`. The must-fail `latest`
row above, in the table against `v0.1.0`, tests `assert_latest_propagated`; that assertion is gone
from this interface, and its mismatch case is the second row below.

| Case | Seed / run | Expected outcome | Evidence |
|---|---|---|---|
| The documented procedure, run unedited in `<release-dir>`, brings up this release's image | `bring_up.py <release-dir> sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2`, with `WISEKIOSK_IMAGE=ghcr.io/wisewareorg/wisekiosk@sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2` exported to the documented fence — `deploy/compose.yaml`'s `${WISEKIOSK_IMAGE:-...}` resolves to the exported value | the compose service reaches a serving deployment; the running container's own image digest equals `sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2` | |
| A mismatched digest is reported | the same run, but `<release-dir>`'s `compose.yaml` is `v0.2.1`'s own published one instead of the branch's — it hardcodes `ghcr.io/wisewareorg/wisekiosk:latest` rather than referencing `WISEKIOSK_IMAGE`, so the exported variable has no effect and `docker compose up -d` pulls whatever `:latest` currently resolves to | the running container's own image digest does not equal the requested `sha256:55239c9cdf549cfeac87265849bd6277d7bed3619f4bd5766ac55fd00e9e95a2`; `bring_up.py` fails, naming the mismatch | |

**Known gap.** Removing the `chmod 644` line is not seeded: `cp` preserves the tracked 644 mode of
`config.example.json` on the runner, so the line is redundant there and its absence is not a defect
this check can see (owner, 2026-09-04).

**The `bring-up` job's teardown step itself is unverified.** `docker compose down --volumes` under
`if: always()` in `.github/workflows/publish.yml` is outside `bring_up.py` and outside every row
above. The identical command was run by hand, from `bring-up/`, after each of the four local rows
above and tore the deployment down cleanly every time (`docker ps`/`docker network ls` confirmed
nothing left) — but that is this record running it, not the workflow's own step running as a job
step with its own `if: always()` and `working-directory:`. That step has not fired in any CI job
(the `bring-up-ci-confirm` scaffold that produced the committed-block row's CI evidence carried no
teardown step of its own), and stays unobserved in that specific form until the first real
release's job runs it.
