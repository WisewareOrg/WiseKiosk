# Deployment

What a deployment must do to reach a running kiosk, and to move from one published digest to the next.

**None of this is a requirement.** A requirement obliges WiseKiosk itself. A deployment is something an
operator assembles from what the project publishes, and every obligation below is on what ships or on
the procedure that documents it — not on the running system, which cannot violate any of them. The
product's obligations live in [`requirements/`](requirements/README.md)
([ADR 0011 rev 2](decisions/0011-requirement-or-convention.md)).

**What asserts each of these is [`CI.md`](CI.md).** An obligation is stated here; the check that
decides it is described there, which is where every check on this repository is described.

**What a release consists of is
[ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md).** #54 container build
and publish ships the image, the deployment recipe, the example configuration and the health signal
the image declares. #269 publish verification ships the signature and the attestation the optional
verification below reads, and produces what that step verifies against.

## What an operator receives

The image and its provenance material from the registry, and two files from the release tag: the
deployment recipe, and an example configuration. [`CI.md`](CI.md) § *Publishing and provenance* is
where that set is stated and what is asserted of it.

Nothing else is delivered. The host's operating system, its container runtime, its browser and
whatever starts that browser are the operator's, and no requirement in the tree reaches them.

## Bring-up

A deployment reaches a working state on a clean host from the published artifact and the documented
procedure alone — the executable form of *nothing required is missing*. If a clean environment can
deploy by following only in-repo documentation, the documentation is sufficient.

Two things the procedure owes, each of which fails a first deployment when it is missing.

**The example configuration is copied, not edited in place.** The copy is what a deployment binds; the
example stays as the reference an operator can read a broken configuration against. Only the
configuration must be copied — the recipe runs as it ships.

**The configuration file is readable by the image's user.** The image runs as a non-root user
(SRS020<!-- Non-root container user -->), so a configuration written root-owned and unreadable to
others is fetched by nothing, and the display renders the unfetchable case of
SRS004<!-- Page renders a legible error state for every configuration failure class --> rather than
the configured kiosk. The failure is legible and it is still a failure the procedure can prevent, so
the permissions the file needs are stated where an operator sets them.

**The procedure is three commands**, run in a directory holding the release's two assets —
`compose.yaml` and `config.example.json` — which a checkout carries at `deploy/`. CI runs it verbatim
against this exact image, before the release can lock (#138 bring-up check, #418 publish under
immutable releases).

```sh
cp config.example.json config.json
chmod 644 config.json
docker compose up -d
```

The copy carries no path because the recipe binds `./config.json`, resolved against the directory the
recipe sits in. `644` is what the second obligation above costs: the image's user is uid 10001, owning
neither the copy nor its group, so the world-read bit is the one that decides whether the page is
served a configuration at all. Nothing else is edited and `up` is given no argument: the image
reference, the port and the mount are all the recipe's. #138 bring-up check is what runs this sequence
against a clean host and fails on a step that does not.

`config.json` is the installation's own and carries its coordinates, so it is never committed: a
checkout ignores `deploy/config.json`, and `config.example.json` is the tracked shape.

**The image reference is a movable tag, by default.** The recipe reads
`image: ${WISEKIOSK_IMAGE:-ghcr.io/wisewareorg/wisekiosk:latest}`, which without `WISEKIOSK_IMAGE` set
tracks the newest non-pre-release to pass verification ([`CI.md`](CI.md) § *Publishing and
provenance*), so `up` against a re-pulled tag brings up the newest such image. An operator wanting a
fixed one exports `WISEKIOSK_IMAGE=ghcr.io/wisewareorg/wisekiosk@sha256:<digest>` before running `up`
— the same variable the verification note below names — rather than editing the recipe.

**Releases are cut by pushing a tag and dispatching the publish workflow.** An operator does
nothing to produce one: the repository owner runs
`git tag vMAJOR.MINOR.PATCH <sha> && git push origin vMAJOR.MINOR.PATCH`, then
`gh workflow run publish.yml --ref vMAJOR.MINOR.PATCH`, adding `-f prerelease=true` for a release
that publishes its own version tag without moving `latest`. How a release is cut, what the workflow
does to it, and how a failed run is recovered are
[ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md)'s. A failed
verification fails the run before the release ever locks, leaving it a draft; the owner re-dispatches
on the same tag rather than cutting a new version.

**Optional, and recommended: verify the image before trusting it.** An operator who pulls an image is
trusting a stranger's build, so the check against its signature and provenance ships with the
procedure rather than having to be reconstructed, and it names the repository the image is expected to
have been built by.

```sh
gh attestation verify oci://ghcr.io/wisewareorg/wisekiosk@sha256:<digest> --repo WisewareOrg/WiseKiosk \
  --signer-workflow WisewareOrg/WiseKiosk/.github/workflows/publish.yml
```

`<digest>` is the value the release page's `Image:` line names, and what the command reads is the
build-provenance attestation the publish workflow produces for that digest. That CI verifies its own
published output is a separate assertion and is in [`CI.md`](CI.md).

**It is nobody's obligation.** An operator who skips it runs the three commands above and reaches the
same working deployment; the recipe names a tag rather than a digest by default, so an operator who
wants the digest they verified to be the one that runs exports `WISEKIOSK_IMAGE` set to it — which is
what the recipe
being a sample rather than an obligation
([ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md)) leaves them free to do.

## The deployment recipe

The recipe is a **sample carrying opinionated defaults**, a starting point an operator edits
([ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md)). The obligation is on
what ships, never on what runs — an operator who edits the recipe, or deploys without it, has made
their own choice, and WiseKiosk has no way to override it.

It carries restart policy `unless-stopped`, so a deployment that comes back after a host reboot is
what following the documented procedure gets. `unless-stopped` rather than `always` because the
latter also overrides a deliberate manual stop. Coming back after a reboot additionally depends on the
Docker daemon being enabled at boot, which is host provisioning and outside what the project delivers.

The recipe carries no user, because the image declares one and a second declaration is a second place
for it to drift from.

**What the recipe cannot yet carry** is the secret channel.
SYS003<!-- A deployment is parameterised from outside the image --> obliges secrets to arrive from the
deployment environment; which mechanism delivers them is #73 secret delivery mechanism and #74 keeping
secrets out of client output, and the recipe is incomplete in that respect until they land.

## The health signal

The image declares a `HEALTHCHECK` against the service port and runs it through the `-health-check`
self-check flag. [ADR 0020 rev 5](decisions/0020-release-artifact-set-and-operator-tooling.md) fixes
both, and is where the port's number is written — a recipe reads it there rather than carrying a
copy that a later rev would leave behind. The check is healthy while the backend is serving,
unhealthy when it is not, including when the process is alive but wedged.

**The wedged case is answered within two seconds.** The self-check bounds its request at a two-second
client timeout, so an instance that accepts a connection and never answers is *reported* unhealthy
instead of leaving the check hanging with no verdict at all. The bound sits in the binary rather than
in a `HEALTHCHECK --timeout` the recipe carries, because that is the difference between the promise
above holding by construction and holding only where whoever wrote the declaration remembered a flag.
Two seconds is read off what the probe does: a loopback request to a handler that reaches no upstream
and consults no configuration answers in microseconds or does not answer at all, so anything slower
is the wedge rather than a slow reply.

**Nothing acts on the status.** Docker and Compose restart a container whose process exits, not one
reporting unhealthy, so an unhealthy kiosk stays unhealthy until somebody intervenes.
What tells an operator the kiosk is broken is the kiosk —
SRS026<!-- The display says when the backend is gone --> obliges the display to say so, on the screen
in the room, which is the surface somebody is actually looking at.

It is declared for two smaller reasons. It is an oracle three automated checks read — bring-up,
upgrade, and TST011<!-- Two instances from one image share no state --> in the requirements
tree — each of which needs a machine-readable *is it serving yet* rather than a scraped log line. And
an operator wanting a health status in a container listing would otherwise write the declaration into
their own recipe, which is the avoidable step the restart policy above exists to remove.

**Reopen premise.** This sits outside the requirements tree because nothing acts on it. If WiseKiosk is
ever run under an orchestrator that restarts on health status, or external monitoring is pointed at it,
the signal becomes a control rather than an oracle and the obligation belongs in the tree.

## Upgrade

Moving a deployment from one published digest to the next is a scripted act. The operator supplies the
same mount arguments they already use; nothing asks them to read a diff, edit code, or rebuild.

**A newer image applies its own schema.** One image serves every deployment
(SRS018<!-- One generic published image -->) and validation runs in the page against the schema that
image ships ([ADR 0007 rev 2](decisions/0007-config-validation-allocation.md)), so a configuration the
newer image cannot accept is not silently part-applied and is not merged over with defaults: it fails
validation and the page reports which failure occurred, under
SRS004<!-- Page renders a legible error state for every configuration failure class -->. The apply
floor is a page reload (SRS003<!-- A configuration change applies no later than the next page load -->),
so the correction costs an edit and a refresh rather than a redeployment.

That is the whole of the position. **No promise is made that a newer image accepts an older
configuration** — the project does not undertake to keep every key it has ever offered. What it
undertakes is that an incompatibility is legible at the display rather than silent, which is the
property an operator who is not the author can act on.

## What is not here

**What the kiosk does.** Rendering, configuration handling at run time, upstream fetching and failure
behaviour are obligations on the running software and live in the requirements tree.

**What CI publishes and asserts.** The release material, the gates over it, and what each is allowed to
let through are [`CI.md`](CI.md)'s. An obligation here is on a deployment; a check there is something
CI runs.
