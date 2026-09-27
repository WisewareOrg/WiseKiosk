# Module contract

A **display module** is one unit of the kiosk display, added and removed as a unit. It renders into a
region the configuration names for it; regions are the page's and a region may carry several modules
(SRS017<!-- Full-screen assembly at kiosk; reflow, no horizontal scroll, at narrower widths -->). This
page is the contract: what a module is made of, and what an author does to add one.

A module is added by editing the repository: writing the module's files, and — where it is
upstream-backed — adding one registration entry. There is no mechanism to register with at runtime.

The concrete locations — which directory holds a module's files, and where the registration list
lives — are fixed by the repository layout
([ADR 0021 rev 4](../decisions/0021-repository-layout.md)). This page names the parts, not their
paths.

## Two module shapes

A module is **upstream-backed** or **local**. An upstream-backed module fetches an external data
source and has all six parts below; a local module renders from something already present — the
browser's own state, or its configuration — fetches nothing, and has no shaping library, no route
registration and no boundary-schema fragment, there being no upstream to shape and nothing crossing
the boundary. Each part and each build step below carries the shape it applies to.

## How a module is tracked

A module is authored in **three phases, and filed as an epic** — a `module`-type parent ticket
anchoring an integration branch `module_<n>-<name>`, decomposed into three `task` sub-issues whose
pull requests target that branch rather than `main`
([ADR 0013 rev 4](../decisions/0013-work-tracking-invariants.md) defines a sub-issue as exactly that
shared merge target). The phases are not new kinds of work, so they take no new branch type: they are `task`s
named for the phase, and what distinguishes them is the sub-issue they are, not their prefix. The
integration branch keeps the module's churn off `main` and lets a decision taken in one phase be
revised in another before the whole lands — it merges to `main` when the module is done.

The three phases, in order, map to three sections of this contract (which appear below in a different
order — content, not sequence, decides where each sits):

1. **UI design spec** — [§ The module's UI design spec](#the-modules-ui-design-spec).
2. **Requirements & architecture** — [§ Writing the module's requirements](#writing-the-modules-requirements)
   and [§ Drawing the module in the architecture model](#drawing-the-module-in-the-architecture-model);
   this phase settles the source, cadence, config keys and freshness/rate bounds.
3. **Implementation** — [§ Building the module](#building-the-module).

Because the phases share the integration branch, a spec citation the UI design spec cannot yet make —
the requirement it realises does not exist until phase 2 — is firmed up as that phase lands, not
blocked on before phase 1 begins.

## A module is its capability, not its supplier

A module is identified by **what a viewer gets from it** — weather, the time, aviation conditions —
and never by the service it happens to read. The supplier is an implementation detail: a module
reads one source, and changing which one is an edit to this repository rather than a choice an
operator makes or the module arbitrates at runtime. So a module named for its supplier has put a
swappable detail where its identity belongs, and swapping the supplier would rename the module, its
roster entry and the configuration key that names it.

**No requirement names a supplier**, in the need or anywhere in the decomposition. A requirement
naming the party would make replacing that party a specification change, and would tie a viewer's
want to a company the viewer has never heard of. A module's items are written against *a weather
source*, *the source*, *its source*.

**The supplier is named in the concrete-upstream layer, and only there**: the external system
element the architecture model draws, and the route, cache and rate-bucket key that identify the
upstream in code. Those are facts about what runs rather than obligations on it.

The roster in [`README.md`](../../README.md) already carries the convention — a capability name with
its current supplier beside it, `AviationWeather` for CheckWX and `DisneyWaitTimes` for
themeparks.wiki — and a new module joins it the same way.

## The six parts

1. **A Svelte component** *(every module).* Receives the module's configuration and its payload as
   props and renders them into the module's region; it fetches no data, parses no configuration and
   validates no payload. It receives one prop more, `reachable` — the page shell's answer about
   whether the backend is serving — and what a module does with it is
   [§ An unavailable module and an unreachable backend are different states](#an-unavailable-module-and-an-unreachable-backend-are-different-states).
   Where the module has a payload, the component consumes the type generated from the boundary schema
   rather than one declared by hand, **and so does whatever it renders a failure from**: the bodies a
   failing route answers with are declared in that same schema and generated on both sides, so a
   module reading a failure reads the generated type rather than a shape restated in the frontend
   ([ADR 0008 rev 6](../decisions/0008-boundary-contract-openapi-codegen.md);
   SRS016<!-- Both sides consume the generated types -->). A payload's field names, a request's field
   names and a failure body's field names are all values crossing the boundary, and none of the three
   is declared twice.
2. **A configuration-schema section** *(every module).* Declares what this module accepts, as a named
   section of the one configuration schema — authored there rather than in a file of its own, and
   nothing recomposes it — and enforced at apply time in the page, which is where validation runs, per
   [ADR 0007 rev 2](../decisions/0007-config-validation-allocation.md). It does not cross the
   frontend/backend boundary.
3. **Tests** *(every module).* A render test for the component, and — for an upstream-backed module —
   unit tests for the shaping library, and fuzz targets for the shaping library's parser and for the
   route's request decode and validation, each run by `check-fuzz` and each wrapping its own call in
   a per-input wall-clock deadline that fails the test and names the target if a hang reaches it
   ([`CI.md § Backend fuzz`](../CI.md#backend-fuzz)). What they must cover is
   [`TESTING.md`](../TESTING.md)'s, not this document's; that the tests exist and sit where the
   runner reaches them is gated
   ([`CI.md § Module and framework structure`](../CI.md#module-and-framework-structure)), and the
   pending `TST` items they land against are written with the module's decomposition
   ([§ Writing the module's requirements](#writing-the-modules-requirements)).
4. **A shaping library** *(upstream-backed only).* Builds the module's upstream request URL, and
   parses and reshapes the upstream response into the frontend payload. Pure functions, no I/O,
   exercisable in isolation against a captured upstream response without network access — which is
   what the Unit tier in [`TESTING.md`](../TESTING.md) rests on.
5. **A route registration** *(upstream-backed only).* Exactly one entry in the static, compile-time
   list: one field naming the module's own route type, which is what binds the schema's path for this
   source to the library above. The entry is written **in the module's own package**, beside the
   shaping library it is assembled from, and carries every policy governing the route — success cache
   TTL, negative cache TTL, rate limit, outbound timeout, and maximum accepted upstream response
   size. Those values live in the entry and nowhere else in code. **The two cache TTLs are read out
   of the module's requirements**, which carry each value with its rationale; the rate limit, the
   outbound timeout and the response size ceiling leave the enumeration by the other route, as the
   module's own free choices with a written record
   ([§ Writing the module's requirements](#writing-the-modules-requirements)). They live there rather
   than in a framework default because a figure chosen against one source is that source's, and a
   second module arriving at the same figures is what would make them worth holding centrally.

   The module provides, in that same package, **the route's schema handler**: the method the
   generated server interface declares for this path, whose body reads the request and hands it to
   the route built from the entry. Reading the request is the module's because what a request carries
   is the module's — the constraint its location must satisfy, and the key its answer is cached and
   rate-budgeted under, are both named in the module's requirements and neither is a framework
   universal. **A field the schema fragment marks `required` obliges nothing on the Go side**: the
   generator emits a plain scalar, which has no spelling for absence, so a body that omits the field
   decodes to that type's zero and reaches the module's own judgement as a value the caller never
   sent. Establishing that a request named a value is the handler's, before the value is judged.
   **The registration entry of this part is that one route-type field**, and it is the whole of what
   the module costs the shared tree.
6. **A boundary-schema fragment** *(upstream-backed only).* Declares what this module puts across the
   boundary **and what it reads back across it**, as named components in the one boundary schema —
   sections of that schema rather than files of their own, and nothing recomposes them
   ([ADR 0008 rev 6](../decisions/0008-boundary-contract-openapi-codegen.md)). One component is the
   module's payload; the other is the request it answers, which the route carries as a JSON body
   rather than as request parameters. These are what make the module's generated payload and request
   types exist, on both sides.

   The path carries the `module-route` tag, which is what marks it a module data route rather than an
   infrastructure one. Nothing in the build reads the tag: it is there for a reader, and for the
   verification item that compares the schema's module data routes against the modules registered
   (TST032<!-- Pending: boundary schema is single and complete -->).

   **Every field a component declares required is a field something reads.** A required field with no
   consumer is carried across the boundary, asserted by the generated types on both sides, and read
   by nothing — which looks in every review exactly like a field that is used. If what would read it
   has not been written yet, the field is not required yet.

## An unavailable module and an unreachable backend are different states

A module has three states and no fourth, and it renders every one of them: it has not read yet, it
has read, or it read and has no reading to show. There is no absent payload — a component is never
handed nothing and never leaves its region blank — and **the first of the three is a real state
rather than a gap before the others**. A display is watched while it starts, and a region that is
blank until its first answer lands is indistinguishable from a region that is broken.

The third state is the module's own, and it is reached while the backend is reachable and its route
answered something other than a reading. Three things reach it: the route answered at a failing
status, and the structured body it answered with is what the module renders, distinct to its cause
(SRS001<!-- A failed module shows why, and only that module -->); the read did not come back at all,
within the deadline the page gives one; or the route answered at a status the boundary schema does
not describe, which carries no body to take a reason off. The last two have no cause of the module's
to render and are reported as an answer that did not arrive, rather than drawn as an empty box.

What that state is **not** reached by is the backend being gone. That is the next paragraph's, and
the distinction is the whole point of this section: a module that treated an outage as its own
failure would restate one outage once per region.

Where the backend itself is unreachable, no module reports anything. The page shell asks whether the
backend is still serving and reports an outage once for the whole display. Every module is handed
that answer and only an upstream-backed one acts on it: such a module, handed a false reachability
signal, stands down and renders nothing — no unavailable state, no placeholder, no last-known
content. What it holds is dropped with the signal, so it returns to not-having-read rather than
keeping a reading from before the outage. A module that kept one would draw weather from an hour ago
as now for the window between the backend coming back and the first read after it landing, and a
display that cannot say how old what it shows is must not do that. A module that reported for itself
here would restate the one outage once per region, which is what the display is obliged not to do
(SRS026<!-- The display says when the backend is gone -->).

Reachability reaches the component the way its configuration and payload do (part 1): as a prop,
threaded from the page shell through the frame to every module. The frame forwards it to every module
alike and makes no coverage decision from it, so nothing between the shell and the module decides
which modules an outage covers — that is each module's own question, not a placement one. The frame's
own use of the signal is spacing: it drops the top inset beneath a report that already holds that
edge.

A local module ignores the prop — it fetches nothing, so a backend that is gone takes nothing from it
and it keeps rendering beneath the page's report. An upstream-backed component that leaves the prop
undeclared draws its own unavailable state beneath the page's outage report, and nothing says so:
Svelte ignores a prop the component does not declare, and the render tier reads the stand-down
against a module it supplies rather than against this one.

A module still renders only these three states; the host's own `[data-module-faulted]` marker
(`ModuleHost`'s per-module `<svelte:boundary>`) is not a fourth. It stands in the module's place when
the module itself faults, is drawn by the host rather than reached by the module, and a fault raised
while the backend is reachable holds until the backend next comes back from an outage
(SRS069<!-- A module that stops drawing says so in its own place -->,
SRS070<!-- The display comes back on its own when the backend does -->).

## Dependency direction

Modules depend on the shared framework; the framework does not depend on a module. **The whole of
the exception is one file per side, and it is the registration those files hold.** On the backend it
is the route registration list of part 5, whose one field per module names that module's route type
and which therefore imports that module's package; on the frontend it is the roster that binds each
module's component, and its reading where it has one, to the name a configuration places into a
region. No other shared source names a specific module, and no other shared package imports a
module's package.

That the registration crosses at all is what a compile-time registration costs: a list of modules
that the compiler checks is a list that names them. The alternative — a module registering itself as
the process starts — was weighed and rejected where a rejected alternative belongs
([ADR 0008 rev 6](../decisions/0008-boundary-contract-openapi-codegen.md)). The crossing is bounded
to those two files, which is the property worth having, rather than removed.

That is the property that keeps a module removable: deleting its files and its registration entry
leaves nothing behind that referred to it. It is a statement about direction, not about the size of
a diff — framework code that a new module needs may be added, and it is shared code from the moment
it is written, so the next module inherits it. It is written for the one module that needs it, unless
a second consumer is known to be coming; generality built against a module that does not exist is
refused ([`CONTRIBUTING.md`](../../CONTRIBUTING.md)).

## Cadence and TTL are chosen together

The route's two cache TTLs (part 5) and the module's poll cadence are picked together, not
independently. The success cache bounds how far behind its source a served answer can be, and the
poll cadence bounds how long a fresh entry then waits before the display draws it — the two compose
into the module's freshness bound rather than each answering the same question alone. The negative
TTL is picked against a different question and comes out at a different figure: it is how long a
failure is held, which is how often a source that is down is asked again, and a source is least able
to bear load exactly when it is failing. **Neither TTL is the other's consequence, so neither is left
to fall out of the other** — each is read out of a requirement of its own
([§ Writing the module's requirements](#writing-the-modules-requirements)). All three are constants
in code, and none is an operator-tunable configuration key.

## Writing the module's requirements

A module's requirements are written before it is built: one `SYS` for the user-facing want,
decomposed by `SRS` items carrying what is specific to this module
([ADR 0012 rev 2](../decisions/0012-module-requirements-in-tree.md)). The build steps work against
what they produce. Earlier still is the UI design spec ([§ How a module is
tracked](#how-a-module-is-tracked)): a requirement can only formalize a want already made concrete on
the display, so what a module shows is decided in the design spec before it is obliged in the tree.
The order is design, then requirements, then build.

The need states what a viewer gets from this module, in one sentence carrying one `shall`, and it
enumerates nothing: a need listing its own decomposition is a hat over its children rather than a
want of its own. Its header is an indicative claim in noun-phrase form, the way the rest of the need
tier reads — SYS008<!-- The surface carrying no content is a mirror -->, not an imperative and not
the sentence beneath it said again. Write it for a reader with no stake in the code — that is what a
module need is for.

A module need obliges a **capability**, not an outcome: *the display shall be capable of showing …*
rather than *the display shall show …*. A framework need is unconditional because nothing can switch
it off, but nothing forces a module to be configured, so a need obliging the display to show that
module's content outright is unmet by every deployment that leaves the module out — deployments the
configuration permits. A need no conforming display can be built to satisfy is a mistake rather than
a want.

The decomposition beneath it carries what is true of this module and of nothing else — what it
renders in the region it is given and how it behaves there, and what it accepts as configuration;
and, where the module is upstream-backed, the source it fetches, the pattern its parameters must
match, the payload it puts across the boundary, its timing. It stops where the framework universals
begin ([ADR 0012 rev 2](../decisions/0012-module-requirements-in-tree.md) names them): a module
restating one has written it twice. The boundary is categorical rather than a roster of items to
check a draft against: the test on a draft `SRS` is whether rewording it to name a different module
would leave a sentence the framework already says.

**The test runs over rewording and over abstraction, and an author is done only when both have been
tried.** Rewording swaps the module named and keeps everything else — *the clock* for *the weather
module*. Abstraction goes one step further and drops what is particular to the module, leaving the
shape underneath: *the weather module shall report on the location its configuration names*
abstracts to *a module reports on the subject its configuration names*. Rewording alone is not
enough, because an item whose every clause mentions its own subject survives it untouched while the
sentence beneath it is a universal — which is precisely the item most worth catching. Abstraction
alone is not enough either: abstract far enough and every item reduces to *a module does what it is
for*, so the abstraction that counts is the one that drops the module and keeps the obligation.
Where the two disagree, the abstracted reading is the one to take to the second stage.

That test has a second stage, and skipping it is how an obligation goes missing. The test can leave
a sentence the framework *would* say but does not — true of every module, and written down for none
of them. Read as the first stage alone the test says drop it, and dropping it loses the obligation
with nothing recording that it went. So it is not the author's to drop: either it lands as a framework
item in the same change, or it is written as this module's, and a second module needing the same
sentence promotes it to a framework item rather than leaving a duplicate nobody notices. That
promotion is a different thing from
[ADR 0012 rev 2](../decisions/0012-module-requirements-in-tree.md)'s reopen premise, which revisits
the module-is-a-need decision itself once three near-identical decompositions reveal a universal that
was missed; this moves one obligation without reopening anything. Which of the two is an owner's call
rather than the author's, and the author's job is to surface that the sentence has no home yet.

**There is a third outcome, and it is neither drop nor escalate: the sentence is already settled by
this document.** The tree is not the only place an obligation on every module can live — this
contract states structure directly, and what it states is binding without a tree item behind it
([ADR 0011 rev 2](../decisions/0011-requirement-or-convention.md) decides which of the two an
obligation belongs in). *An upstream-backed module reads exactly one source* is part 5's "exactly
one entry in the static, compile-time list", and an author who searches the `SRS` tier for it finds
nothing and concludes the obligation is homeless. It is not; it is written down one document over.

So the second stage asks two questions rather than one. Does a framework item say it — if yes, drop
the draft. Does **this contract** say it — if yes, the draft cites the clause in its own
`rationale`, and nothing is promoted and nothing is escalated. Only when neither says it is the
sentence homeless, and only then is it the owner's call. Search both before raising one.

A module `SRS` names its module, and that is its correct form rather than an instance to be triaged
away. The instinct against naming one instance is aimed at the framework tiers, where an item stating
one module's behaviour sits among items obliging all of them and cannot be told from a universal by
position. Beneath a module need, position already says which module the item is about, and naming it
is what the item is for.

One overlap with the framework is legitimate, and it is an upstream-backed module's parameter
constraint. The framework obliges the validating and the rejecting
(SRS012<!-- Request parameters validated against known-good per-source constraints -->); which
constraint that is, that item's own rationale hands to the module, and it is stated as a module
`SRS`.

An upstream-backed module's timing is stated in its requirements, and each figure is carried together
with the rationale that produced it — the figure in the item, argued in the item, rather than a
constant somewhere with a comment beside it. **Before any of them is written, the author enumerates
every observable figure the module will ship**: both cache TTLs, the poll cadence, the upstream rate
bound, the rate its rate bucket admits and the burst that bucket holds, how far ahead anything it
forecasts reaches, how long it waits for its source to answer before abandoning the ask, how much of
that source's answer it reads before refusing the rest, how long the page gives one read before
abandoning it, and how much of a request body its route reads before refusing the rest. Each one
leaves that list in one of two ways — into a requirement that argues it, or into a written record
that it is a free choice and why nothing constrains it. **A figure that leaves the list by neither
route is an invented figure**, and it is invented whether or not it is a good number: what makes it
invention is that nothing in the specification could have been read differently to produce a
different one, so nothing can ever find it wrong.

**Two figures on that list are the framework's rather than any module's**, and an author confirms the
module fits inside them rather than arguing them: how long the page gives one read before abandoning
it, and how much of a request body a module data route reads before refusing the rest. Both are free
choices with a record rather than requirements — nothing in the specification settles either — and
both are chosen once for every module rather than once per module, so a module that restated either
would be arguing a figure it does not set.

**Four figures on that list are the module's own and still leave it by the free-choice route**, and
this section is their record. A comment at the constant says what the constant does and cites this
section; it is never itself the record, because a reason living only beside the code is unfindable by
anyone reading the specification, which is where the next author looks to see whether a figure was a
decision or an accident.

**The rate its bucket admits and the burst that bucket holds.** That there is a bound at all, and
that no operator tunes it, is obliged
(SRS011<!-- Upstream request rate is bounded, and the bound is not operator-tunable -->); what the
two figures are is not. The bucket is a coarse backstop over the source as a whole and cannot express
a bound the way the requirements state one — its rate is per minute and it is not keyed on what a
request is about — so nothing in the specification could have been read to produce a figure for
either. The enumeration's upstream rate bound is met by the success cache holding an answer, not
here. Both are set loose enough not to refuse the legitimate polling the cache already answers, and
both are there to catch a runaway rather than to be the bound. Neither value is derived.

**The outbound timeout and the ceiling on an upstream answer** are obliged to exist and not to be
anything in particular
(SRS014<!-- No single upstream exchange can stall or exhaust the backend -->): any finite pair of
figures discharges that item, so the figures themselves are chosen rather than read out of it. The
timeout is set well inside the deadline the page holds one read to, so a source that has gone quiet
is abandoned and reported rather than waited on past the point the display has stopped listening.
The ceiling is set several times the captured response the shaping is exercised against, so an
answer that runs away is refused rather than read, and no larger than that because it is a
multiplicand in what the route can come to hold
([`ARCHITECTURE.md`](../ARCHITECTURE.md) § Backend).

Three of those figures are argued **at the capability** rather than from a supplier's published
behaviour. Freshness states how stale the data a viewer sees may be, argued from how often the thing
the module reports on actually changes and from what a display glanced at rather than consulted
needs. Upstream rate states a politeness bound while the source is answering — how often this module
may ask — chosen low enough that a display left running for years is not throttled or cut off by any
free upstream for asking too often. **And the failure path states its own bound**, because the
argument behind the answering one does not survive a source that has not answered: asking oftener
than freshness requires buys a viewer nothing only while there is a fresh answer to hold, and while
there is none what a retry buys is the difference between a source that recovered and a display that
has not noticed. Left to fall out of the success figure, the failure path is either forbidden a retry
worth making or permitted a rate nobody argued for.

Arguing them that way is what makes them **capability requirements that double as
provider-suitability criteria**. A figure read off one service's refresh interval or its published
rate limit is that service's property wearing a requirement's clothes: it names no party and still
cannot outlive one, and swapping the supplier falsifies it silently. A figure argued from the
capability survives the swap and becomes the test a candidate supplier is held to — a source that
moves slower than the freshness figure, or that will not be asked as often as the rate figure
allows, is a source this module cannot use.

The route's two cache TTLs and the module's poll cadence are read out of those items rather than
picked at the keyboard. Its rate limit is not: what the entry's rate figure is, is a free choice with
the record above, and what a module does not restate is that the rate is bounded at all and not left
for an operator to tune
(SRS011<!-- Upstream request rate is bounded, and the bound is not operator-tunable -->) — the
framework obliges that there be a bound, and the module says what the figure is.

Four of [`TESTING.md`](../TESTING.md)'s tiers bear on a module's decomposition, and one of them
bears on every module's. The Render tier reads the component of part 1, which is what that document
states the tier by, so the items saying what this module renders in the region it is given are
written precisely enough for that tier to assert them. The other three are an upstream-backed
module's. The Unit tier reads the shaping library of part 4 — the upstream URL it builds and the
payload it reshapes a response into, pure and without network — so the items stating this module's
parameters and its payload are written precisely enough for that tier to assert them. The Integration
tier reads the route registration of part 5 — what leaves for the upstream and how often — so the
item stating how often this module may ask its source is written precisely enough for that tier to
assert it. The Contract tier is
machinery rather than a module obligation and earns no `SRS`
([`TESTING.md` § Where the Contract tier runs, and how it reaches upstream](../TESTING.md#where-the-contract-tier-runs-and-how-it-reaches-upstream)).

The module's `TST` items are written with the rest of the decomposition, as pending stubs:
`active: false`, header and text both prefixed `Pending:`, stating what will be asserted and what it
lands with — the shape the framework's own pending items already have
(TST002<!-- Pending: module error-state render test -->). Every module's render test is among them,
and an upstream-backed module's unit tests as well. A verification item stays inactive until the
code it checks exists, and a module's directories and test files do not exist until the first
vertical slice creates them; the items are activated, given a references entry and re-read against
their parent then.

## Drawing the module in the architecture model

The architecture model is drawn in the same change that accepts the module's `SYS` and `SRS` items —
they are written active, so the `status` flip is what the model waits on — because every accepted,
active `SYS` or `SRS` item binds to something the model draws
([ADR 0019 rev 7](../decisions/0019-boundary-at-what-deploys-and-tag-tier.md)). The `TST` stubs are
outside that rule, and activating one later owes the model nothing. While the items sit `proposed`,
no model work is owed.

Every module gains a component under the frontend for its Svelte component; an upstream-backed
module gains one under the backend for its shaping library, one external system for the upstream it
reads, and the relationship from the upstream client to that system — the edge that carries the
source-reachability obligation as the module lands
([ADR 0019 rev 7](../decisions/0019-boundary-at-what-deploys-and-tag-tier.md) § Where a tag sits),
and without which the drawn system is a box nothing reaches.

**That external system is named for the supplier, and the requirements above it are not.** The two
say different kinds of thing about the same module and the difference is expected rather than a
seam left open: the model draws what deploys and what it exchanges with, so the box is the service
actually reached and drawing it unnamed would leave the level asserting an upstream exists while
refusing to say which — the aggregate placeholder ADR 0019 rev 7 refuses. The specification says
what a viewer is owed, which the supplier is not ([§ A module is its capability, not its
supplier](#a-module-is-its-capability-not-its-supplier)). A supplier swap therefore moves the model
element and the route key and touches no item in the tree, which is the property the split exists
for.

How a tag is declared and applied is
[`architecture/README.md § What the model holds, and when an element earns a place`](../architecture/README.md#what-the-model-holds-and-when-an-element-earns-a-place)'s;
regenerating the model, its generated artifacts and `ARCHITECTURE.md` and committing them together
is [`architecture/README.md § Editing the model`](../architecture/README.md#editing-the-model)'s.
The prose in `ARCHITECTURE.md` that the drawing falsifies is swept in the same change.
`just check-arch-trace` is what closes the change, and it reads both directions — an accepted,
active item nothing carries fails it, and a tag naming an item still `proposed` fails it the other
way ([`CI.md § Documentation integrity`](../CI.md#documentation-integrity)) — which is why the model
lands with the status flip rather than before or after it.

## The module's UI design spec

Before the module's component is built, its on-screen composition is written down: a UI design spec,
colocated as `frontend/src/modules/<module>/README.md`. It states how the module's content is
composed — which type step each element takes, the alignment, the grouping devices, and the look of
each state — carries a reference render of that composition, and cites the [styling
contract](display-styling-contract.md)'s tokens it uses. It cites the requirements it realises too;
but the design spec is authored first ([§ How a module is tracked](#how-a-module-is-tracked)), so
those citations are added when phase 2's decomposition lands rather than at phase 1. Composition is not a
requirement and does not enter the tree
([the display design study](../design/display-design-study.md), *What belongs in the specification*);
this spec is where it is written down instead. The `clock` and `weather` modules' `README.md` are the
worked instances.

## Building the module

The steps run in the order each one's inputs are produced: an upstream-backed module's component is
written against the type its boundary-schema fragment generates. The component realises the module's
UI design spec above.

1. *(upstream-backed only)* Write the shaping library as pure functions, with its unit tests against
   a captured upstream response.
2. *(every module)* Add the module's section to the configuration schema and check an example
   configuration by loading it in the page.
3. *(upstream-backed only)* Write the registration entry in the module's own package, carrying all
   five of that route's policies — success TTL, negative TTL, rate limit, outbound timeout, maximum
   response size. The two cache TTLs carry the values the module's requirements settled; the rate
   limit, the outbound timeout and the response size are free choices, and this step is where each
   one's record is owed
   ([§ Writing the module's requirements](#writing-the-modules-requirements)). Write the route's
   schema handler beside it, reading the request through the generated request type and handing it to
   the route the entry builds; then add the one field naming that route type to the shared
   registration list.
4. *(upstream-backed only)* Add the module's payload and the request it answers to the boundary
   schema as named components, and tag the path `module-route`; the generated types both sides
   consume are emitted from them. Nothing in the build reads the tag, so it is not a step a red build
   will remind you of — it is what says which kind of path this is
   ([§ The six parts](#the-six-parts), part 6).
5. *(every module)* Write the component, plus its render test. Where the module has a payload, write
   the component against the generated type rather than hand-declaring it. Where the module is
   upstream-backed, declare the `reachable` prop and honour the stand-down it signals
   ([§ An unavailable module and an unreachable backend are different states](#an-unavailable-module-and-an-unreachable-backend-are-different-states)).
6. *(upstream-backed only)* Set the module's poll cadence to what its freshness obligation comes to,
   and check it against that route's TTL
   ([§ Cadence and TTL are chosen together](#cadence-and-ttl-are-chosen-together)).
7. *(every module)* Confirm the dependency direction still runs modules → framework, and that no
   shared framework source names the new module beyond its registration entry
   ([§ Dependency direction](#dependency-direction)).
8. *(every module)* Adding a module is a test-architecture review trigger — run it, per
   [`TESTING.md` § Review cadence](../TESTING.md#review-cadence).
9. *(every module)* Reconcile the documents the module has just falsified, in this change rather than
   after it: this contract where the module's shape is not the shape described, the architecture
   model and `ARCHITECTURE.md` where the drawing or the prose no longer matches
   ([§ Drawing the module in the architecture model](#drawing-the-module-in-the-architecture-model)),
   and the decision record where a decision was taken here rather than recorded there. Which of those
   a given fact belongs in is [ADR 0011 rev 2](../decisions/0011-requirement-or-convention.md)'s to
   say, and a reason living in a code comment because no document was found for it is a reason that
   has not been recorded. **A module is not done while a document describes something else.**

## Adding an obligation to this contract

What this page states is binding without a tree item behind it, which makes an obligation written
here as costly to get wrong as one written in the tree and harder to notice: a requirement that no
code satisfies fails a check, and a sentence here that no code satisfies simply sits.

So an obligation added here is checked against a module that conforms, before it lands. Walk the
build steps against the module in the tree that most nearly matches the new sentence and confirm each
one can be carried out and confirmed — not that it ought to be, that it can. A step that the module
in the repository cannot pass is not a standard the tree has yet to meet; it is a defect in this
page, and it will be read as licence by the next author who finds the code and the contract
disagreeing and picks the code.

The same walk is what an obligation's **removal** owes: a clause deleted here may be the only place a
real obligation was written down, this page being one of the two homes
([ADR 0011 rev 2](../decisions/0011-requirement-or-convention.md) decides which), so a deletion says
where the obligation went rather than that it is gone.

## A shape this contract does not fit

A module fed by a push or real-time transport — a socket the backend writes to rather than a route
the frontend polls — needs a connection manager, a lifecycle and reconnect handling. That is shared
framework code, and it has no place in parts 1–6 as written.

Such a module is accommodated by amending this contract to describe its shape, not by forcing it
into the pull-based one. The same event is a trigger for reviewing the test architecture
([`TESTING.md` § Review cadence](../TESTING.md#review-cadence)); whoever acts on one reads both.
