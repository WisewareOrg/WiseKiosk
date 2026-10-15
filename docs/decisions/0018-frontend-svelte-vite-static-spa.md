# 0018 — Frontend is Svelte 5 + Vite, a static SPA with no meta-framework

**Status:** accepted
**Decided:** 2026-08-04 (#60 authored-language set. The stack was in use from the bootstrap; the
alternatives below were weighed on this date, and this record carries the date of the weighing rather
than the date the stack was picked)
**Rev:** 1

## Revisions

- **rev 1** — 2026-08-05 — revision tracking begins; text as merged (#118 ADR revisions).

## Context

The frontend stack was in use before anything recorded why. [ADR 0001 rev 1](0001-backend-language-go.md) opens
by treating it as given — *"The frontend is settled: Svelte 5 + Vite, a static SPA"* — and reasons
about the backend from there. [`../../README.md`](../../README.md) and
[`../ARCHITECTURE.md`](../ARCHITECTURE.md) state it. [`../CI.md`](../CI.md) § *Module and framework
structure* **gates** it, refusing among other things a declared SSR adapter — that section states what
the gate asserts, and owns it.

Its only rationale home was `docs/FOUNDATIONS.md` § 2, deleted with #69 tree rebuild. What that table
carried and nothing else does is the premise that would reopen the decision. So the repository was
left holding a gate that forbids a declared SSR adapter with no record of why anyone would want one,
which is the shape of a rule that survives past its reason.

What the frontend has to be is narrow, and every alternative below is measured against it. WiseKiosk
is an unattended display on constrained hardware. It renders a fixed set of modules on one screen and
**never navigates**. It has no crawler, no cold visitor and no share link, so first paint is a
once-per-power-cycle event. Its configuration is a static file fetched from the same origin
([ADR 0007 rev 2](0007-config-validation-allocation.md)), not a form anyone submits. The bundle is served as
static files by the Go backend that already serves the configuration.

## Decision

**Svelte 5, built by Vite, emitted as a static single-page bundle.** One HTML entry, client-rendered,
served as files by the Go backend. **No server-side rendering, no router, no meta-framework.**
[`../CI.md`](../CI.md)'s static-bundle gate is the mechanical form of that sentence; this record is
what it asserts against.

Components are authored in the Svelte component format and everything around them in TypeScript,
which is what [ADR 0017 rev 10](0017-authored-language-set.md) admits for what ships: the
configuration-validation engine of [ADR 0007 rev 2](0007-config-validation-allocation.md) and the generated
boundary types of [ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md) both live here. Vite is an
invoked toolchain under that decision, not an authoring language.

## Alternatives considered

- **SvelteKit**, the application framework over the same compiler. It brings filesystem routing, a
  `load` convention that fetches before a page renders, form actions, a rendering-mode choice, and
  adapters that decide the deployment target. Rejected because every one of those serves a capability
  this product deliberately does not have: routing serves navigation and the display never navigates;
  SSR serves first paint and crawlers, and there is neither; `load` serves pages fetching from many
  places, and this one fetches from its own backend; form actions serve forms, and the configuration
  arrives as a file. Adopting it would also mean rewriting the static-bundle gate above, since
  `adapter-static` is exactly the adapter declaration that gate refuses. **The rejection is cheap to
  reverse, which is why it is safe:** the `.svelte` components port essentially unchanged — same
  compiler, same runes — and what changes is the shell, the entry point and the build configuration.
  That is about a day, not a rewrite.
- **No framework at all** — plain TypeScript, web components, Vite. The honest rival and the smallest
  thing that works: no framework dependency, and the module contract's *"receives its configuration
  and its payload as props and renders"* ([`../contracts/module-contract.md`](../contracts/module-contract.md))
  is a shape custom elements serve natively. Rejected because module state and re-render on payload
  refresh then become hand-rolled reactivity, which is a defect surface that grows once per module
  rather than once. Svelte's runes are that same mechanism, compiled, for single-digit kilobytes.
- **React or Vue.** Rejected on runtime footprint against the hardware: both ship a framework runtime
  and a virtual DOM to the device, where Svelte compiles to direct DOM updates and ships neither.
  Their real advantage is ecosystem size, and that advantage is proportional to the component surface
  it serves — which for a fixed module set on one screen is close to nothing.

## Consequences

- **A gate gains its reason.** [`../CI.md`](../CI.md)'s static-bundle bullet cites this record instead
  of standing on its own. Its allowlist form is deliberate and stays as written: a denylist of named
  routers and meta-frameworks fails open the first time somebody hand-rolls a hash router.
- **The ecosystem cost is real and is accepted.** A component that would have come off the shelf in
  React gets written here. For a fixed module roster that is a bounded cost; it would not be for a
  product with a growing component surface.
- **First paint waits for the bundle and the first fetch.** Acceptable for a display that boots once
  and runs for weeks, and it is exactly the trade that would be wrong for a page a person opens cold.
- **The frontend owns configuration validation** ([ADR 0007 rev 2](0007-config-validation-allocation.md)) and
  consumes generated boundary types ([ADR 0008 rev 6](0008-boundary-contract-openapi-codegen.md)), so the
  TypeScript toolchain is unconditional here whatever else changes.
- **No requirement item states any of this.** The stack constrains the repository rather than the
  running software, which [ADR 0011 rev 2](0011-requirement-or-convention.md) routes to a check —
  [`../CI.md`](../CI.md)'s, above.

**Premise that would reopen this:** the product grows a genuinely interactive surface, of which **a
graphical configuration editor is the named case** — multiple views, forms that submit, and state
that outlives a render. At that point SvelteKit's routing and form actions start earning their cost,
and the components move across for roughly a day's work. A multi-page or SEO requirement would do the
same, and is not expected to arrive. Absent either, do not relitigate.
