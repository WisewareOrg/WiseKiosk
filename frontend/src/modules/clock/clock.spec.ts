import type { Page } from '@playwright/test';

import { LIVENESS_INTERVAL_MS } from '../../lib/liveness';
import {
  advanceHostClock,
  asksBeyondTheShell,
  channelsBeyondTheTier,
  expect,
  holdHostClock,
  render,
  serveLiveness,
  test,
  watchTraffic,
  type Fixture,
} from '../../../tests/render/harness';

/** Locale and zone pinned so the host arranges the reading the same on every machine. */
test.use({ locale: 'en-US', timezoneId: 'UTC' });

/** An afternoon, so the two hour forms name the hour differently rather than alike. */
const HOST_TIME = new Date('2026-08-30T15:04:05Z');

const HOST_DATE_PARTS = ['August', '30', '2026'];

const SECOND_TIME = new Date('2027-01-15T09:20:00Z');
const SECOND_DATE_PARTS = ['January', '15', '2027'];

const REGION = 'middle_center';

function placed(options: Record<string, unknown>): Fixture {
  return { modules: [{ region: REGION, module: 'clock', options }] };
}

const TIME = '[data-clock-time]';
const DATE = '[data-clock-date]';
// Seconds and meridiem are siblings of `[data-clock-time]`, inside this wrapper.
const CLOCK = '[data-clock]';

test('TST051: takes the time off the host clock, asking nothing for it', async ({
  page,
  baseURL,
}) => {
  const traffic = watchTraffic(page);
  await holdHostClock(page, HOST_TIME);
  await render(page, placed({}));

  // Schema defaults are twelve-hour, so 15:04:05 reads 03:04:05.
  await expect(page.locator(CLOCK)).toContainText(/03:04\D*05/);
  await page.clock.runFor(3000);
  await expect(page.locator(CLOCK)).toContainText(/03:04\D*08/);

  const asked = traffic.requests.map((request) => new URL(request.url).pathname);
  expect(asked, 'the watcher saw the shell ask for its configuration').toContain('/config.json');

  expect(asksBeyondTheShell(traffic)).toEqual([]);
  expect(channelsBeyondTheTier(traffic, baseURL)).toEqual([]);
});

test('TST052: keeps the displayed time moving as the host clock moves', async ({ page }) => {
  await holdHostClock(page, HOST_TIME);
  await render(page, placed({}));
  const time = page.locator(CLOCK);

  await expect(time).toContainText(/03:04\D*05/);

  await page.clock.runFor(5000);
  await expect(time).toContainText(/03:04\D*10/);

  await page.clock.runFor(65_000);
  await expect(time).toContainText(/03:05\D*15/);
});

test('TST053: presents the hour in the form its configuration selects', async ({ page }) => {
  await holdHostClock(page, HOST_TIME);

  await render(page, placed({ twenty_four_hour: true }));
  const asTwentyFour = (await page.locator(TIME).innerText()).trim();
  const asTwentyFourClock = (await page.locator(CLOCK).innerText()).trim();

  await render(page, placed({ twenty_four_hour: false }));
  const asTwelve = (await page.locator(TIME).innerText()).trim();
  const asTwelveClock = (await page.locator(CLOCK).innerText()).trim();

  // At 15:04:05 only the hour reads 15, and only the hour reads 3.
  expect(asTwentyFour).toContain('15');
  expect(asTwentyFourClock).not.toMatch(/[ap]\.?m\.?/i);
  expect(asTwelve).toMatch(/(^|\D)0?3(\D|$)/);
  expect(asTwelveClock).toMatch(/[ap]\.?m\.?/i);
  expect(asTwelve).not.toContain('15');

  expect(asTwelve).not.toBe(asTwentyFour);
});

test('TST054: shows the date when its configuration asks and omits it when it does not', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);

  await render(page, placed({ show_date: true }));
  const shown = (await page.locator(DATE).innerText()).trim();

  for (const part of HOST_DATE_PARTS) {
    expect(shown, `the date carries ${part}`).toContain(part);
  }

  await page.clock.setSystemTime(SECOND_TIME);
  await page.clock.runFor(1000);
  const later = (await page.locator(DATE).innerText()).trim();
  for (const part of SECOND_DATE_PARTS) {
    expect(later, `the date follows the host to ${part}`).toContain(part);
  }
  expect(later, 'the first date is gone once the host has moved on').not.toContain('August');

  await render(page, placed({ show_date: false }));

  await expect(page.locator(`[data-region="${REGION}"] ${TIME}`)).toBeVisible();
  await expect(page.locator(`[data-region="${REGION}"] ${DATE}`)).toHaveCount(0);
});

test('TST063: shows seconds when its configuration asks and omits them when it does not', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);

  await render(page, placed({ show_seconds: true }));
  const withSeconds = (await page.locator(CLOCK).innerText()).trim();

  await render(page, placed({ show_seconds: false }));
  const without = (await page.locator(`[data-region="${REGION}"] ${CLOCK}`).innerText()).trim();

  expect(withSeconds).toContain('05');

  expect(without).not.toContain('05');
  expect(without).toContain('04');
});

test('renders no annotations sibling at all when neither seconds nor a meridiem is shown', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);

  await render(page, placed({ show_seconds: false, twenty_four_hour: true }));

  await expect(page.locator(`[data-region="${REGION}"] .annotations`)).toHaveCount(0);
});

test('TST055: goes on showing an advancing time while the backend is unreachable', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);

  const time = page.locator(`[data-region="${REGION}"] ${CLOCK}`);
  await render(page, {
    modules: [
      { region: REGION, module: 'clock', options: {} },
      { region: 'top_left', module: 'throws' },
    ],
  });
  await expect(time).toContainText(/03:04\D*05/);
  await expect(page.locator('[data-backend-unreachable]')).toHaveCount(0);

  await serveLiveness(page, 'abort');
  await advanceHostClock(page, 2 * LIVENESS_INTERVAL_MS);

  await expect(page.locator('[data-backend-unreachable]')).toBeVisible();
  await expect(time).toContainText(/03:04\D*15/);

  await advanceHostClock(page, 65_000);
  await expect(time).toContainText(/03:05\D*20/);

  await expect(page.locator(`[data-region="${REGION}"] [data-clock]`)).toHaveCount(1);
});

/** Lines per element via `Range.getClientRects()`. Two date lines is composition (./README.md § The
    reading — time and date as peers); no TST cites it. */
async function lineCounts(page: Page, selector: string): Promise<number[]> {
  return page.$$eval(selector, (elements) =>
    elements.map((element) => {
      const range = document.createRange();
      range.selectNodeContents(element);
      return range.getClientRects().length;
    }),
  );
}

test('the date holds to two lines even in the narrowest region, not a third from a wrapped one', async ({
  page,
}) => {
  await holdHostClock(page, HOST_TIME);
  // The narrowest region the frame lays out (../../lib/regions.ts).
  await render(page, {
    modules: [{ region: 'top_left', module: 'clock', options: { show_date: true } }],
  });

  const lines = await lineCounts(page, `${DATE} p`);
  expect(lines, 'the weekday and the day/month/year line each render as one line, not wrapped').toEqual([
    1, 1,
  ]);
});
