import type { Page } from '@playwright/test';

import { LIVENESS_INTERVAL_MS } from '../../src/lib/liveness';
import {
  advanceHostClock,
  expect,
  holdHostClock,
  render,
  serveLiveness,
  test,
  type Fixture,
} from './harness';

/**
 * SRS069<!-- A module that stops drawing says so in its own place --> and
 * SRS070<!-- The display comes back on its own when the backend does -->, read against
 * `stubs/Throws.svelte`. Locale and zone pinned for the reason ../../src/modules/clock/clock.spec.ts
 * states.
 */
test.use({ locale: 'en-US', timezoneId: 'UTC' });

const HOST_TIME = new Date('2026-08-30T15:04:05Z');
/** Once the outage is detected, two liveness intervals in. */
const AT_OUTAGE = /03:04\D*15/;
/** A minute on: a reading a stopped page cannot produce. */
const PAST_THE_MINUTE = /03:05\D*20/;
/** Two liveness intervals after the backend answers again. */
const AT_RECOVERY = /03:05\D*30/;

const FAULTING_REGION = 'top_left';
const SURVIVING_REGION = 'middle_center';

/** A sibling sharing the faulting region, and a clock elsewhere whose reading moves on its own. */
const FIXTURE: Fixture = {
  modules: [
    { region: FAULTING_REGION, module: 'throws' },
    { region: FAULTING_REGION, module: 'fits' },
    { region: SURVIVING_REGION, module: 'clock', options: {} },
  ],
};

/** The framework's marker (ModuleHost.svelte), not a module's own `[data-module-unavailable]`. */
const FAULTED = '[data-module-faulted]';

const THROWS = '[data-stub="throws"]';
const SIBLING = '[data-stub="fits"]';
const CLOCK = '[data-clock]';

const OUTAGE = '[data-backend-unreachable]';

/** A live page first: the throw comes from the outage's teardown, which a page loaded unreachable
    never does. */
async function raiseTheFault(page: Page): Promise<void> {
  await holdHostClock(page, HOST_TIME);
  await render(page, FIXTURE);

  await expect(page.locator(THROWS)).toBeVisible();
  await expect(page.locator(SIBLING)).toBeVisible();
  await expect(page.locator(CLOCK)).toBeVisible();
  await expect(page.locator(FAULTED)).toHaveCount(0);
  await expect(page.locator(OUTAGE)).toHaveCount(0);

  await serveLiveness(page, 'abort');
  await advanceHostClock(page, 2 * LIVENESS_INTERVAL_MS);
}

test('says so where a module that threw stood, and nowhere else', async ({ page }) => {
  const escaped: string[] = [];
  page.on('pageerror', (error) => escaped.push(String(error)));

  await raiseTheFault(page);

  // The whole-page count catches a marker drawn over every placement.
  await expect(page.locator(`[data-region="${FAULTING_REGION}"] ${FAULTED}`)).toBeVisible();
  await expect(page.locator(FAULTED)).toHaveCount(1);

  expect((await page.locator(FAULTED).innerText()).trim()).not.toBe('');

  await expect(page.locator(THROWS)).toHaveCount(0);

  // Read after the marker: a stub that never threw also raises nothing.
  expect(escaped, 'the fault was contained rather than raised at the page').toEqual([]);
});

test('leaves every other module rendering, including one sharing the faulting region', async ({
  page,
}) => {
  await raiseTheFault(page);

  await expect(page.locator(`[data-region="${FAULTING_REGION}"] ${SIBLING}`)).toBeVisible();

  const clock = page.locator(`[data-region="${SURVIVING_REGION}"] ${CLOCK}`);
  await expect(clock).toContainText(AT_OUTAGE);
  await advanceHostClock(page, 65_000);

  await expect(clock).toContainText(PAST_THE_MINUTE);

  await expect(page.locator(`[data-region="${SURVIVING_REGION}"] ${FAULTED}`)).toHaveCount(0);
});

test('goes on reporting the outage it is in, and stops reporting it on recovery', async ({
  page,
}) => {
  await raiseTheFault(page);

  await expect(page.locator(OUTAGE)).toBeVisible();

  await serveLiveness(page, 'ok');
  await advanceHostClock(page, 2 * LIVENESS_INTERVAL_MS);

  await expect(page.locator(OUTAGE)).toHaveCount(0);
});

test('draws the module again once the fault clears, and takes the marker away with it', async ({
  page,
}) => {
  await raiseTheFault(page);

  await expect(page.locator(FAULTED)).toHaveCount(1);
  await expect(page.locator(THROWS)).toHaveCount(0);

  await advanceHostClock(page, 65_000);

  await serveLiveness(page, 'ok');
  await advanceHostClock(page, 2 * LIVENESS_INTERVAL_MS);

  await expect(page.locator(THROWS)).toBeVisible();
  await expect(page.locator(FAULTED)).toHaveCount(0);

  await expect(page.locator(`[data-region="${SURVIVING_REGION}"] ${CLOCK}`)).toContainText(
    AT_RECOVERY,
  );
});
