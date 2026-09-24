import {
  ParkWaitTimesState,
  type ParkWaitTimesPark,
  type ParkWaitTimesPayload,
  type ParkWaitTimesRide,
} from '../../lib/boundary/client';
import { LIVENESS_INTERVAL_MS } from '../../lib/liveness';
import {
  advanceHostClock,
  asksBeyondTheShell,
  channelsBeyondTheTier,
  expect,
  holdHostClock,
  render,
  serveLiveness,
  serveModuleData,
  test,
  watchTraffic,
  type Fixture,
} from '../../../tests/render/harness';
import { HOLD_END_S, HOLD_HOME_S, MARQUEE_PX_PER_S } from './marquee-clock';

/**
 * The park-wait-times module's render tests. Every one answers the module's route from the test,
 * never a real source.
 */

const READ_INTERVAL_MS = 5 * 60 * 1000;

/** The last stretch of that interval, held back so a read can be shown to fall inside it. */
const ALMOST = 10_000;

/** An instant to hold the host clock at, wherever a case drives time rather than waiting it out. */
const HOST_TIME = new Date('2026-08-31T14:00:00Z');

/** Enough driven time under a held clock for a marquee measurement's own animation frame to run.
    `page.clock` fakes requestAnimationFrame along with the timers it drives, so the measurement
    `ParkCard.svelte`'s `marquee` action schedules at mount stays queued until the clock is run. */
const MEASURE_FRAME_MS = 100;

/** The most a scroll reading may lag the instant it is taken at: the fake clock paces animation
    frames at about 16ms and `scrollLeft` reports whole pixels, together under a pixel and a half of
    travel at `MARQUEE_PX_PER_S`. */
const FRAME_SLACK_PX = 2;

/** One park's answer, defaulting to available with no rides — every case fills in what it reads.
    Takes the park's own name, not an id: the response carries no id field. */
function onePark(name: string, fields: Partial<ParkWaitTimesPark> = {}): ParkWaitTimesPark {
  return { name, available: true, ...fields };
}

/** A full payload, one entry per park named. */
function parksPayload(parks: ParkWaitTimesPark[]): ParkWaitTimesPayload {
  return { parks };
}

/** A ride fixture: a number sets an Operating wait in minutes; a state word sets that state with
    waitMinutes null. */
function ride(name: string, wait: number | ParkWaitTimesState): ParkWaitTimesRide {
  return typeof wait === 'number'
    ? { name, state: ParkWaitTimesState.Operating, waitMinutes: wait }
    : { name, state: wait, waitMinutes: null };
}

/** A placement naming `parks`, laid out `columns` × `rows`. */
function placed(
  parks: string[],
  { columns = parks.length, rows = 1, rotationIntervalSeconds }: { columns?: number; rows?: number; rotationIntervalSeconds?: number } = {},
  region = 'middle_center',
): Fixture {
  return {
    modules: [
      {
        region,
        module: 'park_wait_times',
        options: {
          parks,
          columns,
          rows,
          ...(rotationIntervalSeconds === undefined ? {} : { rotation_interval_seconds: rotationIntervalSeconds }),
        },
      },
    ],
  };
}

const MODULE = '[data-park-wait-times]';
const CARD = '[data-pwt-card]';
const LOADING = '[data-module-loading]';
const MODULE_UNAVAILABLE = '[data-module-unavailable]';
/**
 * What the page draws where a module threw, in place of that module
 * (../../../tests/render/fault-containment.spec.ts). Distinct from `MODULE_UNAVAILABLE`, which is this
 * module's own report that its reading failed: a contained fault is read here because an uncaught
 * error is no longer visible as one — it is caught, and the marker is the only thing left that says it
 * happened.
 */
const MODULE_FAULTED = '[data-module-faulted]';
const PARK_UNAVAILABLE = '[data-pwt-unavailable]';
const HEADER = '[data-pwt-header]';
const LEADERBOARD = '[data-pwt-leaderboard]';
const LEADERBOARD_ROW = '[data-pwt-leaderboard-row]';
const MORE_WAITS = '[data-pwt-more-waits]';
const TOUR_ROW = '[data-pwt-tour-row]';
const FOOTER = '[data-pwt-footer]';
const FOOTER_SEGMENT = '[data-pwt-footer-segment]';
const WAIT = '[data-pwt-wait]';
/** The ride-name column — the clipping scroll container the marquee clock writes `scrollLeft` on,
    not the oversized `.ride-name-text` inside it. */
const RIDE_NAME_COLUMN = '[data-pwt-ride-name]';

/** The six known parks by their own pretty name — the one identity the wire carries
    (boundary/openapi.yaml's ParkWaitTimesPark.name). A render fixture names
    parks by these strings for both the request (`parks`) and the response `onePark` builds. */
const MAGIC_KINGDOM = 'Magic Kingdom';
const EPCOT = 'Epcot';
const HOLLYWOOD_STUDIOS = 'Hollywood Studios';
const ANIMAL_KINGDOM = 'Animal Kingdom';
const UNIVERSAL_STUDIOS = 'Universal Studios';
const ISLANDS_OF_ADVENTURE = 'Islands of Adventure';

/** A name the module has no icon for — an unrecognized park, drawn name-only (the sanctioned
    fallback). */
const UNKNOWN_PARK = 'A Park With No Icon';

/** A ride name far wider than any ride-name column this module draws, at every viewport the render
    tier runs — so a case about an overflowing name is about that and not about which viewport it
    happened to run at. */
const OVERFLOWING_RIDE_NAME =
  'Guardians of the Galaxy: Cosmic Rewind — The Complete Extended Experience Edition';

/** Locates a rendered card by the park's own displayed name rather than the `data-pwt-park`
    test-id. */
function cardNamed(page: import('@playwright/test').Page, name: string) {
  return page.locator(CARD).filter({ has: page.locator('.name', { hasText: name }) });
}

/** A card's own leaderboard-row and tour-row ride names, read in the order drawn. */
async function rideNamesIn(card: ReturnType<import('@playwright/test').Page['locator']>, selector: string): Promise<string[]> {
  return card.locator(selector).locator('.ride-name').allTextContents();
}

test('TST071: reports on the parks its configuration names, in the region it names, and moves with a second configuration', async ({
  page,
}) => {
  // One stub, two placements: the answer is a function of the ask, so the two rosters are told apart
  // by what each request carried rather than by the order the module happened to ask in.
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured))) };
  });

  await render(page, {
    modules: [
      { region: 'middle_center', module: 'park_wait_times', options: { parks: [MAGIC_KINGDOM], columns: 1, rows: 1 } },
      { region: 'lower_third', module: 'park_wait_times', options: { parks: [EPCOT], columns: 1, rows: 1 } },
    ],
  });

  // Each region shows the answer given for the park its own placement named.
  const here = page.locator(`[data-region="middle_center"] ${CARD}`);
  const there = page.locator(`[data-region="lower_third"] ${CARD}`);
  await expect(here.locator('.name')).toHaveText(MAGIC_KINGDOM);
  await expect(there.locator('.name')).toHaveText(EPCOT);

  // A second configuration moves both.
  await render(page, {
    modules: [
      { region: 'middle_center', module: 'park_wait_times', options: { parks: [HOLLYWOOD_STUDIOS], columns: 1, rows: 1 } },
      { region: 'lower_third', module: 'park_wait_times', options: { parks: [ANIMAL_KINGDOM], columns: 1, rows: 1 } },
    ],
  });
  await expect(here.locator('.name')).toHaveText(HOLLYWOOD_STUDIOS);
  await expect(there.locator('.name')).toHaveText(ANIMAL_KINGDOM);
});

test('TST074: draws every configured park at once, none absent awaiting a rotation between parks', async ({
  page,
}) => {
  const roster = [MAGIC_KINGDOM, EPCOT, HOLLYWOOD_STUDIOS, ANIMAL_KINGDOM];
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured))) };
  });
  await render(page, placed(roster, { columns: 2, rows: 2 }));

  // All four, read on the first paint — nothing here waits for a clock to advance, because a park
  // rotating onto screen later would still pass a count taken after one.
  await expect(page.locator(CARD)).toHaveCount(roster.length);
  const shown = await page.locator(CARD).evaluateAll((cards) => cards.map((card) => card.querySelector('.name')?.textContent ?? ''));
  expect(new Set(shown)).toEqual(new Set(roster));
});

test('renders two configured parks that resolve to the same name without throwing — the grid’s own each block is keyed positionally, never on identity', async ({
  page,
}) => {
  // Two parks sharing the one identity the wire carries, the name — the case
  // a name- or id-keyed `{#each}` throws Svelte's own each_key_duplicate on.
  const pageErrors: string[] = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));

  // One ride each, and different ones: the two cards then differ by content while sharing the name
  // they are keyed against, so a grid that collapsed them into one is caught by what is drawn rather
  // than only by how many boxes there are.
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(MAGIC_KINGDOM, { rides: [ride('Space Mountain', 45)] }),
      onePark(MAGIC_KINGDOM, { rides: [ride('Big Thunder Mountain', 20)] }),
    ]),
  }));
  await render(page, placed([MAGIC_KINGDOM, MAGIC_KINGDOM], { columns: 2, rows: 1 }));

  // What the module drew, not what the page failed to raise. A throw here no longer reaches the page
  // as an uncaught error — it is caught where the module is mounted and the module is replaced by the
  // marker (../../../tests/render/fault-containment.spec.ts) — so the absence below would be satisfied
  // by the very failure it was written to catch. Both cards, each with its own name and its own ride,
  // is what a module that rendered the duplicate looks like and a contained one does not.
  await expect(page.locator(CARD)).toHaveCount(2);
  await expect(page.locator(`${CARD} ${HEADER}`)).toHaveCount(2);
  expect(
    await page
      .locator(CARD)
      .evaluateAll((cards) => cards.map((card) => card.querySelector('.name')?.textContent ?? '')),
  ).toEqual([MAGIC_KINGDOM, MAGIC_KINGDOM]);
  await expect(page.locator(`${CARD} ${RIDE_NAME_COLUMN}`)).toHaveText([
    'Space Mountain',
    'Big Thunder Mountain',
  ]);
  await expect(page.locator(MODULE_FAULTED)).toHaveCount(0);

  // Kept beside them rather than in place of them: a boundary catches what is thrown under a render,
  // and an error raised anywhere else still arrives here.
  expect(pageErrors, 'the page raised nothing while rendering the duplicate').toHaveLength(0);
});

test('renders a park whose rides share a name without throwing — the ride each block is keyed positionally too', async ({
  page,
}) => {
  // Two rides sharing a name, the payload carrying no ride id
  // (boundary/openapi.yaml's ParkWaitTimesRide) — the case a name-keyed
  // `{#each}` throws Svelte's own each_key_duplicate on.
  const pageErrors: string[] = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));

  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride('Test Track', 40), ride('Test Track', 15)] })]),
  }));
  await render(page, placed([EPCOT]));

  // Both rows drawn, each carrying its own name and its own wait, for the reason the duplicate-park
  // case above states: a throw is contained rather than raised at the page, so what the module drew is
  // the reading and the absence of an uncaught error no longer is.
  await expect(page.locator(LEADERBOARD_ROW)).toHaveCount(2);
  await expect(page.locator(`${LEADERBOARD_ROW} ${RIDE_NAME_COLUMN}`)).toHaveText([
    'Test Track',
    'Test Track',
  ]);
  await expect(page.locator(`${LEADERBOARD_ROW} ${WAIT}`)).toHaveText(['40', '15']);
  await expect(page.locator(MODULE_FAULTED)).toHaveCount(0);

  expect(pageErrors, 'the page raised nothing while rendering the duplicate ride name').toHaveLength(0);
});

/** Seven rides spread widely enough that the ranking
    (SRS058<!-- The park-wait-times module keeps each park's longest current waits in view -->) and
    the rotation
    (SRS059<!-- The park-wait-times module tours the remaining rides on an interval its
    configuration sets -->) are each unambiguous: three waits longer than every other entry, and
    four more than the tour's own two-at-a-time page holds, so a full cycle takes more than one
    page. */
function rankedRoster(): ParkWaitTimesRide[] {
  return [
    ride('Test Track', 80),
    ride('Soarin', 50),
    ride('Spaceship Earth', 20),
    ride('Mission: Space', 10),
    ride('Imagination!', 5),
    ride('The Seas', 8),
    ride('Living with the Land', 3),
  ];
}

test('TST076: tours the remaining rides two at a time, on the configured interval, reaching every one of them', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT], { rotationIntervalSeconds: 8 }));

  const card = page.locator(CARD);
  // Held: Test Track, Soarin, Spaceship Earth. Remaining, in the source's own order: Mission: Space,
  // Imagination!, The Seas, Living with the Land — four rides, two pages at two per page.
  await expect(card.locator(TOUR_ROW)).toHaveCount(2);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  // Not yet — a step short of the configured interval finds the same pair still shown.
  await advanceHostClock(page, 8 * 1000 - 500);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  // The interval elapses, and the tour advances to the next pair — the whole of the remainder is
  // reachable, not only the pair the first paint happened to show.
  await advanceHostClock(page, 500);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['The Seas', 'Living with the Land']);

  // And a full cycle returns to the first pair, the tour being a repeating rotation rather than a
  // one-time advance.
  await advanceHostClock(page, 8 * 1000);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);
});

test('TST076: the rotation interval is the configuration’s, not one fixed in the module', async ({ page }) => {
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT], { rotationIntervalSeconds: 20 }));

  const card = page.locator(CARD);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  // A shorter span than the one configured here does not advance a module configured for twenty
  // seconds, proving the interval read is the configuration's rather than a value fixed in the
  // component — the same distinction TST069 draws for the weather module's series switch.
  await advanceHostClock(page, 6 * 1000);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  await advanceHostClock(page, 20 * 1000 - 6 * 1000);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['The Seas', 'Living with the Land']);
});

test('TST076: a placement that omits its own rotation interval tours on the schema’s default of eight seconds', async ({
  page,
}) => {
  // rotation_interval_seconds is genuinely absent from this placement's own options (unlike the
  // cases above, which all set it) — the schema's own default: 8 (config/schema.json) is what
  // ajv's useDefaults fills in before the component ever reads the config, and the component reads
  // that value directly rather than falling back to one of its own. This pins the eight-second
  // figure as the only source that fill comes from; it does not by itself prove every way that
  // fill could stop working.
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT]));

  const card = page.locator(CARD);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  // A step short of eight seconds finds the same pair still shown.
  await advanceHostClock(page, 8 * 1000 - 500);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  // The eight-second default elapses, and the tour advances.
  await advanceHostClock(page, 500);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['The Seas', 'Living with the Land']);
});

test('TST076: the footer marks the tour’s own position, one segment per page', async ({ page }) => {
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT], { rotationIntervalSeconds: 8 }));

  const segments = page.locator(CARD).locator(FOOTER_SEGMENT);
  await expect(segments).toHaveCount(2);
  await expect(segments.nth(0)).toHaveClass(/filled/);
  await expect(segments.nth(1)).not.toHaveClass(/filled/);

  await advanceHostClock(page, 8 * 1000);
  await expect(segments.nth(0)).not.toHaveClass(/filled/);
  await expect(segments.nth(1)).toHaveClass(/filled/);
});

/** A park's roster of `count` rides, waits descending so the ranking is unambiguous and every name
    distinct. `count` fixes the card's page count: three are held, the rest tour two at a time. */
function rosterOf(park: string, count: number): ParkWaitTimesRide[] {
  return Array.from({ length: count }, (_unused, index) => ride(`${park} ride ${index}`, count - index));
}

/** The index of the footer segment this card draws filled — its own position in its own tour
    (`class:filled`, ParkCard.svelte). */
async function filledSegment(card: ReturnType<import('@playwright/test').Page['locator']>): Promise<number> {
  return card
    .locator(FOOTER_SEGMENT)
    .evaluateAll((segments) => segments.findIndex((segment) => segment.classList.contains('filled')));
}

test('advances two cards of different page counts on the same tick — neither moves early, and the shorter wraps home as the longer takes its last page', async ({
  page,
}) => {
  // Different page counts are what make this a reading of the cards' agreement rather than of one
  // card twice: seven rides tour in two pages, nine in three, so the positions the pair holds are
  // `tick % 2` and `tick % 3`, and the shorter card wraps home on the very tick the longer one
  // takes to its last page. Each position is read a step BEFORE its tick as well as after, which is
  // what pins the two advances to the same tick rather than to two moments inside one interval —
  // a card a fraction of an interval out of phase fails that reading.
  const ROTATION_S = 8;
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, { rides: rosterOf(EPCOT, 7) }),
      onePark(MAGIC_KINGDOM, { rides: rosterOf(MAGIC_KINGDOM, 9) }),
    ]),
  }));
  await render(page, placed([EPCOT, MAGIC_KINGDOM], { rotationIntervalSeconds: ROTATION_S }));

  const shorter = cardNamed(page, EPCOT);
  const longer = cardNamed(page, MAGIC_KINGDOM);
  // Self-check the fixture: the two cards really do tour different numbers of pages, so a shared
  // position below is not two cards drawing the same tour.
  await expect(shorter.locator(FOOTER_SEGMENT), 'four remaining rides, two pages').toHaveCount(2);
  await expect(longer.locator(FOOTER_SEGMENT), 'six remaining rides, three pages').toHaveCount(3);
  expect([await filledSegment(shorter), await filledSegment(longer)]).toEqual([0, 0]);

  // A step short of the interval, neither has moved — which is what pins the two advances below to
  // the same tick rather than to two moments inside the same interval.
  await advanceHostClock(page, ROTATION_S * 1000 - 500);
  expect([await filledSegment(shorter), await filledSegment(longer)]).toEqual([0, 0]);
  await advanceHostClock(page, 500);
  expect(
    [await filledSegment(shorter), await filledSegment(longer)],
    'both cards advanced across the one tick',
  ).toEqual([1, 1]);

  // The tick the shorter card wraps on and the longer one does not: still one counter, read modulo
  // each card's own page count.
  await advanceHostClock(page, ROTATION_S * 1000 - 500);
  expect([await filledSegment(shorter), await filledSegment(longer)]).toEqual([1, 1]);
  await advanceHostClock(page, 500);
  expect(
    [await filledSegment(shorter), await filledSegment(longer)],
    'the shorter card wrapped home on the same tick the longer one took to its last page',
  ).toEqual([0, 2]);
});

test('TST077: lays its cards out in the column and row counts its configuration names, and a second shape re-lays them', async ({
  page,
}) => {
  const roster = [MAGIC_KINGDOM, EPCOT, HOLLYWOOD_STUDIOS];
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured))) };
  });

  await render(page, placed(roster, { columns: 3, rows: 1 }));
  const grid = page.locator('[data-pwt-grid]');
  const firstShape = await grid.evaluate((element) => ({
    columns: getComputedStyle(element).gridTemplateColumns.trim().split(/\s+/).length,
    rows: getComputedStyle(element).gridTemplateRows.trim().split(/\s+/).length,
  }));
  expect(firstShape).toEqual({ columns: 3, rows: 1 });

  // The same three parks, a second shape — the layout follows the configuration rather than the
  // roster it happens to have been built for.
  await render(page, placed(roster, { columns: 1, rows: 3 }));
  const secondShape = await grid.evaluate((element) => ({
    columns: getComputedStyle(element).gridTemplateColumns.trim().split(/\s+/).length,
    rows: getComputedStyle(element).gridTemplateRows.trim().split(/\s+/).length,
  }));
  expect(secondShape).toEqual({ columns: 1, rows: 3 });
  const shown = await page.locator(CARD).evaluateAll((cards) => cards.map((card) => card.querySelector('.name')?.textContent ?? ''));
  expect(new Set(shown), 're-laid rather than re-fetched, so the same roster still shows').toEqual(new Set(roster));
});

test('TST078: draws a ride’s wait as the mark its own payload names, not the page’s clock or the park’s hours', async ({
  page,
}) => {
  // The park's own hours say it is open right now, and the clock is held to a moment inside them —
  // and yet what each ride draws is read off its own wait field alone.
  const openNow = { open: '2026-08-31T09:00:00-04:00', close: '2026-08-31T22:00:00-04:00' };
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(MAGIC_KINGDOM, {
        hours: openNow,
        rides: [
          ride('Big Thunder Mountain Railroad', 25),
          ride('Splash Mountain', 'Closed'),
        ],
      }),
    ]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  const minutes = page.locator(`${WAIT}[data-pwt-wait-kind="minutes"]`);
  const state = page.locator(`${WAIT}[data-pwt-wait-kind="state"]`);
  await expect(minutes).toHaveText('25');
  await expect(state).toHaveText('Closed');

  // A length of time and a state are different kinds of mark, not merely different text
  // (SRS061<!-- The park-wait-times module draws a wait as the time or the not-operating state it
  // is handed -->; the UI design spec § The wait slot) — read as a class distinction between the
  // two.
  await expect(minutes).not.toHaveClass(/state/);
  await expect(state).toHaveClass(/state/);

  // And the state a ride draws is the payload's, not a computation this component made from the
  // hours or the clock: a ride the payload calls `Closed` draws `Closed` beside another ride, at the
  // same park, at the same moment, that the payload calls a numeric wait — if the drawing followed
  // the hours or the clock instead, the two rides could not disagree.
  await expect(minutes).not.toHaveText(/Closed/);
});

test('TST079: follows its source to a new reading inside the freshness bound, without reloading', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM, { rides: [ride('The Barnstormer', 10)] })]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  const wait = page.locator(WAIT).first();
  await expect(wait).toHaveText('10');

  // A mark on the page a reload would clear, so the change below is attributable to the module
  // re-reading rather than to the display having started over.
  await page.evaluate(() => {
    (window as unknown as { standing?: boolean }).standing = true;
  });

  // The source now says something else, staged as the answer a later poll receives.
  const afterTheChange = await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM, { rides: [ride('The Barnstormer', 35)] })]),
  }));

  await advanceHostClock(page, READ_INTERVAL_MS - ALMOST);
  expect(afterTheChange.urls, 'it had not asked again before its interval was up').toEqual([]);
  await advanceHostClock(page, ALMOST);

  await expect(wait).toHaveText('35');
  await expect(wait, 'the reading it opened with is gone').not.toHaveText('10');
  expect(afterTheChange.urls, 'the reading was asked for again').toHaveLength(1);
  expect(
    await page.evaluate(() => (window as unknown as { standing?: boolean }).standing),
    'the page never reloaded',
  ).toBe(true);
});

test('TST082: draws the wait times its own route answered with, and reads no other source', async ({
  page,
  baseURL,
}) => {
  // Registered before the page loads: a listener added afterwards would miss the load's own asks,
  // and an absence measured over nothing is not an absence.
  const traffic = watchTraffic(page);
  const served = await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM, { rides: [ride('The Barnstormer', 10)] })]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  await expect(page.locator(WAIT).first()).toHaveText('10');
  expect(served.urls.length, 'the module asked its own route').toBeGreaterThan(0);

  // And nothing else was asked — the shell's own two asks are named rather than every request being
  // permitted, so a second source would be left over rather than absorbed.
  expect([...new Set(asksBeyondTheShell(traffic))]).toEqual(['/api/park-wait-times']);
  expect(channelsBeyondTheTier(traffic, baseURL)).toEqual([]);
});

test('holds a park’s place in the grid and shows why, when that park’s own reading could not be produced', async ({
  page,
}) => {
  // One park fails, the other answers — the owner's per-park graceful-degradation ruling (WI-3),
  // read from the render side: the failing park holds its place and shows a reason, the healthy one
  // is unaffected.
  const REASON = 'The wait-times source did not answer for this park.';
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return {
      status: 200,
      data: parksPayload(
        parks.map((configured) =>
          configured === MAGIC_KINGDOM
            ? onePark(configured, { available: false, message: REASON })
            : onePark(configured, { rides: [ride('Test Track', 40)] }),
        ),
      ),
    };
  });
  await render(page, placed([MAGIC_KINGDOM, EPCOT], { columns: 2, rows: 1 }));

  await expect(page.locator(CARD)).toHaveCount(2);
  const failing = cardNamed(page, MAGIC_KINGDOM);
  const healthy = cardNamed(page, EPCOT);
  await expect(failing.locator(PARK_UNAVAILABLE)).toHaveText(REASON);
  await expect(failing.locator(LEADERBOARD_ROW)).toHaveCount(0);
  await expect(healthy.locator(LEADERBOARD_ROW)).toContainText('Test Track');

  // The failing park's own header — icon and name — stays drawn: the README's own wording is
  // "in place of its rides" (§ States, Park unavailable), not the whole card, so a viewer can tell
  // which park failed rather than reading only its grid position.
  await expect(failing.locator('[data-pwt-header]')).toBeVisible();
  await expect(failing.locator('[data-pwt-header]')).toContainText(MAGIC_KINGDOM);
});

test('renders each unavailable park’s own message verbatim, transient or permanent — the card never branches on the wording', async ({
  page,
}) => {
  // The permanent unsupported-park outcome and a transient failure are both `available: false`,
  // told apart only by the message the backend sends (#309 build spec decision 1). The card draws
  // whatever text it is handed, the same way for either — it never matches on the wording to treat
  // one specially. Two unavailable parks carrying distinct messages, each shown its own, is the
  // proof: these strings are the test's own data, not a copy of a backend contract this side
  // depends on — the exact unsupported wording is the backend's own, tested there.
  const transient = 'the source did not answer in time';
  const unsupported = 'the source has no such park';
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return {
      status: 200,
      data: parksPayload(
        parks.map((configured) => onePark(configured, { available: false, message: configured === EPCOT ? unsupported : transient })),
      ),
    };
  });
  await render(page, placed([MAGIC_KINGDOM, EPCOT], { columns: 2, rows: 1 }));

  await expect(
    cardNamed(page, MAGIC_KINGDOM).locator(PARK_UNAVAILABLE),
    'the transient failure shows its own message',
  ).toHaveText(transient);
  await expect(
    cardNamed(page, EPCOT).locator(PARK_UNAVAILABLE),
    'the permanent unsupported failure shows its own distinct message, through the same path',
  ).toHaveText(unsupported);
});

test('holds a park-unavailable card to a full card’s own height, the same hidden-skeleton reference the Closed card uses', async ({
  page,
}) => {
  const roster = [EPCOT, ISLANDS_OF_ADVENTURE, MAGIC_KINGDOM];
  const REASON = 'The wait-times source did not answer for this park.';
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return {
      status: 200,
      data: parksPayload(
        parks.map((configured) =>
          configured === EPCOT ? onePark(configured, { available: false, message: REASON }) : onePark(configured, { rides: rankedRoster() }),
        ),
      ),
    };
  });
  await render(page, placed(roster, { columns: 3, rows: 1 }));

  const heights = Object.fromEntries(
    await page
      .locator(CARD)
      .evaluateAll((els) => els.map((el) => [el.querySelector('.name')?.textContent ?? '', el.getBoundingClientRect().height] as const)),
  );
  expect(
    Math.abs(heights[EPCOT] - heights[MAGIC_KINGDOM]),
    'the unavailable card matches a full leaderboard-plus-tour card’s own height',
  ).toBeLessThan(1);
});

test('holds every card to a full card’s own height when every park is unavailable, not just when one full card is on screen to borrow from', async ({
  page,
}) => {
  const roster = [EPCOT, ISLANDS_OF_ADVENTURE, MAGIC_KINGDOM];
  const REASON = 'The wait-times source did not answer for this park.';
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured, { available: false, message: REASON }))) };
  });
  await render(page, placed(roster, { columns: 3, rows: 1 }));

  const unavailableHeights = await page.locator(CARD).evaluateAll((els) => els.map((el) => el.getBoundingClientRect().height));
  expect(
    new Set(unavailableHeights).size,
    'every unavailable card the same height, with no filled card on screen to borrow from',
  ).toBe(1);

  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured, { rides: rankedRoster() }))) };
  });
  await render(page, placed(roster, { columns: 3, rows: 1 }));

  const filledHeights = await page.locator(CARD).evaluateAll((els) => els.map((el) => el.getBoundingClientRect().height));
  expect(new Set(filledHeights).size, 'every filled card the same height').toBe(1);

  expect(
    Math.abs(unavailableHeights[0] - filledHeights[0]),
    'an all-unavailable grid holds the same footprint as a filled grid',
  ).toBeLessThan(1);
});

test('shows that it is reading while its route has not answered yet', async ({ page }) => {
  // The one state `serveModuleData` cannot drive: every answer it gives is an answer, and this is
  // what is on screen before there is one. The route is taken and never fulfilled, which is the
  // ask-in-flight the module first paints against.
  await page.route('**/api/*', () => {});
  await render(page, placed([MAGIC_KINGDOM]));

  const loading = page.locator(`[data-region="middle_center"] ${LOADING}`);
  await expect(loading).toBeVisible();
  await expect(loading).not.toBeEmpty();
  await expect(page.locator(CARD)).toHaveCount(0);
  await expect(page.locator(MODULE_UNAVAILABLE)).toHaveCount(0);
});

test('renders why its own route failed, in its own place, while the backend is reachable', async ({
  page,
}) => {
  const REASON = 'The wait-times source did not answer.';
  await serveModuleData(page, () => ({
    status: 502,
    data: { module: 'park_wait_times', cause: 'upstream_unavailable', message: REASON },
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  const box = page.locator(`[data-region="middle_center"] ${MODULE_UNAVAILABLE}`);
  await expect(box).toBeVisible();
  await expect(box).toContainText(REASON);
  await expect(page.locator(CARD)).toHaveCount(0);
  await expect(page.locator('[data-backend-unreachable]')).toHaveCount(0);
});

test('sets the module’s own left default against the region’s inherited centring — a guard, not proof of a visible fix', async ({
  page,
}) => {
  // Round 4 read L2/L3 as the ride-name bug's own class: the loading, module-unavailable and
  // park-unavailable lines inheriting the region's own `text-align` (`placementStyle()`, regions.ts).
  // Measured directly (Range over each line's own text node, against a card/region given deliberate
  // slack) and found neither one actually moves under `text-align: center` — `.waiting`
  // (ParkWaitTimes.svelte) shrink-wraps to its own text with the region's own `align-items` (no
  // slack for centring to show, in any region width tried, including a full-width band); `.unavailable`
  // (ParkCard.svelte) is positioned by its own `display: flex` default `justify-content: flex-start`,
  // which text-align does not govern — reproducibly zero shift even with ~265px of deliberate slack.
  // L2/L3 are not visible, reproducible defects; `.park-wait-times`'s own `text-align: left` stays as
  // defensive hygiene (the same contract-only status L1's hours-weight drift holds) rather than a
  // claim this test can back with geometry.
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM)]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  const textAlign = await page.locator(MODULE).evaluate((el) => getComputedStyle(el).textAlign);
  expect(textAlign, 'the module’s own root sets a left default against the region’s own anchor').toBe('left');
});

test('lays out uniform, aligned cards that do not run past the viewport, with real park and ride names', async ({
  page,
}) => {
  // The card is a fixed width (ParkWaitTimes.svelte's measured `--pwt-card-width`), not one grown to
  // its own content — a name too wide for its column scrolls to reveal itself (`ParkCard.svelte`'s
  // `marquee`) rather than widening the card or wrapping the grid past the viewport, and every card
  // takes the same width regardless of what its own park's names are. Read at the deployed
  // three-column shape (config.json), over a real (unabbreviated) roster.
  const roster = [
    MAGIC_KINGDOM,
    EPCOT,
    HOLLYWOOD_STUDIOS,
    ANIMAL_KINGDOM,
    UNIVERSAL_STUDIOS,
    ISLANDS_OF_ADVENTURE,
  ];
  const names: Record<string, string> = {
    [MAGIC_KINGDOM]: 'Magic Kingdom',
    [EPCOT]: 'Epcot',
    [HOLLYWOOD_STUDIOS]: 'Hollywood Studios',
    [ANIMAL_KINGDOM]: 'Animal Kingdom',
    [UNIVERSAL_STUDIOS]: 'Universal Studios',
    [ISLANDS_OF_ADVENTURE]: 'Islands of Adventure',
  };
  const rides: Record<string, ParkWaitTimesRide[]> = {
    [MAGIC_KINGDOM]: [
      ride('Seven Dwarfs Mine Train', 90),
      ride("Walt Disney's Carousel of Progress", 15),
    ],
    [EPCOT]: [
      ride('Guardians of the Galaxy: Cosmic Rewind', 75),
      ride('Remy’s Ratatouille Adventure', 40),
    ],
    [HOLLYWOOD_STUDIOS]: [
      ride('Star Wars: Rise of the Resistance', 85),
      ride('Mickey & Minnie’s Runaway Railway', 35),
    ],
    [ANIMAL_KINGDOM]: [
      ride('Avatar Flight of Passage', 95),
      ride('Expedition Everest - Legend of the Forbidden Mountain', 30),
    ],
    [UNIVERSAL_STUDIOS]: [
      ride("Harry Potter and the Escape from Gringotts", 70),
      ride('Revenge of the Mummy', 25),
    ],
    [ISLANDS_OF_ADVENTURE]: [
      ride('Harry Potter and the Forbidden Journey', 80),
      ride('Jurassic World VelociCoaster', 45),
    ],
  };
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return {
      status: 200,
      data: parksPayload(parks.map((configured) => onePark(configured, { name: names[configured], rides: rides[configured] }))),
    };
  });
  await render(page, placed(roster, { columns: 3, rows: 2 }));

  const cards = page.locator(CARD);
  await expect(cards).toHaveCount(roster.length);

  const boxes = await cards.evaluateAll((els) => els.map((el) => el.getBoundingClientRect()));
  const widths = new Set(boxes.map((box) => box.width));
  expect(widths.size, 'every card the same width').toBe(1);

  // The two rows of three: each row's own three cards share one top, proving the grid holds its
  // shape rather than one card (an empty leaderboard, an overflowing name) sitting off the line.
  const tops = new Set(boxes.map((box) => box.top));
  expect(tops.size, 'exactly two distinct row positions').toBe(2);

  const { scrollWidth, clientWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(scrollWidth).toBeLessThanOrEqual(clientWidth);
});

test('leaves a park’s leaderboard at its own real row count — no permanent blank row where it holds fewer than three numeric waits', async ({
  page,
}) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, {
        rides: [
          ride('Test Track', 60),
          ride('Spaceship Earth', 10),
        ],
      }),
    ]),
  }));
  await render(page, placed([EPCOT]));

  const card = page.locator(CARD);
  await expect(card.locator(LEADERBOARD_ROW)).toHaveCount(2);
  // The mechanism that reserved a third slot regardless of how many rides a park holds is gone —
  // not merely unfilled here, absent from the DOM.
  await expect(card.locator('[data-pwt-leaderboard-placeholder]')).toHaveCount(0);
});

test('holds the footer’s own position across the tour’s pages, including a last page short a ride', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(MAGIC_KINGDOM, {
        rides: [
          ride('Seven Dwarfs Mine Train', 90),
          ride('Big Thunder Mountain Railroad', 45),
          ride('Pirates of the Caribbean', 30),
          ride("Walt Disney's Carousel of Progress", 10),
          ride('Tomorrowland Speedway', 8),
          ride('Jungle Cruise', 6),
          ride("It's a Small World", 4),
          ride('Haunted Mansion', 2),
        ],
      }),
    ]),
  }));
  await render(page, placed([MAGIC_KINGDOM], { rotationIntervalSeconds: 8 }));

  const card = page.locator(CARD);
  const footerY = async () => (await card.locator(FOOTER_SEGMENT).first().boundingBox())?.y;

  // Held: Seven Dwarfs Mine Train, Big Thunder Mountain Railroad, Pirates of the Caribbean.
  // Remaining, in source order: Carousel of Progress, Tomorrowland Speedway, Jungle Cruise, Small
  // World, Haunted Mansion — five, an odd count, three pages (2, 2, 1).
  const footer0 = await footerY();
  await expect(card.locator(TOUR_ROW)).toHaveCount(2);

  await advanceHostClock(page, 8 * 1000);
  await expect(card.locator(TOUR_ROW)).toHaveCount(2);
  expect(await footerY()).toBe(footer0);

  await advanceHostClock(page, 8 * 1000);
  // The odd last page: one real row, padded by exactly one blank row so the footer beneath it does
  // not move up for having one fewer ride to show — padding the two full pages above never carried.
  await expect(card.locator(TOUR_ROW)).toHaveCount(1);
  await expect(card.locator('[data-pwt-tour-placeholder]')).toHaveCount(1);
  expect(await footerY()).toBe(footer0);
});

test('anchors the footer to the card’s own bottom edge, with nothing beneath it', async ({ page }) => {
  // Read here even against a single card, the footer must sit flush with the card's own inner
  // bottom edge (inside its padding and border) rather than leaving trailing space below it — a
  // card is its own natural height, so nothing beneath the footer is left to fill.
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT]));

  const card = page.locator(CARD);
  const cardBox = await card.boundingBox();
  const footerBox = await card.locator(FOOTER).boundingBox();
  if (!cardBox || !footerBox) {
    throw new Error('the card or its footer did not render a box');
  }
  // `boundingBox` reads the card's own border box; its inner (padding) bottom edge is that box's
  // bottom less its own bottom border and padding.
  const { paddingBottom, borderBottomWidth } = await card.evaluate((element) => {
    const style = getComputedStyle(element);
    return {
      paddingBottom: parseFloat(style.paddingBottom),
      borderBottomWidth: parseFloat(style.borderBottomWidth),
    };
  });
  const cardInnerBottom = cardBox.y + cardBox.height - paddingBottom - borderBottomWidth;
  const gap = cardInnerBottom - (footerBox.y + footerBox.height);
  expect(Math.abs(gap), 'the footer’s bottom edge sits flush with the card’s own inner bottom edge').toBeLessThan(1);
});

test('holds two full cards — a leaderboard and its own More Waits block — to the same natural height, neither forced by a min-height', async ({
  page,
}) => {
  // Two different real rosters, each with a full three-ride leaderboard plus at least one page of
  // touring rides, so a coincidence of ride counts cannot explain the two landing on the same height
  // — every real park draws the same structure, so with nothing forcing it they line up on their own
  // (owner ruling, #309).
  const rides: Record<string, ParkWaitTimesRide[]> = {
    [MAGIC_KINGDOM]: rankedRoster(),
    [EPCOT]: [
      ride('Guardians of the Galaxy: Cosmic Rewind', 75),
      ride('Remy’s Ratatouille Adventure', 40),
      ride('Test Track', 25),
      ride('Soarin’', 15),
    ],
  };
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured, { rides: rides[configured] }))) };
  });
  await render(page, placed([MAGIC_KINGDOM, EPCOT], { columns: 2, rows: 1 }));

  const cards = page.locator(CARD);
  const heights = await cards.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().height));
  expect(Math.abs(heights[0] - heights[1]), 'both full cards land at the same natural height').toBeLessThan(1);

  // Neither card carries a forced floor: an open card's `min-height` is left at the property's own
  // initial value — `auto`, not `0px`, being a grid item (a grid item's `auto` does not stretch a
  // card past its own content the way it would default `align-items` to do; `.grid`'s own
  // `align-items: start` is what leaves each card at its own height, ParkWaitTimes.svelte).
  const minHeights = await cards.evaluateAll((els) => els.map((el) => getComputedStyle(el).minHeight));
  expect(minHeights).toEqual(['auto', 'auto']);
});

test('leaves no slack between the leaderboard and the More Waits divider — the same gap as the header’s own, not a leftover from forcing height', async ({
  page,
}) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT]));

  const card = page.locator(CARD);
  const headerBox = await card.locator(HEADER).boundingBox();
  const leaderboardBox = await card.locator(LEADERBOARD).boundingBox();
  const moreBox = await card.locator(MORE_WAITS).boundingBox();
  if (!headerBox || !leaderboardBox || !moreBox) {
    throw new Error('the header, leaderboard or More Waits block did not render a box');
  }
  const headerToLeaderboardGap = leaderboardBox.y - (headerBox.y + headerBox.height);
  const leaderboardToMoreGap = moreBox.y - (leaderboardBox.y + leaderboardBox.height);
  // `.card`'s own single flex `gap` (the styling contract's `md` step) is what sets every one of its
  // children apart, header-to-leaderboard the same as leaderboard-to-More-Waits — proving there is no
  // extra slack between the leaderboard and More Waits beyond that one gap. The bug this rewrites
  // forced a taller card and pushed More Waits down (`margin-top: auto`) to fill it, landing the
  // difference here instead.
  expect(
    Math.abs(leaderboardToMoreGap - headerToLeaderboardGap),
    'the leaderboard-to-More-Waits gap is the same as the header-to-leaderboard gap',
  ).toBeLessThan(1);
});

test('holds the Closed card to a full card’s own height, without forcing an open card that has less to show', async ({
  page,
}) => {
  const roster = [EPCOT, ISLANDS_OF_ADVENTURE, MAGIC_KINGDOM];
  const rides: Record<string, ParkWaitTimesRide[]> = {
    [EPCOT]: [
      ride('Test Track', 60),
      ride('Spaceship Earth', 10),
    ],
    [ISLANDS_OF_ADVENTURE]: [
      ride('Harry Potter and the Forbidden Journey', 'Closed'),
      ride('Jurassic World VelociCoaster', 'Down'),
    ],
    [MAGIC_KINGDOM]: [
      ride('Seven Dwarfs Mine Train', 90),
      ride('Big Thunder Mountain Railroad', 45),
      ride('Pirates of the Caribbean', 30),
      ride("Walt Disney's Carousel of Progress", 10),
      ride('Tomorrowland Speedway', 8),
    ],
  };
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return {
      status: 200,
      data: parksPayload(parks.map((configured) => onePark(configured, { rides: rides[configured] }))),
    };
  });
  await render(page, placed(roster, { columns: 3, rows: 1 }));

  const heights = Object.fromEntries(
    await page
      .locator(CARD)
      .evaluateAll((els) =>
        els.map((el) => [el.querySelector('.name')?.textContent ?? '', el.getBoundingClientRect().height] as const),
      ),
  );

  // The Closed card (no ride reporting a length of time) matches a full leaderboard-plus-More-Waits
  // card's own height — not forced by anything external, but because its own hidden skeleton
  // (`ParkCard.svelte`'s `.full-frame`) draws that same structure itself.
  expect(
    Math.abs(heights[ISLANDS_OF_ADVENTURE] - heights[MAGIC_KINGDOM]),
    'the Closed card matches the full card’s height',
  ).toBeLessThan(1);

  // epcot holds a leaderboard (two numeric rides) but nothing left to tour — open, its own natural
  // height, smaller than the Closed card's own reserved full-card footprint (owner ruling, #309).
  expect(
    heights[MAGIC_KINGDOM] - heights[EPCOT],
    'an open card with less to show is left shorter, not forced to match',
  ).toBeGreaterThan(10);

  // Neither `.card`'s own min-height nor `.grid`'s own item-stretch (its default `align-items`,
  // ParkWaitTimes.svelte) may leave epcot's own box taller than its own content: the card's inner
  // bottom edge (its border, one card-padding below the leaderboard) sits flush against the
  // leaderboard itself, not against the row's tallest neighbour.
  const epcotCard = cardNamed(page, EPCOT);
  const epcotBox = await epcotCard.boundingBox();
  const epcotLeaderboardBox = await epcotCard.locator(LEADERBOARD).boundingBox();
  if (!epcotBox || !epcotLeaderboardBox) {
    throw new Error('epcot’s card or leaderboard did not render a box');
  }
  const cardBottomChrome = await epcotCard.evaluate((el) => {
    const style = getComputedStyle(el);
    return parseFloat(style.paddingBottom) + parseFloat(style.borderBottomWidth);
  });
  const trailingSpace =
    epcotBox.y + epcotBox.height - (epcotLeaderboardBox.y + epcotLeaderboardBox.height) - cardBottomChrome;
  expect(Math.abs(trailingSpace), 'no dead space below epcot’s own leaderboard, past its own padding').toBeLessThan(
    1,
  );
});

test('holds every card to a full card’s own height when every park is Closed, not just when one full card is on screen to borrow from', async ({
  page,
}) => {
  const roster = [EPCOT, ISLANDS_OF_ADVENTURE, MAGIC_KINGDOM];
  const allClosed: ParkWaitTimesRide[] = [
    ride('Guardians of the Galaxy: Cosmic Rewind', 'Closed'),
    ride('Space Mountain', 'Down'),
  ];

  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured, { rides: allClosed }))) };
  });
  await render(page, placed(roster, { columns: 3, rows: 1 }));

  const closedHeights = await page.locator(CARD).evaluateAll((els) => els.map((el) => el.getBoundingClientRect().height));
  expect(
    new Set(closedHeights).size,
    'every Closed card the same height, with no open card on screen for a live-measured floor to borrow from',
  ).toBe(1);

  // The same roster, every park filled — the reference a Closed card's own hidden skeleton
  // (`ParkCard.svelte`'s `.full-frame`) draws, whether or not any other card on the page happens
  // to be filled too.
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured, { rides: rankedRoster() }))) };
  });
  await render(page, placed(roster, { columns: 3, rows: 1 }));

  const filledHeights = await page.locator(CARD).evaluateAll((els) => els.map((el) => el.getBoundingClientRect().height));
  expect(new Set(filledHeights).size, 'every filled card the same height').toBe(1);

  expect(
    Math.abs(closedHeights[0] - filledHeights[0]),
    'an all-Closed grid holds the same footprint as a filled grid',
  ).toBeLessThan(1);
});

test('draws every ride name flush left in its own column, whatever its own length — the wait stays right', async ({
  page,
}) => {
  // `placed`'s own default region (middle_center) is centre-anchored — the exact case that
  // centred every ride name before this fix: RegionFrame's own `text-align` (`placementStyle()`,
  // regions.ts) inherits straight through ParkCard.svelte's rows unless a row explicitly
  // overrides it, the same way `.wait`'s own `text-align: right` already does. The park's own
  // name is long enough that the card — and so the ride-name column, which the park's own header
  // sizes (ParkWaitTimes.svelte's measured `--pwt-card-width`), never a ride name — is wider than the shorter
  // ride name below: a column no wider than every name in it would leave a centred name
  // indistinguishable from a left-aligned one, both overflowing the same way.
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, {
        name: 'Islands of Adventure',
        hours: { open: '2026-01-01T09:00:00Z', close: '2026-01-01T21:00:00Z' },
        rides: [
          ride('Guardians of the Galaxy: Cosmic Rewind', 65),
          ride('Frozen Ever After', 40),
        ],
      }),
    ]),
  }));
  await render(page, placed([EPCOT]));

  const rows = page.locator(LEADERBOARD_ROW);
  const names = await rows.evaluateAll((els) =>
    els.map((el) => {
      const column = el.querySelector('[data-pwt-ride-name]') as HTMLElement;
      const text = column.querySelector('span') as HTMLElement;
      return { columnLeft: column.getBoundingClientRect().x, textLeft: text.getBoundingClientRect().x };
    }),
  );
  for (const { columnLeft, textLeft } of names) {
    expect(Math.abs(textLeft - columnLeft), 'the ride name starts flush at its own column’s left edge').toBeLessThan(
      1,
    );
  }
  // The direct proof a shorter name is not centred within a column a longer one fills: both start
  // at the very same x regardless of the name's own length.
  expect(
    Math.abs(names[0].textLeft - names[1].textLeft),
    'a shorter ride name starts at the same x as a longer one',
  ).toBeLessThan(1);

  const waitTextAlign = await rows.first().locator(WAIT).evaluate((el) => getComputedStyle(el).textAlign);
  expect(waitTextAlign, 'the wait figure stays right-aligned').toBe('right');
});

test('sizes every card to the widest rendered header, and no wider — the true content, not the probe’s own formula', async ({
  page,
}) => {
  const roster = [MAGIC_KINGDOM, EPCOT, ISLANDS_OF_ADVENTURE];
  const names: Record<string, string> = {
    [MAGIC_KINGDOM]: 'Magic Kingdom',
    [EPCOT]: 'Epcot',
    [ISLANDS_OF_ADVENTURE]: 'Islands of Adventure',
  };
  const hours = { open: '2026-01-01T09:00:00Z', close: '2026-01-01T21:00:00Z' };
  await serveModuleData(page, (_asked, body) => {
    const { parks } = body as { parks: string[] };
    return { status: 200, data: parksPayload(parks.map((configured) => onePark(configured, { name: names[configured], hours }))) };
  });
  await render(page, placed(roster, { columns: 3, rows: 1 }));

  // 'Islands of Adventure' is the widest header in this roster — read its own true rendered content
  // span (identity's own left edge to hours' own right edge) directly off the DOM, rather than
  // re-deriving ParkWaitTimes.svelte's own probe formula: a probe that has drifted from the CSS it
  // measures (L1 — the hours were probed at the wrong font-weight) computes the same wrong number
  // every card shares, so comparing cards only to each other cannot catch it.
  const widest = cardNamed(page, ISLANDS_OF_ADVENTURE);
  const measured = await widest.evaluate((card) => {
    const identity = card.querySelector('.identity') as HTMLElement;
    const hoursEl = card.querySelector('[data-pwt-hours]') as HTMLElement;
    const style = getComputedStyle(card);
    return {
      cardWidth: card.getBoundingClientRect().width,
      headerContentWidth: hoursEl.getBoundingClientRect().right - identity.getBoundingClientRect().left,
      chrome:
        parseFloat(style.paddingLeft) +
        parseFloat(style.paddingRight) +
        parseFloat(style.borderLeftWidth) +
        parseFloat(style.borderRightWidth),
    };
  });

  const expectedCardWidth = Math.ceil(measured.headerContentWidth + measured.chrome);
  expect(
    Math.abs(measured.cardWidth - expectedCardWidth),
    'the card is exactly the widest header’s own rendered width, no wider and no narrower',
  ).toBeLessThan(2);
  // The near-equality check above passes if both sides collapse to zero together.
  expect(measured.cardWidth, 'the card actually has a width, not a collapsed one both sides agree on').toBeGreaterThan(0);
});

test('draws every wait right-aligned and tabular, the column never moving under a changing digit count', async ({
  page,
}) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, {
        rides: [
          ride('A', 5),
          ride('B', 45),
          ride('C', 120),
        ],
      }),
    ]),
  }));
  await render(page, placed([EPCOT]));

  const waits = page.locator(WAIT);
  await expect(waits).toHaveCount(3);

  const textAlign = await waits.first().evaluate((el) => getComputedStyle(el).textAlign);
  expect(textAlign, 'the wait column reads right-aligned').toBe('right');

  const variants = await waits.evaluateAll((els) => els.map((el) => getComputedStyle(el).fontVariantNumeric));
  for (const variant of variants) {
    expect(variant, 'a numeric wait renders tabular figures').toContain('tabular-nums');
  }

  // The column's own reserved box — not the glyphs' own position, which text-align already covers
  // above — never moves, whether the figure is one, two or three digits.
  const lefts = await waits.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().x));
  expect(new Set(lefts).size, 'the wait column’s own left edge holds across 1, 2 and 3 digits').toBe(1);
});

test('reserves one constant wait column across a numeric wait and the longest not-operating word alike, neither wider than the other', async ({
  page,
}) => {
  // The column's reservation must clear the widest reading it is ever handed — a six-letter
  // not-operating word ("REFURB"/"CLOSED"), wider on the page than a three-digit "999" — so it
  // never moves between a numeric row and a state row (SRS058, SRS061). A card whose leaderboard
  // holds numeric waits and whose tour holds those two words shows both kinds at once: every wait
  // box the same width is the proof the reservation covers the longest word rather than that word
  // growing its own box past a shorter one's.
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, {
        rides: [
          ride('A', 5),
          ride('B', 45),
          ride('C', 120),
          ride('D', 'Refurb'),
          ride('E', 'Closed'),
        ],
      }),
    ]),
  }));
  await render(page, placed([EPCOT]));

  const waits = page.locator(WAIT);
  await expect(waits).toHaveCount(5);
  const kinds = await waits.evaluateAll((els) => els.map((el) => el.getAttribute('data-pwt-wait-kind')));
  expect(new Set(kinds), 'both a numeric wait and a not-operating word are on screen').toEqual(
    new Set(['minutes', 'state']),
  );
  const widths = await waits.evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().width)));
  expect(new Set(widths).size, 'every wait box the same width, numeric and state alike').toBe(1);
});

test('scrolls a ride name too wide for its own column — held home, one constant-speed pass to the end, then home until the next tick', async ({
  page,
}) => {
  // The motion is the name COLUMN's own `scrollLeft` — `.ride-name`, the clipping scroll container
  // the placement's marquee clock writes (marquee-clock.ts) — never a transform on the oversized
  // text inside it.
  //
  // Every instant below is derived from the distance THIS render measures, so the schedule is read
  // at each viewport's own geometry rather than against a figure written here. The name is
  // deliberately short: it only has to overflow, and a smaller distance is a shorter cycle to drive.
  const overflowingName = 'Cosmic Rewind';
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride(overflowingName, 40)] })]),
  }));
  // A rotation interval longer than the whole cycle read below: the tick that starts the next pass
  // must not land inside this one.
  await render(page, placed([EPCOT], { rotationIntervalSeconds: 60 }));

  const column = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`).first();
  await expect(column, 'the full name is in the DOM, not truncated').toHaveText(overflowingName);
  await page.clock.runFor(MEASURE_FRAME_MS);

  // The distance the clock is registered with, and so the schedule's own scale: the column's own
  // `scrollWidth - clientWidth` (marquee-clock.ts's `registerMarquee`).
  const distance = await column.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(distance, 'the fixture’s name really does overflow its own column').toBeGreaterThan(0);
  const moveSeconds = distance / MARQUEE_PX_PER_S;

  /** Drives the held clock to `seconds` after the cycle began, and reads the column's own offset.
      The cycle begins on the first frame of the run above, so a reading lags its instant by at most
      that frame's own worth of travel (`FRAME_SLACK_PX`). */
  let drivenMs = MEASURE_FRAME_MS;
  const scrolledAt = async (seconds: number): Promise<number> => {
    const target = Math.round(seconds * 1000);
    await page.clock.runFor(target - drivenMs);
    drivenMs = target;
    return column.evaluate((el) => el.scrollLeft);
  };

  expect(await scrolledAt(HOLD_HOME_S - 0.2), 'still home through the opening hold').toBe(0);

  // One pace, not a duration fitted to the name: two samples a second apart each land where
  // MARQUEE_PX_PER_S alone puts them, while the column still has travel left to give.
  expect(
    2 * MARQUEE_PX_PER_S,
    'the later ramp sample still has travel left, so it reads the ramp rather than the clamp',
  ).toBeLessThan(distance);
  const afterOneSecond = await scrolledAt(HOLD_HOME_S + 1);
  const afterTwoSeconds = await scrolledAt(HOLD_HOME_S + 2);
  expect(
    Math.abs(afterOneSecond - MARQUEE_PX_PER_S),
    'a second into the pass, a second of travel at the module’s own pace',
  ).toBeLessThanOrEqual(FRAME_SLACK_PX);
  expect(Math.abs(afterTwoSeconds - 2 * MARQUEE_PX_PER_S), 'two seconds in, twice that').toBeLessThanOrEqual(
    FRAME_SLACK_PX,
  );

  expect(
    await scrolledAt(HOLD_HOME_S + moveSeconds + HOLD_END_S / 2),
    'clamped at the end of the name and held there, so the end of it can be read',
  ).toBe(distance);

  expect(
    await scrolledAt(HOLD_HOME_S + moveSeconds + HOLD_END_S + 0.3),
    'home again once the closing hold elapses',
  ).toBe(0);

  // One pass per rotation tick, never a loop of its own: `startMarqueeCycle` is the only thing that
  // begins another, and ParkWaitTimes.svelte calls it on the tick the cards flip on. A name long
  // enough that one pass at this pace shows only part of it is read in part — the designed reading,
  // not a shortfall the scroll should make up by going faster.
  expect(
    await scrolledAt(HOLD_HOME_S + moveSeconds + HOLD_END_S + 6),
    'still home six seconds on — nothing but the next tick restarts the pass',
  ).toBe(0);
});

test('leaves a ride name that already fits its own column unregistered, while an overflowing name on the same card scrolls', async ({
  page,
}) => {
  // A park with a long enough name/hours that its own header — the ride-name column's own width,
  // never a ride name — leaves genuine room for 'Test Track' to fit: a short park name (this
  // fixture's own default) produces too narrow a column, and 'Test Track' would overflow it for an
  // unrelated reason, a false positive for this specific invariant.
  //
  // The overflowing ride beside it is the control that makes the fitting row's bare absence of
  // `.marquee` mean something. The action sets that class on the same animation frame it registers
  // the column with the clock, so waiting for it on the overflowing row is what proves the
  // measurement pass has run at all: an absence read before that pass would pass over a column that
  // does get registered a frame later, and it is this one row rather than both that goes red if a
  // pass ever registers every row regardless of overflow.
  const fittingName = 'Test Track';
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, {
        name: ISLANDS_OF_ADVENTURE,
        hours: { open: '2026-01-01T09:00:00Z', close: '2026-01-01T21:00:00Z' },
        rides: [ride(OVERFLOWING_RIDE_NAME, 80), ride(fittingName, 40)],
      }),
    ]),
  }));
  await render(page, placed([EPCOT]));

  const columns = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`);
  await expect(columns).toHaveCount(2);
  const overflowingRow = columns.nth(0);
  const fittingRow = columns.nth(1);
  await expect(overflowingRow, 'the longer wait ranks first').toHaveText(OVERFLOWING_RIDE_NAME);
  await expect(fittingRow).toHaveText(fittingName);

  await expect(
    overflowingRow.locator('.ride-name-text'),
    'the overflowing name is registered with the marquee clock',
  ).toHaveClass(/marquee/);
  await expect(
    fittingRow.locator('.ride-name-text'),
    'a name that already fits its column is not',
  ).not.toHaveClass(/marquee/);

  // Self-check the fixture at this render's own geometry, and at the distance registration is
  // measured on — the column's own, never the text node's against its parent. The two rows really
  // are the two cases, rather than both overflowing or both fitting.
  const overflowOf = (locator: typeof columns) => locator.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(await overflowOf(overflowingRow), 'the long name really does overflow its own column').toBeGreaterThan(0);
  expect(await overflowOf(fittingRow), 'the short one really does fit').toBeLessThanOrEqual(0);
});

test('re-measures the marquee after a poll refresh reorders rows in place, not just at mount', async ({
  page,
}) => {
  // The leaderboard's `{#each ... (index)}` keeps each row's DOM node across a reorder, so the ride
  // shown under a row changes without the row remounting — `use:marquee={ride.name}` (ParkCard.svelte)
  // re-runs the measurement on that change (the action's `update`) and reconciles the registration
  // both ways, which is what this asserts: a row that gains an overflowing name is registered with
  // the marquee clock, one that loses it is dropped. `.marquee` is the probe because registration
  // is the only thing that changes in the shrinking direction — a column with nothing left to
  // scroll sits at offset zero whether or not a stale registration is still driving it, so its own
  // `scrollLeft` cannot tell the two apart.
  const SHORT = 'A';
  const LONG = OVERFLOWING_RIDE_NAME;

  // Driving the five-minute read interval costs some eighty times more under `page.clock` with the
  // marquee's frame loop running (marquee-clock.ts).
  test.setTimeout(3 * 60 * 1000);

  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride(SHORT, 50), ride(LONG, 10)] })]),
  }));
  await render(page, placed([EPCOT]));

  const names = page.locator('.ride-name-text');
  await expect(names).toHaveCount(2);
  const row0 = names.nth(0);
  const row1 = names.nth(1);
  await expect(row0, 'the higher wait ranks first').toHaveText(SHORT);
  await expect(row1).toHaveText(LONG);

  // Self-check the fixture, at this render's own geometry and at the distance registration is
  // measured on: the COLUMN's own `scrollWidth - clientWidth` (marquee-clock.ts), never the text
  // node's against its parent. A fixture where the long name did not actually overflow, or the
  // short one did, could pass the invariant below without ever exercising the bug.
  const columns = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`);
  await expect(columns).toHaveCount(2);
  const overflowOf = (locator: typeof columns) => locator.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(await overflowOf(columns.nth(1)), 'the long name overflows its own column').toBeGreaterThan(0);
  expect(await overflowOf(columns.nth(0)), 'the short name does not').toBeLessThanOrEqual(0);

  // A poll refresh of the same card, not a remount: the waits reorder so the long name now ranks
  // first (row 0, previously the short name's) and the short name second (row 1, previously the long
  // name's) — the index-keyed rows persist across this, the precondition that makes the
  // stale-measurement bug reachable.
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride(SHORT, 5), ride(LONG, 80)] })]),
  }));
  await advanceHostClock(page, READ_INTERVAL_MS);
  await expect(row0, 'the reorder landed').toHaveText(LONG);
  await expect(row1).toHaveText(SHORT);
  // `page.clock` fakes requestAnimationFrame along with the timers `holdHostClock`/`advanceHostClock`
  // drive, so `marquee`'s rAF-scheduled measurement stays queued rather than firing on a real frame —
  // one more run of the fake clock is what lets it fire.
  await page.clock.runFor(1000);

  // The invariant itself: a row is registered with the marquee clock iff its own COLUMN overflows —
  // derived from each row's live geometry, not a hardcoded "row 0 marquees", so it stays correct
  // regardless of which name is overflowing.
  await expect
    .poll(
      () =>
        columns.evaluateAll((els) =>
          els.every((el) => {
            const overflows = el.scrollWidth > el.clientWidth;
            return el.querySelector('.ride-name-text')?.classList.contains('marquee') === overflows;
          }),
        ),
      { message: 'each ride name is registered iff its own column overflows, after the reorder' },
    )
    .toBe(true);
});

test('starts every overflowing name on the placement’s one clock — two columns of different widths leave home together and share a pace, not a duration', async ({
  page,
}) => {
  // One loop over one cycle timestamp, read by every registered column (marquee-clock.ts), is what
  // starts them together. Two cards whose names need different distances is what tells that apart
  // from both alternatives: columns given a shared DURATION would sit at different offsets at the
  // same instant, and columns started separately would leave home at different ones.
  const shorterName = 'Guardians of the Galaxy: Cosmic Rewind';
  const longerName = 'Tron Lightcycle Run: The Complete Extended Experience';
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, { rides: [ride(shorterName, 40)] }),
      onePark(MAGIC_KINGDOM, { rides: [ride(longerName, 40)] }),
    ]),
  }));
  await render(page, placed([EPCOT, MAGIC_KINGDOM], { rotationIntervalSeconds: 60 }));

  const columns = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`);
  await expect(columns).toHaveCount(2);
  await page.clock.runFor(MEASURE_FRAME_MS);

  // Self-check the fixture: both names overflow, and by genuinely different distances — the grid
  // gives both cards one width, so the two columns differ only in the name each holds.
  const distances = await columns.evaluateAll((els) => els.map((el) => el.scrollWidth - el.clientWidth));
  const [shorter, longer] = distances;
  expect(shorter, 'the shorter name still overflows its own column').toBeGreaterThan(0);
  expect(shorter, 'the two columns have genuinely different distances to cover').toBeLessThan(longer);

  let drivenMs = MEASURE_FRAME_MS;
  const scrolledAt = async (seconds: number): Promise<number[]> => {
    const target = Math.round(seconds * 1000);
    await page.clock.runFor(target - drivenMs);
    drivenMs = target;
    return columns.evaluateAll((els) => els.map((el) => el.scrollLeft));
  };

  expect(await scrolledAt(HOLD_HOME_S - 0.2), 'both still home through the one opening hold').toEqual([0, 0]);

  // Together, and at the one pace: the same offset on both columns at the same instant, while each
  // still has travel left to give.
  expect(2 * MARQUEE_PX_PER_S, 'both columns are still ramping at the later sample').toBeLessThan(shorter);
  const afterOneSecond = await scrolledAt(HOLD_HOME_S + 1);
  expect(afterOneSecond[0], 'a second into the pass, the columns have left home').toBeGreaterThan(0);
  expect(new Set(afterOneSecond).size, 'and are at the one offset between them').toBe(1);
  expect(new Set(await scrolledAt(HOLD_HOME_S + 2)).size, 'two seconds in, still the one offset').toBe(1);

  // The shorter column reaches its own end first and is held there while the longer one carries on
  // past it — one velocity across the rows. A shared duration would have read as the two arriving
  // together instead.
  const atShorterEnd = await scrolledAt(HOLD_HOME_S + shorter / MARQUEE_PX_PER_S + 0.5);
  expect(atShorterEnd[0], 'the shorter name is clamped at its own end').toBe(shorter);
  expect(atShorterEnd[1], 'the longer one is already past that offset').toBeGreaterThan(shorter);
  expect(atShorterEnd[1], 'and has not reached its own end yet').toBeLessThan(longer);
});

test('restarts every scroll on the rotation tick the cards flip on, rather than on a clock of its own', async ({
  page,
}) => {
  // ParkWaitTimes.svelte's single rotation interval both advances the tick the cards page on and
  // calls `startMarqueeCycle` (marquee-clock.ts) — a card flip and a marquee restart are one event
  // on one clock rather than two that drift apart. Read on either side of a single tick: the
  // footer's own filled segment advances, and a scroll that was in flight is back home and sets off
  // again.
  const ROTATION_S = 8;
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, {
        rides: [
          // Ranked first, so this row keeps its place across the flip and its column is the one
          // thing on the card a restart can be read off. The rest make the four remaining rides the
          // tour needs for two pages, so the footer has a position to advance.
          ride(OVERFLOWING_RIDE_NAME, 80),
          ride('Soarin', 50),
          ride('Spaceship Earth', 20),
          ride('Mission: Space', 10),
          ride('Imagination!', 5),
          ride('The Seas', 8),
          ride('Living with the Land', 3),
        ],
      }),
    ]),
  }));
  await render(page, placed([EPCOT], { rotationIntervalSeconds: ROTATION_S }));

  const column = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`).first();
  await expect(column, 'the scrolling name holds the card’s first row').toHaveText(OVERFLOWING_RIDE_NAME);
  const segments = page.locator(CARD).locator(FOOTER_SEGMENT);
  await expect(segments).toHaveCount(2);

  let drivenMs = 0;
  const driveTo = async (ms: number): Promise<void> => {
    await page.clock.runFor(Math.round(ms) - drivenMs);
    drivenMs = Math.round(ms);
  };
  const scrollLeft = () => column.evaluate((el) => el.scrollLeft);
  const tickMs = ROTATION_S * 1000;

  // Just short of the tick: the first page is still the one marked, and the scroll is under way.
  await driveTo(tickMs - 200);
  await expect(segments.nth(0), 'the tour has not advanced yet').toHaveClass(/filled/);
  expect(await scrollLeft(), 'the scroll is in flight when the tick lands').toBeGreaterThan(0);

  // Across it: the card flipped, and that same tick put the column back home.
  await driveTo(tickMs + 300);
  await expect(segments.nth(1), 'the tour advanced on the tick').toHaveClass(/filled/);
  expect(await scrollLeft(), 'and the scroll is home again on that one tick').toBe(0);

  // Home because the cycle restarted, not because the column stopped: the new cycle's own opening
  // hold elapses and it sets off again at the module's own pace.
  await driveTo(tickMs + (HOLD_HOME_S + 1) * 1000);
  expect(
    Math.abs((await scrollLeft()) - MARQUEE_PX_PER_S),
    'a second past the new cycle’s own opening hold, a second of travel',
  ).toBeLessThanOrEqual(FRAME_SLACK_PX);
});

test('draws the Closed card’s icon and label centred — the one deliberate exception to reading left', async ({
  page,
}) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(ISLANDS_OF_ADVENTURE, { rides: [ride('Test', 'Closed')] })]),
  }));
  await render(page, placed([ISLANDS_OF_ADVENTURE]));

  const closed = page.locator('[data-pwt-closed]');
  await expect(closed.locator('[data-pwt-closed-label]')).toHaveText('Closed');
  await expect(closed.locator('.closed-icon')).toBeVisible();

  const style = await closed.evaluate((el) => ({
    alignItems: getComputedStyle(el).alignItems,
    textAlign: getComputedStyle(el).textAlign,
  }));
  // A guard against a future left-align sweep wrongly straightening this: the Closed content is
  // explicitly opted back into centring, not left to inherit the module's own left default.
  expect(style.alignItems, 'the Closed content stays centred, not the module’s own left default').toBe('center');
  expect(style.textAlign, 'the Closed content’s own text-align stays centred too').toBe('center');
});

test('draws the More Waits divider with no heading text', async ({ page }) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT]));

  const more = page.locator(MORE_WAITS);
  await expect(more.locator('h1, h2, h3, h4, h5, h6')).toHaveCount(0);
  await expect(more, 'no heading text sits alongside the divider and the tour rows').not.toContainText('More waits');
});

test('draws a park’s hours to the right of its name, both on the header’s own single row, never wrapping', async ({
  page,
}) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(MAGIC_KINGDOM, {
        name: 'Magic Kingdom',
        hours: { open: '2026-01-01T09:00:00Z', close: '2026-01-01T21:00:00Z' },
      }),
    ]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  const name = page.locator('.name').first();
  const hours = page.locator('.hours').first();
  const [nameBox, hoursBox] = await Promise.all([name.boundingBox(), hours.boundingBox()]);
  if (!nameBox || !hoursBox) {
    throw new Error('expected both the name and the hours to render');
  }

  expect(nameBox.x + nameBox.width, 'the hours start at or after the name’s own right edge').toBeLessThanOrEqual(
    hoursBox.x + 1,
  );

  // "One row" (`.header`'s own `align-items: baseline`) read as a real measurement: the two
  // elements' vertical centres land close together rather than the hours sitting a whole line away
  // — which `justify-content: space-between` alone (checked above) would not rule out, since that
  // only anchors the two horizontally.
  const verticalGap = Math.abs(nameBox.y + nameBox.height / 2 - (hoursBox.y + hoursBox.height / 2));
  expect(verticalGap, 'the name and the hours sit on the header’s one row').toBeLessThan(
    Math.min(nameBox.height, hoursBox.height),
  );

  const whiteSpace = await hours.evaluate((el) => getComputedStyle(el).whiteSpace);
  expect(whiteSpace, 'the hours never wrap').toBe('nowrap');
});

test('draws a park’s name with no icon at all, for a name the module has no matching glyph for', async ({ page }) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(UNKNOWN_PARK)]),
  }));
  await render(page, placed([UNKNOWN_PARK]));

  const header = page.locator(HEADER);
  await expect(header, 'the park’s own name is drawn').toContainText(UNKNOWN_PARK);
  await expect(
    header.locator('[data-pwt-icon]'),
    'no icon element renders for a name the module has no glyph for — the sanctioned fallback is name-only',
  ).toHaveCount(0);
});

test('keeps a long park name on one line, never wrapping the header', async ({ page }) => {
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { name: 'A Very Extraordinarily Long Theme Park Name That Keeps Right On Going' })]),
  }));
  await render(page, placed([EPCOT]));

  const name = page.locator('.name').first();
  const whiteSpace = await name.evaluate((el) => getComputedStyle(el).whiteSpace);
  expect(whiteSpace, 'the park name is set to never wrap').toBe('nowrap');

  // A wrapped text node renders as more than one client rect, one per visual line — a single-line
  // nowrap name renders as exactly one, whatever its own length.
  const rectCount = await name.evaluate((el) => el.getClientRects().length);
  expect(rectCount, 'the name renders as a single line, not wrapped onto a second').toBe(1);
});

test('draws the park’s own icon beside its name in the header, for a park the module has a glyph for', async ({
  page,
}) => {
  // The fixture supplies no id, so this proves the icon set is keyed on the
  // park's own pretty name.
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM)]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  const header = page.locator(HEADER);
  await expect(header, 'the park’s own name is drawn').toContainText('Magic Kingdom');
  const icon = header.locator('[data-pwt-icon]');
  await expect(icon, 'the park’s own icon is drawn').toBeVisible();
  await expect(icon.locator('svg'), 'the icon carries real glyph content, not an empty mark').toHaveCount(1);
});

test('stands down to nothing while the backend is unreachable, and stops asking it', async ({ page }) => {
  await holdHostClock(page, HOST_TIME);

  const whileServing = await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM)]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));
  await expect(page.locator(MODULE)).toBeVisible();
  const askedOnce = whileServing.urls.length;
  expect(askedOnce, 'it asked while the backend was serving').toBeGreaterThan(0);

  const whileGone = await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM)]),
  }));
  await render(page, placed([MAGIC_KINGDOM]), 'frame', { healthz: 'abort' });

  await expect(page.locator('[data-backend-unreachable]')).toBeVisible();
  await expect(page.locator(`[data-region="middle_center"] ${MODULE}`)).toHaveCount(0);
  await expect(page.locator(MODULE_UNAVAILABLE)).toHaveCount(0);
  await expect(page.locator(LOADING)).toHaveCount(0);

  const askedBeforeTheOutageWasKnown = whileGone.urls.length;
  await advanceHostClock(page, READ_INTERVAL_MS * 2);
  expect(whileGone.urls.length, 'it asked nothing further once the backend was gone').toBe(
    askedBeforeTheOutageWasKnown,
  );
});

test('is drawn again once the backend answers, the outage having left the page live', async ({
  page,
}) => {
  // The tier's own recovery case (../../../tests/render/backend-unreachable.spec.ts) reads a stub
  // module, which holds no element of its own across the transition. This one reads the real module,
  // because what the outage tears down here is the measured grid: the width effect outlives the
  // `{#if reachable}` block that owns the element, and Svelte writes `null` back through `bind:this`
  // as that block goes. An effect that throws on it takes the whole page's rendering with it — the
  // display then holds the outage report with the backend answering behind it, for as long as it
  // runs, while every timer on the page goes on firing into a screen nothing reaches.
  //
  // Staged over one page rather than a load that is already unreachable: a page that never drew the
  // grid never tears one down, which is the tier's blind spot rather than a second case.
  const thrown: string[] = [];
  page.on('pageerror', (error) => thrown.push(String(error)));

  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM, { rides: [ride('Space Mountain', 45)] })]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));
  await expect(page.locator(CARD)).toHaveCount(1);

  await serveLiveness(page, 'abort');
  await expect(page.locator('[data-backend-unreachable]')).toBeVisible({
    timeout: 2 * LIVENESS_INTERVAL_MS,
  });
  await expect(page.locator(MODULE)).toHaveCount(0);

  // Stood down rather than faulted, read while the outage is up. The defect this case exists for
  // throws during exactly this teardown, and once a boundary is catching it the throw is no longer
  // visible at the page at all — the marker is, and only for as long as the fault lasts, so it is read
  // here rather than only after the recovery has cleared it.
  await expect(page.locator(MODULE_FAULTED)).toHaveCount(0);

  await serveLiveness(page, 'ok');

  // Two intervals, the ask that reaches the restored backend being the next one after the answer
  // changes. The card, not the module box alone: a page still rendering draws the grid it measured.
  await expect(page.locator(CARD)).toHaveCount(1, { timeout: 2 * LIVENESS_INTERVAL_MS });
  await expect(page.locator('[data-backend-unreachable]')).toHaveCount(0);

  // What it drew, read on the far side of the transition: the park's own header and its ride, the
  // things the measured grid holds. The count above is a module that came back; these are a module
  // that came back with its content, which is what the effect that threw was measuring.
  await expect(page.locator(`${CARD} ${HEADER}`)).toHaveCount(1);
  await expect(page.locator(`${CARD} ${RIDE_NAME_COLUMN}`)).toHaveText(['Space Mountain']);

  // Read last, and separately: the recovery above is the symptom, and this is the cause. Both spellings
  // of it, because the fix for this defect is a boundary at the module's mount point
  // (../../../tests/render/fault-containment.spec.ts), and a boundary is exactly what stops a throw
  // reaching the page as an uncaught error — so once one is there, the absence below is satisfied
  // whether this module threw or not, and the marker is what tells the two apart. A module that
  // recovers while having thrown is one that was lucky about ordering.
  await expect(page.locator(MODULE_FAULTED)).toHaveCount(0);
  expect(thrown, 'the outage raised no uncaught error').toEqual([]);
});

test('stops the marquee’s frame loop when the placement is torn down', async ({ page }) => {
  // The placement's marquee clock runs one `requestAnimationFrame` loop for as long as any column is
  // registered with it (marquee-clock.ts). Nothing on a torn-down page can drop a column left behind
  // in it — the row that would is gone — so a teardown that does not unregister its own columns
  // leaves that loop running for the life of the document: a frame callback every frame, forever, on
  // a display that runs for weeks (SRS021<!-- Frontend runs on a Pi Zero-class browser host -->).
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride(OVERFLOWING_RIDE_NAME, 40)] })]),
  }));
  await render(page, placed([EPCOT], { rotationIntervalSeconds: 60 }));

  const column = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`).first();
  await expect(column).toHaveText(OVERFLOWING_RIDE_NAME);
  await page.clock.runFor(MEASURE_FRAME_MS);

  // The precondition, read rather than assumed: this column is registered, so there is a running
  // loop for the teardown to leave behind. Torn down before the measurement fires, nothing would be
  // registered and the case would pass whether or not a teardown drops anything.
  await expect(
    column.locator('.ride-name-text'),
    'the column is registered with the clock, so its loop is running',
  ).toHaveClass(/marquee/);

  // Counted from here, so what is measured is the frames the page asks for *after* it is torn down.
  await page.evaluate(() => {
    const held = window as unknown as {
      __frames: number;
      requestAnimationFrame: typeof requestAnimationFrame;
    };
    held.__frames = 0;
    const asking = held.requestAnimationFrame.bind(window);
    held.requestAnimationFrame = (callback) => {
      held.__frames += 1;
      return asking(callback);
    };
  });

  // The page's own teardown path, as `tests/render/unmount.spec.ts` drives it.
  await page.evaluate("import('/src/main.ts').then((main) => main.unmount(main.default))");
  await expect(page.locator(CARD), 'the placement is gone').toHaveCount(0);

  await page.clock.runFor(2000);

  expect(
    await page.evaluate(() => (window as unknown as { __frames: number }).__frames),
    'a torn-down placement asks for no further animation frames — a loop left running asks for one per frame',
  ).toBe(0);
});
