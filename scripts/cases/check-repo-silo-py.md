# `check-repo-silo.py`

The inputs this check has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s; how to run a case is [`../README.md`](../README.md)'s.

Covers three assertions: the root listing, the shebang-recipe ban, and the root `renovate.json`
resolving to the pinned `WisewareOrg/wise-renovate` preset.

Re-exercised under #54 container image and publish, script md5
`71b6118115840c94d5d4aa3a5ce1c850`, when the `docker` ecosystem gained a manifest mapping and the
one exception to the non-root rule — [ADR 0021 rev 4](../../docs/decisions/0021-repository-layout.md)
puts the `Dockerfile` at the repository root. Every must-fail row below was re-run against the
changed check; the root row was run once per language ecosystem, so the exception is shown not to
reach them, and the two `docker` rows it added belonged to the per-ecosystem check the Renovate
cutover retired. A passing run over this branch's head reports
`renovate.json` resolving the pinned `WisewareOrg/wise-renovate` preset.

Re-exercised under #260 Exclude the project's own image from Renovate digest pinning, when
`renovate.json` gained a `packageRules` key beside the pinned `extends`. The script inspects
`extends` alone, so the added key passes unread — a passing run over this branch's head reports the
same message as above, `renovate.json` resolving the pinned `WisewareOrg/wise-renovate` preset.

Re-exercised under PR 409 Move the wise-renovate preset to WisewareOrg, script md5
`65bb296f311f74ee8c549bb28db58297`, when the runner repository moved to the `WisewareOrg` organization and
`RUNNER_PRESET_PREFIX` followed it. Every `renovate.json` row below was re-run against the changed
check, and an `extends` naming the preset at its pre-move owner now fails as the runner preset
missing.

| Direction | Case | Input |
|---|---|---|
| Must fail | Manifest at the root | `package.json`, `go.mod`, `pyproject.toml` and `requirements.txt`, each at the repository root |
| Must fail | Environment directory at the root | `.venv/` |
| Must fail | Recipe carries a shebang | a `probe-recipe` opening `#!/usr/bin/env bash`, grouped under `docs` and reachable from no gate — the assertion is over every recipe, not the ones `verify` runs |
| Must fail | The dump names no recipe | `just --dump` returning an empty recipe set, so the loop cannot judge anything |
| Must fail | A module hides a script recipe | `mod deploy` beside a `deploy.just` whose `push` recipe opens `#!` — the dump lists it under `modules`, not `recipes` |
| Must fail | renovate.json missing | `renovate.json` deleted from the repository root |
| Must fail | extends lacks the runner preset | `extends` holds entries, none starting `github>WisewareOrg/wise-renovate` |
| Must fail | extends preset unpinned (no `#`) | `extends: ["github>WisewareOrg/wise-renovate"]`, no `#tag` |
| Must fail | extends a same-prefix repo, not the preset | `extends: ["github>WisewareOrg/wise-renovate-fork#v1"]` — shares the prefix but names a different repository |
| Must fail | file is not JSON | `renovate.json` edited to invalid JSON |
| Must pass | The real `renovate.json` | the tree as it stands, `extends: ["github>WisewareOrg/wise-renovate#v1.5.0"]` |
| Must pass | `renovate.json` carries a `packageRules` key beside the pinned `extends` | the tree as it stands, `packageRules` disabling the `docker` datasource for `ghcr.io/tjwise99/wisekiosk` beside `extends: ["github>WisewareOrg/wise-renovate#v1.5.0"]` |
| Must pass | Manifest below the root | `web/package.json` |
