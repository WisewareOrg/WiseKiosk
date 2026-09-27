# Park wait-times — UI design spec

How the theme-park wait-times module composes its content on the display. This is the visual half of
the module's specification — the composition the module's component (built in phase 3) is built to and
reviewed against. It states composition — where each element sits, at what step, and how the groups
are set apart — and cites the obligation each choice realises rather than restating it.

![Reference render of the wait-times module: a grid of per-park cards, each a park icon and name over its hours, a three-ride leaderboard, a rotating "more waits" pair, and a segmented footer counter, drawn full-white on the black display. One park is shown closed for the evening.](./park_wait_times-composition.png)

*Rendered with the bundled Inter face and this module's own park icons. The grid shape and region
proportion are illustrative — the column/row count and where the module is placed are the
configuration's, not this spec's. Sample data is Walt Disney World and Universal Orlando in the
evening.*

It is **not** a requirement. Composition is deliberately outside the requirements tree (the
[display design study](../../../../docs/design/display-design-study.md), *What belongs in the
specification*); what the tree owns is the behaviour, authored in phase 2 — the requirements
sub-issue of the #298 wait-times epic — which the *Realises* column in *What each choice realises*
below traces each composition choice to. Where this spec and the
[display styling contract](../../../../docs/contracts/display-styling-contract.md) meet, the contract
owns the shared design language (the type and spacing scales, the emission rule, the card and
grouping vocabulary) and this spec owns only how this module reaches for it.

## The reading — a grid of park cards

The module shows several parks at once, each as one **card**: a self-contained board a viewer reads
without reference to its neighbours. A park is never off-screen waiting its turn — every configured
park holds its place, and the cycling happens *inside* each card rather than between parks.

A card is the display's bordered, rounded treatment for one self-contained reading — the same idiom
the weather module uses for its stat and day cards, drawn in `--emission-stroke` at
`--divider-stroke-width × 2` with a `--space-sm` radius. Several independently-labelled park lists
sharing one region is the exact case the styling contract's grouping vocabulary is built for (its
decision rule, *three parks' wait lists sharing one region*); the card outline earns its place there.
The separation is carried first by the grid's spacing, so a bright reflection erasing the outline
does not collapse one park into the next.

## The card, top to bottom

Each card reads as three stacked zones under a header, every zone a full-white content reading:

### Header — icon, park, hours

- a **park icon** and the **park name** on a shared centre line, the icon left of the name — the icon
  gives the park an identity a viewer recognises before reading the word (see *The park icon set*);
- the park's **operating hours** for the day, right-aligned on the same line. Where the hours cannot
  be read, the line is simply absent — no placeholder, and the rest of the card is unaffected
  (SRS067<!-- The park-wait-times module confines a failure to the part of its own response the failure touches -->).

The park name is the card's title and the most prominent element on it; the hours are its quiet peer.
The header sits above a dim divider — the label-over-rule idiom the whole display shares (the clock's
peer rule, the weather module's *Next hours* / *Next days* headers, and this header are the same
line).

### Leaderboard — the three longest waits, held

The three longest current **numeric** waits in the park, persistent: each a ride **name** on the left
and its **wait** on the right. A ride that is not operating carries no minute figure to rank by, so it
does not hold a leaderboard place — it still reads, in the rotation below.

### More waits — the rotation

Beneath a plain divider (the label dropped to save vertical space; owner ruling, #309), the rest of
the park's rides tour through **two at a
time**, advancing on a configured interval (the same on-an-interval idiom as the weather module's
series toggle). Every card advances on **one clock the placement owns**, so the cards turn their
pages together on the same tick rather than each running its own timer from its own mount moment and
drifting out of step with the others.

A deployment may set that interval to anything the schema admits, and **production leaves it at the
schema's default** (owner, 2026-09-23). That is the interval every timing below is read against — the
marquee's reach in particular — so a deployment that overrides it is reading this composition at a
pace it was not drawn for.

### Footer — the rotation counter

A single segmented bar across the foot of the card: one segment per rotation page, filled to the
current position. It tells a viewer how far through the park's rides the rotation has come and that
more are coming, without a floating icon or a number that would read as another datum. Its placement
(pinned to the card foot) and its being one deliberate element rather than a scatter of ticks are
what make it read as chrome, not content.

## The wait slot — a number, or a state

The wait on every row — leaderboard or rotation — answers one question: *how long until you can ride
this?* Its answer is either a **wait in minutes** or a **not-operating state** — `Down` (a temporary
stoppage), `Closed` (outside the ride's hours, or the whole park's), or `Refurb` (a longer closure). A
state is not a different kind of thing from a number here: both answer the same question, so they
share the one slot rather than each claiming a place of their own. A number reads as a figure; a
state reads as an uppercase-tracked word, so the two are never mistaken for one another while
sharing the column. A park
with no ride reporting a length of time is not read row by row at all — see *Park closed* under
*States*.

## The grid — constant card, configured shape

The card is **identical at every park count** and a **fixed width**: the widest a park's own header
(icon, name, hours) draws across the configured roster, and no wider — a ride name does not set it,
however long. The grid only arranges the cards; the number of **columns and rows is configuration**
(`nCol` × `nRows`) — six parks as 3 × 2, or 2 × 3, or 6 × 1 — so an operator lays the module out for
the region it is placed in.

A park name is drawn on a single line and never wraps. A ride name is too, but where the fixed card
leaves it no room, it scrolls to reveal itself rather than wrapping, truncating, or widening the card
— held at home, scrolled left to the end, held there, returned home; a name that already fits is left
static. The hold at home also keeps the scroll clear of the card flip's repaint, which otherwise reads
as a stutter.

What moves is the **name column's own `scrollLeft`**, not a transform on the text inside it. On a Pi
Zero-class host, painting a clipped container's scroll offset costs a fraction of translating the
text with every overflowing row in motion
(SRS021<!-- Frontend runs on a Pi Zero-class browser host -->, measured in
meta-wisekiosk #100 gpu-compositing), and no row is promoted to a compositor layer of its own. One
`requestAnimationFrame` loop across the whole placement drives every scrolling column rather than one
loop per row, so the motion costs a single callback whatever the roster size.

The scroll runs **once per rotation tick**, not on a loop of its own: the tick that turns the cards'
pages is the tick that returns every column home and starts it moving again, so a page turn and a
name restarting are one event on one clock rather than two cadences drifting apart. A name needing
more travel than its reach — `(rotation_interval_seconds − HOLD_HOME_S) × MARQUEE_PX_PER_S`, the
distance travelled before the rotation tick (`marquee-clock.ts`) — is cut short by that reset rather
than scrolling faster to fit. At the schema's eight-second floor the reach covers the longest
configured-park ride name measured at the deployed viewport, but a long enough name is read in part
rather than in full.

## Type and spacing

Every element takes a named step from the
[styling contract](../../../../docs/contracts/display-styling-contract.md)'s type scale; none is a
one-off size. Steps are provisional against the deployed panel and confirmed in situ, not on a
monitor (the contract's calibrated bounds).

| Element | Step |
|---|---|
| park name | `body` (uppercase, tracked) |
| park hours | `section-header` |
| leaderboard ride name, leaderboard wait | `section-header` |
| rotation ride name, rotation wait | `section-header` |
| a `Down` / `Closed` / `Refurb` state word | `caption` (uppercase, tracked) |
| closed-park icon | `annotation` |
| closed-park `Closed` label | `body` (uppercase, tracked) |

The park name is the card's one prominent step so the identity leads; the wait figure sits at the
**same step as its ride name**, weighted (bold) and `tabular-nums` rather than enlarged, so the number
scans without dominating the name. All figures are `tabular-nums`, so a wait changing under the
display never shifts the layout and the right-hand column stays aligned down a card. Legibility is not
traded for width here: the marquee is what a long ride name gives up its width for
(owner ruling, #309), not the type scale — the header is what fixes the card's own width instead.

Within a card, the header is set from the leaderboard by `md`; leaderboard rows and rotation rows from
each other by `sm`; the More Waits divider from its rows by `md`; the footer bar from the rotation by
`md`. Cards are set apart in the grid by `lg`.

## Grouping, and coherence with the rest of the display

Hierarchy is carried by **size, weight, and position only — never by dimming or colour** (the
contract's emission rule). Every reading — park name, hours, ride names, waits — is drawn at
`--emission-content`; the only dim marks are the header and More Waits dividers and the card outline,
all `--emission-stroke` below the emission ceiling, and the footer bar's unfilled segments. This is
the display's coherence device: the same uppercase-tracked label idiom (the weather module's group
labels, the clock's weekday), the same dim-stroke divider weight everywhere, the same type scale, and
`tabular-nums` throughout.

## States

The module is upstream-backed, so it draws each state its payload can be in, in the viewer's language,
never a blank region (the styling contract; the
[module contract](../../../../docs/contracts/module-contract.md)):

- **Park open, ride operating** — the composition above.
- **Ride not operating** — `Down` or `Closed` in the wait slot, drawn as data, no special card.
- **Ride in refurbishment** — `Refurb` in the wait slot, a third state distinct from `Down` and `Closed`, drawn as data, no special card.
- **Park closed** — no ride reports a length of time, drawn from the ride data alone, never the
  park's hours; the card's icon and `Closed` replace its leaderboard and tour, in the same box, so
  the grid stays aligned.
- **Park unavailable** — that park's own reading could not be produced while the rest of the grid
  answered; its card holds its place, showing a plain-language reason in place of its rides, and
  every other park reads normally.
- **Loading** — a plain line (*Reading wait times…*) at the `body` step. A module asked for but
  unanswered is neither a reading nor a failure.
- **Unavailable** — the failure's own plain-language message at the `body` step, in the module's own
  place, affecting no other module.
- **Backend unreachable** — the module stands down and renders nothing; the page reports the one
  outage for the whole display (the module contract, *an unavailable module and an unreachable backend
  are different states*). This is not the module's own state to draw.

## The park icon set

Each park carries a small monochrome line-glyph, drawn in the display's stroke idiom (full-white, a
thin even stroke, a 24-unit grid), colocated as SVG under [`./icons`](./icons): `castle` (Magic
Kingdom), `globe` (Epcot), `sorcerer-hat` (Hollywood Studios), `tree` (Animal Kingdom), `clapperboard`
(Universal Studios), `coaster` (Islands of Adventure). They are **original, stylised archetypes** — a
spired castle, a faceted sphere, a spreading tree — not reproductions of any park's trademarked logo
or landmark; the actual branded marks are deliberately not used.

Two things this spec leaves to the implementation phase, because they are behaviour and asset scope
rather than composition: the **`park → icon` mapping** (a configured park with no matching glyph draws
**no icon**, name only — the fallback), and whether the set is carried as these files or inlined by the
component.

## What each choice realises

The composition choices this spec makes, each traced to the obligation the requirements tree writes
for it (the #298 epic's requirements sub-issue). Where a choice realises a framework obligation rather
than one of this module's own — full-emission content, the emission ceiling, the type-size floor — the
*Realises* column cites the framework item, since composition reaches for those the same way every
module does.

| Composition choice | Realises |
|---|---|
| a card per park, every configured park held on screen at once | SRS057<!-- The park-wait-times module holds every configured park on screen at once --> |
| a persistent three-ride leaderboard of the longest current waits | SRS058<!-- The park-wait-times module keeps each park's longest current waits in view --> |
| the remaining rides rotate two at a time, on a configured interval | SRS059<!-- The park-wait-times module tours the remaining rides on an interval its configuration sets --> |
| a ride name too wide for its column scrolls to reveal itself rather than wrapping or truncating | SRS071<!-- The park-wait-times module brings the hidden part of a ride name into view --> |
| every card's rotation advances on one clock the placement owns | SRS072<!-- The park-wait-times module advances every card's tour together --> |
| the wait slot carries a wait in minutes, or a `Down` / `Closed` / `Refurb` state | SRS056<!-- The park-wait-times module puts each park's identity, hours, and ride waits across the boundary --> / SRS061<!-- The park-wait-times module draws a wait as the time or the not-operating state it is handed --> |
| the grid shape (`nCol` × `nRows`) is configuration | SRS060<!-- The park-wait-times module arranges its parks in a grid its configuration shapes --> |
| the hours line is absent, with no placeholder, when a park's hours cannot be read | SRS067<!-- The park-wait-times module confines a failure to the part of its own response the failure touches --> |
| full-white content, dim strokes only, hierarchy by size and weight | SRS032<!-- Readable text is carried at full emission --> / SRS030<!-- Only content is rendered above the emission ceiling --> |
| every type step at or above the type-size floor | SRS033<!-- Text holds a minimum size against the display, at every resolution --> |

## To confirm in situ

Confirmed against a photograph of the deployed display, not a monitor (the design study):

- the **footer bar** and the **icon strokes** are thin marks, and the study is explicit that thin
  strokes are the first thing a bright reflection dissolves — both are the parts of this composition
  to check against the deployed panel before they are considered final;
- whether the footer bar's **filled** segments read best at `--emission-content` or a step below the
  ceiling, since the bar is chrome rather than a datum;
- the type steps against the type-size floor at the deployed viewing distance, once the grid's
  `nCol` × `nRows` for the placed region is set (a denser grid draws smaller cards);
- the marquee's scroll speed and hold timing read comfortably at the deployed viewing distance, and
  whether a name long enough to be cut short by the rotation interval is a real loss at the deployed
  geometry;
- whether the eight-second floor's reach still covers the longest ride name after a roster or grid
  change — the floor was measured against one configured roster at one card width, and nothing
  re-checks it.
