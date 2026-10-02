# 0021 — Lay the repository out as the container decomposition: a root per container, and a home for what belongs to neither

**Status:** accepted
**Decided:** 2026-08-30 (the configuration-schema fragment dropped from the module directory,
#156 config-schema composer; the layout itself taken 2026-08-12 at #5 repo layout, once #97 C4
phase 2 Container closed and the decomposition this projects landed as
[ADR 0019 rev 7](0019-boundary-at-what-deploys-and-tag-tier.md))
**Rev:** 4

## Revisions

- **rev 4** — 2026-09-15 — drops the "committed" clause and the "committed because the gate is a
  `git diff`" causal sentence: [ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md) generates
  the boundary types at build rather than committing them. The layout claim itself — which package
  each generated type lands inside — is unchanged, and is what
  TST034<!-- Both sides consume the generated boundary types --> still relies on. What was chosen is
  unchanged, so the `Decided` date does not move (#339 generated-code-not-committed).
- **rev 3** — 2026-09-04 — restates the three Dependabot-entry mechanism passages as the rule holds
  under Renovate: manifests live in their package-root directories, which Renovate discovers there
  without per-directory configuration; the silo gate asserts the root holds no manifest and that
  `renovate.json` names the pinned preset; and the Dockerfile's base-image reference at the root is
  tracked by Renovate's `docker` manager. What was chosen is unchanged, so the Decided date does not
  move (#223 renovate cutover).
- **rev 2** — 2026-08-30 — the configuration schema is one central file under
  `frontend/src/config/` and a module directory holds the component and the render test only; the
  per-module configuration-schema fragment is gone (owner, 2026-08-30). This changes what was
  chosen, so the `Decided` date moves with it. Nothing else in the layout moves (#156 config-schema
  composer).
- **rev 1** — 2026-08-12 — first written (#5 repo layout).

## Context

**Documents across the tree deferred a location to this decision by name.**
[The module contract](../contracts/module-contract.md) states a module's parts and leaves their
concrete locations — which directory holds a module's files, and where the registration list lives —
to the repository layout. [ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md) chose the
boundary-contract mechanism and left the schema's location open, which is what held #7
boundary-contract codegen from building it. No element of the architecture model carried a source
`link`, and where that source would sit was undecided. So the layout was a blocking dependency rather
than a matter of tidiness,
and what it unblocks is a set of paths other work resolves.

**Half the answer is already forced, by a rule written for another reason.**
[`../CI.md`](../CI.md) § *Repository shape* gates that a depth-1 listing of the root holds no
`go.mod`, `package.json`, `pyproject.toml`, `requirements*.txt` or `.venv/` — tooling is siloed with
the feature it serves — and that the root `renovate.json` parses as JSON and names the pinned
`wise-renovate` preset. Manifests live in their package-root directories, which Renovate discovers
there without any per-directory configuration. Neither package root can therefore be the repository
root.
What is left to decide is what the roots are called, what sits in neither, and where a module's files
go.

**A repository layout is a projection of the container decomposition onto directories**, which is why
this decision waited on one. [ADR 0019 rev 7](0019-boundary-at-what-deploys-and-tag-tier.md) decided
two containers behind one origin, and that the provisioning material shipped beside the image falls
outside the boundary altogether. The third thing the projection has to hold is older: the boundary
schema belongs to neither container, which is
[ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md)'s and which ADR 0019 rev 7 reasons from
rather than decides.

**The module is where the projection stops being obvious.** A module is "added and removed as a unit",
and its parts run in two different execution contexts — a shaping library in the backend, a component
in the browser. A local module fetches nothing, so it has no backend half at all; which modules those
are follows from the roster in [`../../README.md`](../../README.md), which this record does not copy.
Whatever this decides has to leave the checks
[`../CI.md`](../CI.md) § *Module and framework structure* describes — module directories in bijection
with test files, and one registration entry per upstream-backed
module — walking populations that can be read off the tree.

**Generated output has no say in where it sits.** A compiler reads a package where the package is, so
the Go and TypeScript types [ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md) emits are
inside the package that consumes them whatever this record prefers.

## Decision

**The top level projects the containers.** One directory per container, a third for the contract
between them, and a fourth for what ships outside the boundary. The repository's own material —
`docs/`, `scripts/`, `.github/` — was never part of that projection: it describes and checks the
product rather than running in it.

- **`backend/` is the Go module root.** `go.mod` sits there and nowhere else; `cmd/` holds the
  binary's entry point and `internal/` the shared framework half, which is every component the
  Backend container draws.
- **`frontend/` is the npm package root.** `package.json` sits there; `src/` holds the sources Vite
  builds, with the framework half under `src/lib/`.
- **`boundary/openapi.yaml` is the one boundary schema.** It is owned by neither package
  ([ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md)), so it is inside neither, and the
  directory is named for what the repository already calls the thing.
- **`deploy/` holds what a release carries beside the image** — the deployment recipe and the example
  configuration file ([ADR 0020 rev 5](0020-release-artifact-set-and-operator-tooling.md)). Outside
  both packages, because it is outside the boundary: it runs nowhere the system runs.
- **The `Dockerfile` is at the repository root.** A build that produces one image from a Go binary and
  a built bundle has a context spanning both package roots, so the file belongs to neither of them; of
  the places left, the root is the one a build finds without being told, `PATH/Dockerfile` being what
  `docker build <PATH>` reads by default. That is a convention rather than a constraint — `-f` names a
  Dockerfile anywhere and `COPY` resolves against the context regardless — and it is chosen because
  the default is what a reader and an unfamiliar builder both reach for first. The root-manifest rule
  above does not reach it: a Dockerfile is not a dependency manifest, and nothing resolves it by
  walking up from a package.

**A module's files sit in each package that runs one, under the same name.**
`backend/internal/modules/<name>/` holds the shaping library and its unit tests;
`frontend/src/modules/<name>/` holds the component and the render test. No configuration lives
there: a module's configuration keys are a section of the one configuration schema below, not a
file beside the component. A local module has the second only. The directory name is the module's
name and is identical in both trees, which is what lets a check pair the halves without a mapping
table to maintain.

**The single route registration is framework code and sits outside `modules/`**, in
`backend/internal/registry/`. The reason is the populations above: every child of `modules/` is a module,
so a check that walks it is reading a directory listing rather than a listing minus the entries it
knows to skip — and a listing minus known exceptions is the shape that goes quietly wrong when
somebody adds a second exception.

**Generated boundary types land inside the package that compiles them**, each in one package
of its own named for the boundary — `backend/internal/boundary/` and `frontend/src/lib/boundary/` — so
that "the generated package" is a place rather than a pattern, which is what
TST034<!-- Both sides consume the generated boundary types --> asks a type to resolve into.

**The configuration schema is the frontend's, and it is one file**, under `frontend/src/config/`,
beside the one engine that enforces it ([ADR 0007 rev 2](0007-config-validation-allocation.md)).
It is authored there and nowhere else: no part of it sits in a module directory, and nothing
assembles it from parts, so the file a reader opens is the file the page validates against. A
module's keys are a section within it — the same *section of one document* shape the boundary
schema already uses for a module's payload. Its format, its dialect and its file name are #8
config schema format's, and nothing here constrains them.

**What this does not decide.** Every path another document defers here is named above. The framework
packages beyond them — whatever holds the response cache, the upstream client, the page shell —
follow each toolchain's conventions and arrive with the code that needs them; an inventory of them
here would be a list nothing compares to the tree. Nor does this decide any of the module
roster's contents, which is [`../../README.md`](../../README.md)'s.

## Alternatives considered

**One `modules/<name>/` root holding both halves of a module.** The strongest rival, and it makes the
module contract's *"added and removed as a unit"* literal rather than a property two checks maintain:
one directory to add, one to delete, both halves visible together. Rejected on what it costs to reach.
Go source has to be inside a Go module, so this needs either a root `go.mod` — which
[`../CI.md`](../CI.md) § *Repository shape* forbids — or a second module plus a workspace, and a Vite
build reaching outside its own root. Then the population argument runs the other way: a local module
has no backend half, so its directory would hold frontend files only while the shared name promises
both. What actually keeps a module removable is the dependency direction the
contract states and the bijection checks that enforce it, neither of which is adjacency on disk.

**The repository root as the Go module root**, with the frontend nested under it. It is what a Go
project looks like when Go is the only language. Rejected by the silo rule above, and independently by
what it would say: a repository whose specification and architecture precede its code is not a Go
project with documentation inside it.

**`api/openapi.yaml`**, the widely used Go project-layout convention for exactly this file. Rejected
because it names the wrong thing: sitting beside `backend/`, an `api/` directory reads as the API the
backend serves — the ownership [ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md) refuses,
since a schema owned by one side is a schema that side can change alone. *Boundary* is the word this
repository already uses for the contract and for the types generated from it.

**`openapi.yaml` at the repository root**, expressing "owned by neither" as strongly as a path can and
adding no directory for one file. The honest position is that it works, and that the directory holds
one file with no second occupant anyone can name — ADR 0008 rev 6 puts each generator in a package
toolchain, reading the schema from outside both, so nothing there is waiting for a home. It is rejected
on the top level's own shape rather than on a future file: the roots here each name what they hold,
and the loose files beside them are the repository's front matter — the README, the licence, the task
runner. A schema is neither, and a root entry that is neither is the one a later reader has to be told
about. The `Dockerfile` is exactly such an entry and it is there anyway, on the one ground that does
not transfer: a build reads `PATH/Dockerfile` by default, so the root is where an unfamiliar builder
looks without being told. Nothing resolves a schema by convention, so the same argument buys
`openapi.yaml` nothing.

**The configuration schema in a neutral directory beside the boundary schema.** Two schemas, one place
to look, and the symmetry is genuinely easier to explain. Rejected because the symmetry is false. The
boundary schema is neutral because two independently compiled sides generate from it; the configuration
schema has one owner and, by [ADR 0007 rev 2](0007-config-validation-allocation.md), exactly one
implementation may enforce it. A neutral home advertises a shared artifact, and the first consumer that
takes up the invitation is the second enforcer that decision forbids by name.

**The registration file inside `modules/`.** Tempting, since it is the one framework file that names
every module. Rejected on the population argument in the Decision: it makes the module set a directory
listing minus one known entry.

**`pkg/` for the shared framework, with `internal/` for the rest.** The convention exists for code
other modules import. Nothing outside this repository imports any of it, and `internal/` refuses that
mechanically rather than by intent, which is the stronger statement for a private application.

**Say nothing about the `Dockerfile` or the release material**, on the ground that the tickets building
them will decide. Rejected: each would then invent a path at build time, which is the plausible
invention [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md)'s design-first rule exists to prevent, and
the second one to be invented would have to argue against the first rather than against a record.

## Consequences

- **#7 boundary-contract codegen and #8 config schema format unblock**, each gaining a location and
  neither gaining a format: the schema files have homes, the generated packages have homes, and what
  goes in them is still those tickets' to decide.
- **Each package root becomes a tooling silo** with its own manifest, lockfile and install step,
  joining `docs/requirements/`, `docs/site/` and `docs/architecture/`. That is the existing pattern
  rather than a new one, and it is why the root-manifest rule reads as a constraint here instead of an
  obstacle.
- **Renovate discovers a new package root's manifest without any entry to add.** Manifests live in
  their package-root directories, and Renovate discovers them there without per-directory
  configuration, so nothing in `renovate.json` needs updating when a package root's manifest arrives.
- **A `README.md` in any of these roots needs a row in [`README.md`](../README.md).** Under
  [ADR 0014 rev 4](0014-documentation-index-claims-documents.md) a row is the only thing that claims a
  document, the sole exception being a top-level dot-directory, and none of these roots is one. A new
  top-level directory is cheap; a document inside one is not, and that is deliberate.
- **The Dockerfile's base-image reference needs no home under the rule above.** It sits at the
  repository root, and Renovate's `docker` manager tracks it there directly, with no per-ecosystem
  entry mapping a directory to a manifest. Recorded for #54 container build and publish, which is
  where the Dockerfile's root placement lands.
- **The model's `link` properties gain their targets with the source they point at.** This record
  supplies the layout a `link` needs; no source exists, and that is what keeps every element without
  one. How ADR 0019 rev 7 words its own deferral is that record's to change, so this one does not
  count its reasons — a claim about another record's argument goes stale when that record is revised,
  and a pinned citation is green either way, the pin being current while what it is attached to is
  not.
- **Almost none of this is gated.** The root-manifest half is enforced today; the rest is a convention
  that becomes checkable as the module and framework structure checks
  [`../CI.md`](../CI.md) describes are built against these paths. Until then review is the control,
  which is the ordinary division here rather than a weakness of this record.

**Premise that would reopen this:** a third container, or a module shape whose parts do not split
across the two — the push-transport module [the module contract](../contracts/module-contract.md)
already names as a shape it does not fit is the candidate, since a connection manager is framework code
with no half in either module directory.
