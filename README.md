# WiseKiosk

A config-driven smart-mirror display. A full-screen browser page renders a fixed set of five modules
— `clock`, `compliments`, `weather` (Open-Meteo), `AviationWeather` (CheckWX METAR/TAF), and
`DisneyWaitTimes` (themeparks.wiki) — from a handful of public APIs, running unattended on a display
behind one-way glass. What that glass returns where the display carries nothing is the point rather
than a side effect: the page spends what light it spends on what it presents,
and next to none anywhere else, so the surface it does not fill is mirror (SYS008<!-- The surface carrying no content is a mirror -->). How much that
is follows from the configuration — a deployment that fills the display has chosen to. The page defines the display's regions;
the configuration names which one each module goes in
(SYS002<!-- The display's rendering keeps nothing from a viewer -->,
SRS017<!-- Full-screen assembly at kiosk; reflow, no horizontal scroll, at narrower widths -->); there
is nothing to interact with, it renders and it refreshes. Shipped as one container image; each
deployment is independent, customised through configuration, never through a fork
(SYS003<!-- A deployment is parameterised from outside the image -->). A sixth module is added by
the documented six-part contract in
[`docs/contracts/module-contract.md`](docs/contracts/module-contract.md).

**Minimum specs.** The backend runs on `amd64`, `arm64` and `armv6l`
(SRS019<!-- The backend runs on every supported architecture -->) — a Raspberry Pi, a mini PC, a
spare desktop. The display is driven by a browser on a host of Raspberry Pi Zero capability
(SRS021<!-- Frontend runs on a Pi Zero-class browser host -->). These are the minimum WiseKiosk
declares, and SYS007<!-- The declared minimum host, and staying within it --> is the obligation to
run on them and to keep running there. They are set by what an operator is likely to already have
rather than by what WiseKiosk would prefer — requiring a particular machine would make that choice
for them (SRS019<!-- The backend runs on every supported architecture -->). Which machines an
operator buys, and how they are provisioned, is theirs.

**What WiseKiosk does not own.** The kiosk host lies outside the system: its operating system, its
container runtime, its browser, and whatever starts that browser on boot. Provisioning that machine is
the operator's, and no requirement in the tree reaches it. That bounds what WiseKiosk can be failed
against rather than what it is willing to ship — the release carries a deployment recipe and an example
configuration precisely to shorten the path ([`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)); building a
full appliance image is likewise outside that boundary, and
[`WisewareOrg/meta-wisekiosk`](https://github.com/WisewareOrg/meta-wisekiosk) is the Yocto/kas layer that does
it, producing an image that runs this product.

**The operator is frequently not the author.** Deployments run at the author's house and at friends'
and family's houses — separate networks, separate configs, separate owners. This constraint outranks
every other requirement here: it is why a bad configuration must fail loudly and legibly rather than
as a blank screen (SYS001<!-- Failure is legible and proportionate -->), and why correcting one costs
an edit and a page reload rather than a redeployment
(SRS003<!-- A configuration change applies no later than the next page load -->).

Nothing is inherited from any prior mirror framework, and there is no compatibility layer.

> **Status: bootstrapping (pre-code).** The specification is written; no application code exists yet.

## Stack

- **Backend:** Go — a thin, stateless REST proxy. See [ADR 0001 rev 1](docs/decisions/0001-backend-language-go.md).
- **Frontend:** Svelte 5 + Vite, a static SPA. See [ADR 0018 rev 1](docs/decisions/0018-frontend-svelte-vite-static-spa.md).
- **Boundary:** one schema, both sides generated from it — no hand-maintained parallel types.

## Running it locally

The backend serves the page, `/api/*` and `/healthz` from one origin. These recipes run the whole
app against the working tree — no configuration authored by hand, no edit to the shipped recipe. The
local display configuration is seeded from `deploy/config.example.json` into the gitignored
`frontend/public/config.json` on first run, and edited there; weather is keyless (Open-Meteo), so the
data is live with no secrets to supply.

```sh
just serve          # terminal 1: the backend, reloading on change
just dev            # terminal 2: the page, hot-reloading; /api and /healthz proxy to `just serve`
just run            # one process: build the bundle, serve the whole app on :8080 (no reload)
just run-container  # build the working tree into the image and run it on :8080 (needs Docker)
```

`compose.dev.yaml` is the local container recipe; the release recipe is `deploy/compose.yaml`, whose
bring-up is [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)'s.

## Documentation

**The requirements tree is the specification.** Every obligation WiseKiosk is built against is a
numbered item in [`docs/requirements/`](docs/requirements/README.md) — system needs (`SYS`)
decomposed into testable "shall" statements (`SRS`), each traced to a verification item (`TST`), with
CI failing on a broken chain. If a statement is normative, it has an ID. The documents below mostly
explain, orient, and cite rather than oblige.

- [`docs/README.md`](docs/README.md) — the documentation index: which document guarantees which kind
  of fact, and what each one excludes. Read this before adding to any document.
- [`docs/requirements/`](docs/requirements/README.md) — the specification, and how the tree is gated
  ([ADR 0002 rev 4](docs/decisions/0002-requirements-management-doorstop.md),
  [ADR 0005 rev 4](docs/decisions/0005-traceability-gating.md)).
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — the living description of the system as built
  (a skeleton until code lands).
- [`docs/TESTING.md`](docs/TESTING.md) — the test architecture, written as a specification before any
  tests exist.
- [`docs/decisions/`](docs/decisions/README.md) — the decisions that carried a rejected alternative.
- [`docs/contracts/module-contract.md`](docs/contracts/module-contract.md) — the six-part contract
  for adding a display module.
- [`docs/CI.md`](docs/CI.md) — every check on the repository: what CI provides, what blocks a merge,
  and what each gate is allowed to let through. None of it is a requirement — it constrains the
  repository, not the running system.
- [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) — what a deployment must do to reach a running kiosk and
  to move between published digests: bring-up, the recipe's shipped defaults, and upgrade. Obligations
  on what ships rather than on the running system, so they are here rather than in the tree.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — running the checks and getting a change merged.
