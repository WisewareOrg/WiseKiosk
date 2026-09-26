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

/** Every case answers the module's route from the test, never a real source. */

const READ_INTERVAL_MS = 5 * 60 * 1000;

/** Held back from the interval, so a read falls inside it. */
const ALMOST = 10_000;

const HOST_TIME = new Date('2026-08-31T14:00:00Z');

/** Driven time for the marquee's measurement frame, which `page.clock` holds queued. */
const MEASURE_FRAME_MS = 100;

/** ~16ms frame pacing plus whole-pixel `scrollLeft` at `MARQUEE_PX_PER_S`. */
const FRAME_SLACK_PX = 2;

/** Available, no rides; keyed by name, since the response carries no id. */
function onePark(name: string, fields: Partial<ParkWaitTimesPark> = {}): ParkWaitTimesPark {
  return { name, available: true, ...fields };
}

function parksPayload(parks: ParkWaitTimesPark[]): ParkWaitTimesPayload {
  return { parks };
}

/** A number is an Operating wait in minutes; a state word sets that state with `waitMinutes`
    null. */
function ride(name: string, wait: number | ParkWaitTimesState): ParkWaitTimesRide {
  return typeof wait === 'number'
    ? { name, state: ParkWaitTimesState.Operating, waitMinutes: wait }
    : { name, state: wait, waitMinutes: null };
}

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
/** The framework's contained-fault marker (ModuleHost.svelte). */
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
/** The scroll container, not `.ride-name-text` inside it. */
const RIDE_NAME_COLUMN = '[data-pwt-ride-name]';

/** Keyed by name, the one identity the wire carries (boundary/openapi.yaml's
    ParkWaitTimesPark.name). */
const MAGIC_KINGDOM = 'Magic Kingdom';
const EPCOT = 'Epcot';
const HOLLYWOOD_STUDIOS = 'Hollywood Studios';
const ANIMAL_KINGDOM = 'Animal Kingdom';
const UNIVERSAL_STUDIOS = 'Universal Studios';
const ISLANDS_OF_ADVENTURE = 'Islands of Adventure';

/** No icon for this name: drawn name-only. */
const UNKNOWN_PARK = 'A Park With No Icon';

/** Wider than the ride-name column at every render viewport. */
const OVERFLOWING_RIDE_NAME =
  'Guardians of the Galaxy: Cosmic Rewind — The Complete Extended Experience Edition';

function cardNamed(page: import('@playwright/test').Page, name: string) {
  return page.locator(CARD).filter({ has: page.locator('.name', { hasText: name }) });
}

async function rideNamesIn(card: ReturnType<import('@playwright/test').Page['locator']>, selector: string): Promise<string[]> {
  return card.locator(selector).locator('.ride-name').allTextContents();
}

test('TST071: reports on the parks its configuration names, in the region it names, and moves with a second configuration', async ({
  page,
}) => {
  // The answer is a function of the ask, so the rosters are told apart by request, not by order.
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

  const here = page.locator(`[data-region="middle_center"] ${CARD}`);
  const there = page.locator(`[data-region="lower_third"] ${CARD}`);
  await expect(here.locator('.name')).toHaveText(MAGIC_KINGDOM);
  await expect(there.locator('.name')).toHaveText(EPCOT);

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

  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(MAGIC_KINGDOM, { rides: [ride('Space Mountain', 45)] }),
      onePark(MAGIC_KINGDOM, { rides: [ride('Big Thunder Mountain', 20)] }),
    ]),
  }));
  await render(page, placed([MAGIC_KINGDOM, MAGIC_KINGDOM], { columns: 2, rows: 1 }));

  // Read what was drawn: a contained throw raises no page error
  // (SRS069<!-- A module that stops drawing says so in its own place -->).
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

  await expect(page.locator(LEADERBOARD_ROW)).toHaveCount(2);
  await expect(page.locator(`${LEADERBOARD_ROW} ${RIDE_NAME_COLUMN}`)).toHaveText([
    'Test Track',
    'Test Track',
  ]);
  await expect(page.locator(`${LEADERBOARD_ROW} ${WAIT}`)).toHaveText(['40', '15']);
  await expect(page.locator(MODULE_FAULTED)).toHaveCount(0);

  expect(pageErrors, 'the page raised nothing while rendering the duplicate ride name').toHaveLength(0);
});

/** Three held and four touring, so the ranking
    (SRS058<!-- The park-wait-times module keeps each park's longest current waits in view -->) and
    the rotation
    (SRS059<!-- The park-wait-times module tours the remaining rides on an interval its
    configuration sets -->) are each unambiguous. */
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
  // Held: Test Track, Soarin, Spaceship Earth; four remaining, two pages.
  await expect(card.locator(TOUR_ROW)).toHaveCount(2);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  await advanceHostClock(page, 8 * 1000 - 500);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  await advanceHostClock(page, 500);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['The Seas', 'Living with the Land']);

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

  await advanceHostClock(page, 6 * 1000);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  await advanceHostClock(page, 20 * 1000 - 6 * 1000);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['The Seas', 'Living with the Land']);
});

test('TST076: a placement that omits its own rotation interval tours on the schema’s default of eight seconds', async ({
  page,
}) => {
  // `rotation_interval_seconds` is absent, so ajv fills the schema default (8).
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: rankedRoster() })]),
  }));
  await render(page, placed([EPCOT]));

  const card = page.locator(CARD);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

  await advanceHostClock(page, 8 * 1000 - 500);
  expect(await rideNamesIn(card, TOUR_ROW)).toEqual(['Mission: Space', 'Imagination!']);

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

/** `count` distinct rides, waits descending: three held, the rest touring two a page. */
function rosterOf(park: string, count: number): ParkWaitTimesRide[] {
  return Array.from({ length: count }, (_unused, index) => ride(`${park} ride ${index}`, count - index));
}

async function filledSegment(card: ReturnType<import('@playwright/test').Page['locator']>): Promise<number> {
  return card
    .locator(FOOTER_SEGMENT)
    .evaluateAll((segments) => segments.findIndex((segment) => segment.classList.contains('filled')));
}

test('advances two cards of different page counts on the same tick — neither moves early, and the shorter wraps home as the longer takes its last page', async ({
  page,
}) => {
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
  await expect(shorter.locator(FOOTER_SEGMENT), 'four remaining rides, two pages').toHaveCount(2);
  await expect(longer.locator(FOOTER_SEGMENT), 'six remaining rides, three pages').toHaveCount(3);
  expect([await filledSegment(shorter), await filledSegment(longer)]).toEqual([0, 0]);

  await advanceHostClock(page, ROTATION_S * 1000 - 500);
  expect([await filledSegment(shorter), await filledSegment(longer)]).toEqual([0, 0]);
  await advanceHostClock(page, 500);
  expect(
    [await filledSegment(shorter), await filledSegment(longer)],
    'both cards advanced across the one tick',
  ).toEqual([1, 1]);

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

  await expect(minutes).not.toHaveClass(/state/);
  await expect(state).toHaveClass(/state/);

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

  // A mark a reload would clear.
  await page.evaluate(() => {
    (window as unknown as { standing?: boolean }).standing = true;
  });

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
  const traffic = watchTraffic(page);
  const served = await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(MAGIC_KINGDOM, { rides: [ride('The Barnstormer', 10)] })]),
  }));
  await render(page, placed([MAGIC_KINGDOM]));

  await expect(page.locator(WAIT).first()).toHaveText('10');
  expect(served.urls.length, 'the module asked its own route').toBeGreaterThan(0);

  expect([...new Set(asksBeyondTheShell(traffic))]).toEqual(['/api/park-wait-times']);
  expect(channelsBeyondTheTier(traffic, baseURL)).toEqual([]);
});

test('holds a park’s place in the grid and shows why, when that park’s own reading could not be produced', async ({
  page,
}) => {
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

  await expect(failing.locator('[data-pwt-header]')).toBeVisible();
  await expect(failing.locator('[data-pwt-header]')).toContainText(MAGIC_KINGDOM);
});

test('renders each unavailable park’s own message verbatim, transient or permanent — the card never branches on the wording', async ({
  page,
}) => {
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
  // The route is taken and never fulfilled: the ask in flight.
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

  // Five remaining: three pages (2, 2, 1).
  const footer0 = await footerY();
  await expect(card.locator(TOUR_ROW)).toHaveCount(2);

  await advanceHostClock(page, 8 * 1000);
  await expect(card.locator(TOUR_ROW)).toHaveCount(2);
  expect(await footerY()).toBe(footer0);

  await advanceHostClock(page, 8 * 1000);
  await expect(card.locator(TOUR_ROW)).toHaveCount(1);
  await expect(card.locator('[data-pwt-tour-placeholder]')).toHaveCount(1);
  expect(await footerY()).toBe(footer0);
});

test('anchors the footer to the card’s own bottom edge, with nothing beneath it', async ({ page }) => {
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

  expect(
    Math.abs(heights[ISLANDS_OF_ADVENTURE] - heights[MAGIC_KINGDOM]),
    'the Closed card matches the full card’s height',
  ).toBeLessThan(1);

  expect(
    heights[MAGIC_KINGDOM] - heights[EPCOT],
    'an open card with less to show is left shorter, not forced to match',
  ).toBeGreaterThan(10);

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
  // A long park name makes the column wider than the shorter ride name, so a centred name would
  // show.
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

  // The true content span, read off the DOM rather than ParkWaitTimes.svelte's formula, so a
  // drifted probe is caught.
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

  const lefts = await waits.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().x));
  expect(new Set(lefts).size, 'the wait column’s own left edge holds across 1, 2 and 3 digits').toBe(1);
});

test('reserves one constant wait column across a numeric wait and the longest not-operating word alike, neither wider than the other', async ({
  page,
}) => {
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
  // Instants derive from the distance this render measures; the name only has to overflow.
  const overflowingName = 'Cosmic Rewind';
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride(overflowingName, 40)] })]),
  }));
  // Longer than the cycle, so the next tick cannot land inside it.
  await render(page, placed([EPCOT], { rotationIntervalSeconds: 60 }));

  const column = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`).first();
  await expect(column, 'the full name is in the DOM, not truncated').toHaveText(overflowingName);
  await page.clock.runFor(MEASURE_FRAME_MS);

  const distance = await column.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(distance, 'the fixture’s name really does overflow its own column').toBeGreaterThan(0);
  const moveSeconds = distance / MARQUEE_PX_PER_S;

  /** Drives the clock to `seconds` into the cycle and reads `scrollLeft`; lags by at most
      `FRAME_SLACK_PX`. */
  let drivenMs = MEASURE_FRAME_MS;
  const scrolledAt = async (seconds: number): Promise<number> => {
    const target = Math.round(seconds * 1000);
    await page.clock.runFor(target - drivenMs);
    drivenMs = target;
    return column.evaluate((el) => el.scrollLeft);
  };

  expect(await scrolledAt(HOLD_HOME_S - 0.2), 'still home through the opening hold').toBe(0);

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

  expect(
    await scrolledAt(HOLD_HOME_S + moveSeconds + HOLD_END_S + 6),
    'still home six seconds on — nothing but the next tick restarts the pass',
  ).toBe(0);
});

test('leaves a ride name that already fits its own column unregistered, while an overflowing name on the same card scrolls', async ({
  page,
}) => {
  // A long park name widens the column enough for `fittingName` to fit. The overflowing row is the
  // control: its `[data-marquee]` proves the measurement frame has run.
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
  ).toHaveAttribute('data-marquee');
  await expect(
    fittingRow.locator('.ride-name-text'),
    'a name that already fits its column is not',
  ).not.toHaveAttribute('data-marquee');

  const overflowOf = (locator: typeof columns) => locator.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(await overflowOf(overflowingRow), 'the long name really does overflow its own column').toBeGreaterThan(0);
  expect(await overflowOf(fittingRow), 'the short one really does fit').toBeLessThanOrEqual(0);
});

test('re-measures the marquee after a poll refresh reorders rows in place, not just at mount', async ({
  page,
}) => {
  // Index-keyed rows persist across a reorder, so `update` must re-measure. `[data-marquee]` is the
  // probe: a column with nothing left to scroll reads `scrollLeft` 0 whether or not it is still
  // registered.
  const SHORT = 'A';
  const LONG = OVERFLOWING_RIDE_NAME;

  // The marquee's frame loop makes driving the read interval slow under `page.clock`.
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

  const columns = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`);
  await expect(columns).toHaveCount(2);
  const overflowOf = (locator: typeof columns) => locator.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(await overflowOf(columns.nth(1)), 'the long name overflows its own column').toBeGreaterThan(0);
  expect(await overflowOf(columns.nth(0)), 'the short name does not').toBeLessThanOrEqual(0);

  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride(SHORT, 5), ride(LONG, 80)] })]),
  }));
  await advanceHostClock(page, READ_INTERVAL_MS);
  await expect(row0, 'the reorder landed').toHaveText(LONG);
  await expect(row1).toHaveText(SHORT);
  // `page.clock` fakes rAF; this runs the queued measurement.
  await page.clock.runFor(1000);

  await expect
    .poll(
      () =>
        columns.evaluateAll((els) =>
          els.every((el) => {
            const overflows = el.scrollWidth > el.clientWidth;
            return el.querySelector('.ride-name-text')?.hasAttribute('data-marquee') === overflows;
          }),
        ),
      { message: 'each ride name is registered iff its own column overflows, after the reorder' },
    )
    .toBe(true);
});

test('starts every overflowing name on the placement’s one clock — two columns of different widths leave home together and share a pace, not a duration', async ({
  page,
}) => {
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

  expect(2 * MARQUEE_PX_PER_S, 'both columns are still ramping at the later sample').toBeLessThan(shorter);
  const afterOneSecond = await scrolledAt(HOLD_HOME_S + 1);
  expect(afterOneSecond[0], 'a second into the pass, the columns have left home').toBeGreaterThan(0);
  expect(new Set(afterOneSecond).size, 'and are at the one offset between them').toBe(1);
  expect(new Set(await scrolledAt(HOLD_HOME_S + 2)).size, 'two seconds in, still the one offset').toBe(1);

  const atShorterEnd = await scrolledAt(HOLD_HOME_S + shorter / MARQUEE_PX_PER_S + 0.5);
  expect(atShorterEnd[0], 'the shorter name is clamped at its own end').toBe(shorter);
  expect(atShorterEnd[1], 'the longer one is already past that offset').toBeGreaterThan(shorter);
  expect(atShorterEnd[1], 'and has not reached its own end yet').toBeLessThan(longer);
});

test('restarts every scroll on the rotation tick the cards flip on, rather than on a clock of its own', async ({
  page,
}) => {
  const ROTATION_S = 8;
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([
      onePark(EPCOT, {
        rides: [
          // Ranked first, so its row survives the flip; the rest give the tour two pages.
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
  const distance = await column.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(
    HOLD_HOME_S + distance / MARQUEE_PX_PER_S,
    'the name is still scrolling, not held at its end, when the tick lands',
  ).toBeGreaterThan(ROTATION_S);

  await driveTo(tickMs - 200);
  await expect(segments.nth(0), 'the tour has not advanced yet').toHaveClass(/filled/);
  expect(await scrollLeft(), 'the scroll is in flight when the tick lands').toBeGreaterThan(0);

  await driveTo(tickMs + 300);
  await expect(segments.nth(1), 'the tour advanced on the tick').toHaveClass(/filled/);
  expect(await scrollLeft(), 'and the scroll is home again on that one tick').toBe(0);

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

  // One client rect per rendered line.
  const rectCount = await name.evaluate((el) => el.getClientRects().length);
  expect(rectCount, 'the name renders as a single line, not wrapped onto a second').toBe(1);
});

test('draws the park’s own icon beside its name in the header, for a park the module has a glyph for', async ({
  page,
}) => {
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
  // The real module: its width effect outlives the `{#if reachable}` grid and sees `bind:this`
  // write `null` as the outage tears it down
  // (SRS070<!-- The display comes back on its own when the backend does -->).
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

  // Read during the outage: the marker exists only while the fault lasts.
  await expect(page.locator(MODULE_FAULTED)).toHaveCount(0);

  await serveLiveness(page, 'ok');

  await expect(page.locator(CARD)).toHaveCount(1, { timeout: 2 * LIVENESS_INTERVAL_MS });
  await expect(page.locator('[data-backend-unreachable]')).toHaveCount(0);

  await expect(page.locator(`${CARD} ${HEADER}`)).toHaveCount(1);
  await expect(page.locator(`${CARD} ${RIDE_NAME_COLUMN}`)).toHaveText(['Space Mountain']);

  // Both: a boundary swallows the throw, so only the marker tells a caught throw from none.
  await expect(page.locator(MODULE_FAULTED)).toHaveCount(0);
  expect(thrown, 'the outage raised no uncaught error').toEqual([]);
});

test('stops the marquee’s frame loop when the placement is torn down', async ({ page }) => {
  await holdHostClock(page, HOST_TIME);
  await serveModuleData(page, () => ({
    status: 200,
    data: parksPayload([onePark(EPCOT, { rides: [ride(OVERFLOWING_RIDE_NAME, 40)] })]),
  }));
  await render(page, placed([EPCOT], { rotationIntervalSeconds: 60 }));

  const column = page.locator(`${CARD} ${RIDE_NAME_COLUMN}`).first();
  await expect(column).toHaveText(OVERFLOWING_RIDE_NAME);
  await page.clock.runFor(MEASURE_FRAME_MS);

  await expect(
    column.locator('.ride-name-text'),
    'the column is registered with the clock, so its loop is running',
  ).toHaveAttribute('data-marquee');

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
