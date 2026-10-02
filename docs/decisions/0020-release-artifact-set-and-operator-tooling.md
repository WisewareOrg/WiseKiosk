# 0020 — A release is an image in the registry and two files on a tag; it carries no operator tooling program, and the operator's interface to the binary is a fixed port and two flags

**Status:** accepted
**Decided:** 2026-10-02 (#418 publish under immutable releases, extending the 2026-09-05
tag-triggered-workflow rev on #268 release from a manual tag, the 2026-08-19 operator-interface rev
on #9 backend skeleton and the 2026-08-09 design discussion on #71 release artifact set)
**Rev:** 5

## Revisions

- **rev 5** — 2026-10-02 — moves the trigger from `release: published` to `workflow_dispatch` on a
  pushed tag, carrying one boolean input, `prerelease`, so the draft-first shape below can attach
  everything before the immutable-release lock the Decision names takes hold. The workflow creates
  or reuses a draft release for the dispatched tag, fills it over its own later steps, and publishes
  it last; `latest` moves to the published digest in its own job, once `verify` passes, not at build
  and not in the publish job itself — an image failing verification would otherwise already be
  `latest` by the time `verify` caught it. `bring-up` and `image-swap` gain that job alongside
  `verify` in their own gate. image-swap's previous-release lookup excludes drafts, so an in-flight
  one is never picked as the swap target. A run failing before the publish step is re-dispatched on
  the same tag rather than re-cut; a `latest` job failing transiently needs neither, but a failed
  digest assertion is repointed by hand rather than re-run. What the release itself is made of is
  unchanged; the trigger and its ordering move, so the Decided date moves with them
  (#418 publish under immutable releases).
- **rev 4** — 2026-09-06 — corrects "referring artifacts" to "attached under the signing tools'
  conventions" everywhere it appears: GHCR implements no OCI referrers API, so cosign's own tag
  convention and GitHub's attestation store are what the registry-side material actually attaches
  through. Records that the SBOM is one attestation per platform child rather than one for the
  index, and that the index and each child are both signed. Records that the per-child logic is
  authored Python rather than workflow shell. States that `bring-up` and `image-swap` run only once
  the `verify` job passes. What was chosen is unchanged, so the Decided date does not move
  (#269 publish verification).
- **rev 3** — 2026-09-05 — adopts the tag-triggered publish workflow: a release is cut by hand on a
  semver tag rather than published from every commit on the default branch, `latest` moves only for
  a non-pre-release, the release assets are named by basename, and the release notes gain the
  digest line (#268 release from a manual tag).
- **rev 2** — 2026-08-19 — extends the decision to the operator interface the skeleton built: a
  fixed `:8080` and the two flags on the one binary, housed here so the code and
  [`../DEPLOYMENT.md`](../DEPLOYMENT.md) cite them rather than each asserting them (#9 backend
  skeleton).
- **rev 1** — 2026-08-09 — first written (#71 release artifact set).

## Context

A requirement deleted in the tree rebuild (#69 tree rebuild) enumerated the artifacts the project
delivers and forbade delivering any host provisioning. It went for two reasons: the enumeration was a
table of contents for unrelated things that each had obligations already, and it traced to no clause
of the need above it. Its prohibition was struck separately, on the ground that it forbade a case the
owner could foresee wanting. Nothing replaced either half, so the specification neither describes what
a release consists of nor rules on what may ship beside it.

An enumeration returned as a check rather than a requirement — [`../CI.md`](../CI.md) asserts a
release asset set and fails an undeclared asset — but a check asserting the set is what it says does
not decide what the set should be, and that section named this decision as the thing that would.

**Two premises the question had been asked under turned out to be false.**

The first is that a release is a set of downloadable files. Three of the five things the check
enumerated — the SBOM, the signature, the build-provenance attestation — are attached to the image
digest in the registry under the signing tools' conventions (cosign tags and GitHub's attestation
store), which is where `../CI.md`'s verification job already looks. A fourth, the image reference, is not a file at all. One sentence therefore described three
different queries against two APIs while reading as one assertion.

The second is that a tooling bundle had somewhere to go. The project publishes to a container
registry and to a documentation site, and nothing else; every option the ticket contemplated — a
provisioning bundle, a distributed helper — presumed a channel that did not exist. What the ticket
asked was where such tooling should live, and the answer was that there was nowhere to put it.

Underneath both sat a question nobody had answered: what a provisioning helper would do. Installing a
container runtime is per-host work the project does not want; configuring the display host to render
the page depends on particulars of an operator's network that no shipped script can know. What
remained — pull the image, bind a configuration, run it under a restart policy — is what a compose
file is.

**What rev 1 left open.** Ruling out a separate tooling program says what the operator interface is
*not*, and nothing said what it is. #9 backend skeleton then built one — a listening port and two
flags — with no record to put it in, so the port lived as a constant whose comment cited this ADR for
a claim rev 1 does not make, `DEPLOYMENT.md` named a *constant service port* without a value, and
`ARCHITECTURE.md` asserted the port was fixed with nothing behind it. Three documents describing one
observable, none of them deciding it. The flags are the sharper case: they are what a Dockerfile will
be written against, so leaving them unhoused means #54 container build and publish inherits choices
nobody took.

## Decision

**A release occupies three locations, and only two of them carry it.**

| Where | What |
|---|---|
| The registry, at the image digest | the image, with its SBOM, signature and build-provenance attestation attached to that digest under the signing tools' conventions (cosign tags and GitHub's attestation store) |
| The release tag | the deployment recipe, `compose.yaml`, and an example configuration file, `config.example.json`, each at its committed basename |
| The documentation site | nothing. It tracks the default branch and is deliberately not versioned |

**The SBOM is one attestation per platform child, not one for the index.** The index digest is the
named subject of the signature and the build-provenance attestation, and the platform children are
signed too, so an operator who pins a child digest can verify it on its own.

**Per-child signing, attestation and verification logic is authored Python, not workflow shell.**
`scripts/publish/sbom_attest.py`, `scripts/publish/verify_metadata.py` and
`scripts/publish/verify_release.py` iterate the platform children and assert their content; a
workflow `run:` block carrying that same loop would be authored sh, which nothing here authors
([ADR 0017 rev 9](0017-authored-language-set.md)).

**Bring-up and the image swap run only once verification passes and `latest` has moved.** Both
depend on `publish`, `verify` and `latest`, so a release whose signature or attestation fails to
verify, or whose `latest` retag fails, never reaches either exercise. image-swap's own
previous-release lookup excludes drafts (`--exclude-drafts`), so an in-flight draft on another tag
is never picked as the swap target.

The release notes name the digest, which is what ties the tag to the registry: the publish
workflow keeps exactly one line naming it, `Image: ghcr.io/wisewareorg/wisekiosk@sha256:<digest>`,
replacing that line on a re-dispatch before the release publishes — a rebuild changes the digest, so
matching the whole line would not catch it — and touching nothing else in whatever notes the draft
carries.

**The image reference is not an asset.** It is a pointer — the thing the registry-side material
describes and the thing the recipe resolves. Listing it beside four files is what made one sentence
stand for three assertions.

**Releases on a tag are adopted as a third channel.** They cost a workflow step and give the
documented procedure a stable retrieval URL, and they are what makes the asset-set claim in
[`../CI.md`](../CI.md) something a check can decide rather than a description of a release form the
project does not produce.

**A release is cut by pushing a tag and dispatching the workflow.** The owner runs
`git tag vMAJOR.MINOR.PATCH <sha> && git push origin vMAJOR.MINOR.PATCH`, then
`gh workflow run publish.yml --ref vMAJOR.MINOR.PATCH`, with `-f prerelease=true` for a
pre-release; `publish.yml`'s first step asserts the dispatched ref against that shape and fails the
run rather than publishing from anything else. The workflow creates or reuses a draft release for
that tag, fills it over its own later steps, and publishes it once the image, its signatures and
attestations, the two assets and the digest line are in place — with immutable releases enabled,
which the organisation enforces on this repository, GitHub locks a release's assets and its tag the
moment it publishes, though its notes and title stay editable, so a workflow triggered by that
publish could never attach anything to the release that triggered it. The image is tagged by that
same version, and `latest` moves to the published digest in its own job, once `verify` passes, only
when the release is not a pre-release — a pre-release publishes its own version tag and never moves
the tag the committed recipe references
([`../DEPLOYMENT.md`](../DEPLOYMENT.md) § *Bring-up*). The push that published `latest` from every
commit on the default branch is retired with it: nothing publishes from `main`. A run failing before
the publish step is re-dispatched on the same tag — the draft is reused, its `prerelease` flag
re-synced, its assets `--clobber`ed and the digest line replaced; once a release has published, a
bad one is re-cut under a new version rather than re-run. A `latest` job failing before or during
its retag push is transient and needs neither: publish and verify already succeeded, so only the
run's failed jobs are re-run. A `latest` job failing at its own digest assertion is not: the push
already landed, `:latest` already names whatever digest the copy produced, and a rerun would only
repeat whatever made it wrong — that case is investigated, then `latest` is repointed by hand at
the published digest.

**No operator tooling program ships as part of a release**, and the release carries an example
configuration file instead.
A generator that checked its own output would be the second enforcer
[ADR 0007 rev 2](0007-config-validation-allocation.md) forbids by name; one that did not check it would
not answer the difficulty it exists for, which is authoring a configuration without knowing what the
schema offers. The page already renders the full validation report without a valid configuration
(SRS004<!-- Page renders a legible error state for every configuration failure class -->) and the
apply floor is a page reload
(SRS003<!-- A configuration change applies no later than the next page load -->), so what an operator
lacks is a starting point rather than a verdict, and a starting point is a file.

**The image declares a `HEALTHCHECK`.** Nothing acts on its status: a single-host container runtime
restarts a container whose process exits, not one reporting unhealthy, and what tells anyone the
kiosk is broken is
SRS026<!-- The display says when the backend is gone -->, on the screen in the room. It is declared
because it is an oracle three automated checks read, and because an operator wanting a health status
in a container listing would otherwise write the declaration into their own recipe — the same
avoidable step the recipe's restart policy exists to remove.

**The recipe is a sample carrying opinionated defaults.** What ships is a starting point an operator
edits, so a default costs one line to override and its absence costs knowing to add it. The obligation
is on what ships and never on what runs.

**The service listens on `:8080`, fixed in the binary and not a deployment's to set.** Every
deployment serves on that port. It is an internal number two things have to agree on — the binary and
the image's `HEALTHCHECK` — and a container publishes it at whatever host port the operator wants, so
what an operator chooses is the mapping rather than the number.

**The operator interface to the binary is its flags, and there are two.** `-static-root` names the
directory served as the frontend bundle and defaults to `frontend/dist`, so a run from the repository
root needs no argument and a container points it at its own path. `-health-check` asks the local
instance for liveness and exits on the answer, which is what lets the image declare a `HEALTHCHECK`
without carrying an HTTP client beside the binary. Nothing else is read at start-up
([ADR 0007 rev 2](0007-config-validation-allocation.md)). This is the positive half of *no separate
operator-tooling program*: the operator surface is flags on the one binary that already ships, so
there is one program and it is the one being deployed.

## Alternatives considered

**A tooling bundle as a release asset**, which is the shape #71 release artifact set was filed
expecting. Rejected because the work it would automate dissolved on inspection: the runtime install is
out of scope, the display host is out of reach, and the remainder is a compose file. A bundle carrying
one compose file and one example configuration is those two files with an archive around them.

**A compiled binary attached to the release tag.** It fits the language rule for what an operator runs
([ADR 0017 rev 9](0017-authored-language-set.md)) as written and needs no runtime on the host, so it was
the strongest form of a shipped helper. Rejected with the helper itself, and independently by the
single-enforcer rule above: the only thing it could usefully do is emit a configuration, and it cannot
check what it emits.

**A second published image, holding the tooling.** Attractive on the channel argument — one registry,
one publish workflow, the same signature and provenance as the kiosk image for free. Rejected on two
costs. It carries a second supply-chain surface through every scanning, signing and exception-register
gate `../CI.md` describes. And a tool whose output is a file on the host must bind-mount a directory
and match a uid to write it, which is an incantation an operator needs *before* their first deployment
exists. It would also leave
SRS018<!-- One generic published image --> with two referents for a definite article that has one.

**An OCI artifact pushed beside the image**, retrieved with `oras`. It keeps everything on one channel
and inherits the image's provenance. Rejected: it requires a tool nobody has installed to solve a
problem an HTTP client solves.

**Versioning the documentation site per release.** Rejected as more machinery than the drift justifies,
but the drift is real and is recorded below rather than left to be discovered.

**An operator-configurable port**, as an environment variable or a third flag. Rejected on two
grounds. The backend reads no configuration at all
([ADR 0007 rev 2](0007-config-validation-allocation.md)), and a port read at start-up is runtime
configuration however it is spelled, so this would be the first exception to a property stated
without one. And it has no consumer: a container maps the internal port to whatever host port the
operator wants, so the tunable would let them change a number nothing outside the image reads. What
it would add is a second place the port can be wrong — a binary told one number and an image
healthchecking another — in exchange for a choice nobody is waiting to make.

**A configuration editor served as a second frontend bundle.** Not rejected — deferred. It is the only
form of a configuration tool that can validate what it produces, because it runs where the one engine
runs, and it would be version-locked to the image serving it. It costs a second HTML entry, which
[ADR 0018 rev 1](0018-frontend-svelte-vite-static-spa.md) and `../CI.md`'s static-bundle gate both fix at
one. Nothing here forecloses it: ADR 0007 rev 2 already states that a live-apply path is not designed
out.

**Publish on `release: published`, attaching assets and notes afterward.** The shape revs 3 and 4
ran. Rejected: the lock the Decision names means a workflow triggered by that publish can never
attach anything to the release that triggered it.

**The owner creates the draft by hand; the workflow only fills it.** Rejected: create-or-reuse is one
branch of logic either way, so making the workflow decide it unconditionally costs nothing the
manual step would have saved, and it gives the owner one more step to remember on every release.

**The tag push itself triggers the workflow**, `on: push: tags:`. Rejected: a tag push carries no
input, and the one place the flag could ride, a semver pre-release suffix on the tag, is excluded
by the `vMAJOR.MINOR.PATCH` shape the first step asserts; `workflow_dispatch` is the one
manually-triggered event that takes an input.

**`latest` moves at build time, before the release publishes.** Rejected: it would point `latest` at
a draft's image before that draft is known to ever reach publish. A run that failed between build
and publish would leave `latest` resolving to a digest whose release had never published, with
nothing in the recipe's own procedure to notice or correct it.

**`latest` moves in the publish job, right after it publishes the release, before `verify` runs.**
Rejected: an image failing verification would already be `latest` by the time `verify` caught it,
defeating what gating the move on verification is for. The chosen shape moves `latest` in its own
job, after `verify` passes, so a bad signature or attestation is caught before the floating tag ever
points at it.

## Consequences

**One asset-set sentence becomes three assertions.** Verifying the artifacts attached to a digest
under the signing tools' conventions, enumerating files on a tag, and asserting the release notes
name the digest are separate queries against separate APIs, and #67 security and supply-chain gates
inherits them as such.

**The container build inherits a constraint from the healthcheck.** A declared `HEALTHCHECK` needs
something inside the image able to reach the service over HTTP. A distribution base carries one; a
minimal base does not, and the check then needs a self-check path in the backend binary.
[ADR 0015 rev 4](0015-container-toolchain-and-image-annotations.md) leaves the base image undecided, so
#54 container build and publish takes this cost with that choice rather than meeting it as a surprise.

**An operator on an older digest reads a procedure describing the default branch.** The site is
continuously deployed and the release is not, so the two drift by construction. Accepted: the remedy is
versioning the site, and this is the record that the exposure was chosen rather than overlooked.

**The tooling document dissolves**, and only half of that follows from an existing rule. Every
assertion about a check goes to [`../CI.md`](../CI.md), which is
[ADR 0011 rev 2](0011-requirement-or-convention.md)'s. The remainder does not: that record's table is
closed over the requirements tree, `CI.md` and the review checklist, and an obligation on what ships
beside the image is none of the three. It goes to [`../DEPLOYMENT.md`](../DEPLOYMENT.md), which claims
deploying rather than tooling — **an allocation taken here rather than inherited**, and the gap it
fills predates this record, since the tooling document sat outside that table too.

**#70 configuration generator loses its delivery channel, not its ticket.** What is decided here is
what a release carries, and a tooling program is barred from it; whether a generator should exist at
all is that ticket's to rule. The argument against one does reach it — the page renders the report,
the apply floor is a page reload, and a generator cannot check its own output without becoming the
second enforcer [ADR 0007 rev 2](0007-config-validation-allocation.md) forbids — and it is the same
argument that retired the desk configuration validator. Its obligation left with the tooling document
and returns rewritten if that ticket rules the generator survives, since what the document stated was
a program this record bars from the release.

**How secrets reach a deployment is not decided here.**
SYS003<!-- A deployment is parameterised from outside the image --> obliges them to come from the
deployment environment, and the recipe must carry whatever channel that turns out to be, but the
mechanism is #73 secret delivery mechanism and #74 keeping secrets out of client output. The recipe
ships incomplete in that respect until they land.

**One record decides the port, and three places point at it.** `backend/cmd/main.go` holds the
constant it compiles to, [`../DEPLOYMENT.md`](../DEPLOYMENT.md) names the port the `HEALTHCHECK`
reaches, and [`../ARCHITECTURE.md`](../ARCHITECTURE.md) states it is fixed rather than a deployment's
to set — each citing this record rather than standing the value up on its own.

**#54 container build and publish inherits a target rather than a set of guesses.** The image
publishes the port fixed above, its `HEALTHCHECK` runs the binary's `-health-check`, and it points
`-static-root` at wherever it puts the bundle. Changing any of the three is a rev of this record
rather than an edit to a Dockerfile.

**Premise that would reopen this:** a helper appears with work to do that a compose file cannot
express — which today means the display host becoming reachable by a shipped script, or a
configuration surface large enough that a starting file stops being a starting point. A wish for a
more convenient install is not that premise. A separate premise reopens the draft-first publish
order: immutable releases being lifted from this repository, which exists only because a published
release's assets cannot change.
