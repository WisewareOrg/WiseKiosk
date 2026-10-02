# `check-restart-policy.py`

The inputs this check has been run against, in both directions. What it *asserts*, and why, is
[`docs/CI.md`](../../docs/CI.md)'s § Deployment and bring-up; how to run a case is
[`../README.md`](../README.md)'s.

Covers both assertions and both guards: the policy's presence, its value, a recipe no service parsed
from, and a `restart` key the walk never read. Script md5 `b9a13206d7a2e3bc5a6b09966cb95ab6` at
`30cb78c health signal`; each row is a scratch tree holding that script and a `deploy/compose.yaml`
of its own, the passing row being the recipe this branch commits.

| Direction | Case | Input |
|---|---|---|
| Must fail | The policy is `always` | the committed recipe with the value replaced — `service 'kiosk' declares restart 'always', expected 'unless-stopped'`. A key present with the wrong value, which a presence test would pass |
| Must fail | No policy at all | the `restart:` line deleted — `service 'kiosk' declares no restart policy` |
| Must fail | A second service with no policy | the committed recipe plus a `sidecar` service — `service 'sidecar' declares no restart policy`, so the assertion is over every service rather than the first |
| Must fail | A policy nested deeper than the service's own keys | `restart: unless-stopped` under a `deploy:` key — two problems: the service declares none, and `a 'restart' key sits outside every service's own keys`. Reading it as the policy would pass a recipe that declares none |
| Must fail | No `services` key | `services:` renamed — `declares no top-level 'services' key, or check-restart-policy.py cannot read this layout — either way no service was examined` |
| Must fail | `services` with nothing under it | a recipe that is one `services:` line |
| Must fail | A `restart` key no service owns | `restart: unless-stopped` under a top-level `x-defaults:`, beside a service that declares it correctly — the guard reports the unread key even though the walk's own verdict passed |
| Must fail | The recipe is absent | no `deploy/compose.yaml` — `is absent, so this read no recipe` |
| Must pass | The recipe this branch commits | `deploy/compose.yaml` — `1 service(s) … declare restart 'unless-stopped'` |
| Must pass | The value quoted | `restart: "unless-stopped"`, which is the same scalar spelled differently |
| Must pass | A trailing comment on the value | `restart: unless-stopped # a sample default` |

**The two guards are what the renamed-`services` and `x-defaults` rows prove.** A walk that parses
no service finds no missing policy and would print success; and a `restart` key the walk never
reached is a layout read differently from how it is written, which the walk's own verdict cannot
report. The second guard shares no assumption with the walk — it is a line scan of the whole file
compared against what the walk examined.

**The seeding found a fail-open in the walk.** The first spelling read every key beneath a service,
so `restart: unless-stopped` nested under a `deploy:` key was taken for the service's policy and a
recipe declaring none passed. The walk reads a service's own keys, and the nested row above is what
holds it there.

**Legal input this rejects.**

- **A flow-style mapping.** `services: {kiosk: {restart: unless-stopped}}` fails as `declares
  'services' with nothing under it`: there is no YAML parser here, and a one-line mapping carries no
  nested key a line scan can see. Fail-closed rather than fail-open.
- **A policy reached through an anchor or merge key.** A service whose policy arrives by `<<: *d`
  from a top-level `x-` block fails both as a service declaring none and on the unread key. The
  effective policy would be right; nothing here resolves an alias.

**Known gaps.**

- **The recipe, never a deployment.** Every row is a committed file; no case observes a running
  container. Which of the two the obligation is on is
  [ADR 0020 rev 5](../../docs/decisions/0020-release-artifact-set-and-operator-tooling.md)'s.
- **One key, and no case covers another.** Nothing else in the recipe is read, which is
  [`docs/CI.md`](../../docs/CI.md)'s § Deployment and bring-up boundary rather than an omission here.
- **`unless-stopped` is asserted as a literal.** A recipe expressing the same policy some other way
  — a `deploy.restart_policy` block, an environment substitution — fails. No case covers a spelling
  the committed recipe does not use.
