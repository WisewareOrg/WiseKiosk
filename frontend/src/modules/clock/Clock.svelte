<script lang="ts">
  import { untrack } from 'svelte';

  import type { ClockOptions } from '../../config/types';
  import type { CommonProps } from '../../lib/modules';
  import { partValue } from './parts';

  // No `reachable` prop: a local module (docs/contracts/module-contract.md § An unavailable module
  // and an unreachable backend are different states).
  const { config }: CommonProps = $props();

  // Safe once config validation has run (ADR 0007 rev 2).
  const clockConfig = $derived(config as ClockOptions);

  // The validator writes schema defaults, so no key arrives absent.
  const twentyFourHour = $derived(clockConfig.twenty_four_hour);
  const showSeconds = $derived(clockConfig.show_seconds);
  const showDate = $derived(clockConfig.show_date);

  const READ_INTERVAL_MS = 1000;

  // Seconds off the reactive graph (./README.md § The reading — time and date as peers).
  // `bind:this` writes `null` on teardown.
  let secondsEl: HTMLElement | null | undefined = $state();
  let minuteDate = $state(new Date());
  let dayDate = $state(new Date());
  function pad2(n: number): string {
    return n < 10 ? '0' + n : String(n);
  }
  $effect(() => {
    const write = (): void => {
      const d = new Date();
      // eslint-disable-next-line svelte/no-dom-manipulating
      if (secondsEl) secondsEl.textContent = pad2(d.getSeconds());
      if (d.getHours() !== minuteDate.getHours() || d.getMinutes() !== minuteDate.getMinutes()) {
        minuteDate = d;
      }
      if (
        d.getDate() !== dayDate.getDate() ||
        d.getMonth() !== dayDate.getMonth() ||
        d.getFullYear() !== dayDate.getFullYear()
      ) {
        dayDate = d;
      }
    };
    // Untracked, so write's own reads and writes do not re-run this effect and rebuild the interval.
    untrack(write);
    const reading = setInterval(write, READ_INTERVAL_MS);
    return () => clearInterval(reading);
  });

  // Sliced from hour's index, not 0: some locales lead the string with the day period.
  const timeFormat = $derived(
    new Intl.DateTimeFormat(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: twentyFourHour ? 'h23' : 'h12',
    }),
  );
  const timeParts = $derived(timeFormat.formatToParts(minuteDate));
  const hoursMinutes = $derived(
    timeParts
      .slice(
        timeParts.findIndex((part) => part.type === 'hour'),
        timeParts.findIndex((part) => part.type === 'minute') + 1,
      )
      .map((part) => part.value)
      .join(''),
  );
  const meridiemText = $derived(partValue(timeParts, 'dayPeriod'));

  const weekdayFormat = new Intl.DateTimeFormat(undefined, { weekday: 'long' });
  const fullDateFormat = new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
</script>

<div class="clock" data-clock>
  <div class="time-group">
    <p class="time tabular-figures" data-clock-time>{hoursMinutes}</p>
    {#if showSeconds || !twentyFourHour}
      <div class="annotations">
        {#if showSeconds}
          <span class="seconds-slot tabular-figures">
            <span class="seconds tabular-figures" bind:this={secondsEl}></span>
          </span>
        {/if}
        {#if !twentyFourHour}
          <span class="meridiem">{meridiemText}</span>
        {/if}
      </div>
    {/if}
  </div>
  {#if showDate}
    <div class="rule"></div>
    <div class="date" data-clock-date>
      <p class="weekday section-label">{weekdayFormat.format(dayDate)}</p>
      <p class="full-date tabular-figures">{fullDateFormat.format(dayDate)}</p>
    </div>
  {/if}
</div>

<style>
  .clock {
    display: flex;
    align-items: center;
    gap: var(--space-lg);
    min-width: 0;
  }

  .time-group {
    display: flex;
    align-items: stretch;
    gap: var(--space-xs);
  }

  .time {
    margin: 0;
    font-size: var(--type-display);
    font-weight: var(--type-display-weight);
    line-height: 1;
  }

  /* ./README.md § The reading — time and date as peers */
  .annotations {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
  }

  .seconds-slot,
  .meridiem {
    margin: 0;
    font-size: var(--type-annotation);
    font-weight: var(--type-annotation-weight);
    line-height: 1;
  }

  /* Relayout boundary (./README.md § The reading — time and date as peers). */
  .seconds-slot {
    position: relative;
  }

  .seconds-slot::before {
    content: ':';
  }

  /* Sizes the slot to a two-digit tabular reading; generated content, so it is not module text. */
  .seconds-slot::after {
    content: '00';
    visibility: hidden;
  }

  /* `inset` gives size containment a definite box. */
  .seconds {
    position: absolute;
    inset: 0;
    text-align: right;
    contain: size layout;
  }

  /* A margin, not `space-between`, so a lone meridiem still sits low. */
  .meridiem {
    margin-top: auto;
  }

  /* ./README.md § Grouping, and coherence with the rest of the display */
  .rule {
    align-self: stretch;
    width: var(--divider-stroke-width);
    background: var(--emission-stroke);
  }

  .date {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  /* Never wrapped: a region too narrow is exceeded, not folded (../../lib/RegionFrame.svelte's
     `.region`). */
  .weekday,
  .full-date {
    margin: 0;
    font-size: var(--type-title);
    font-weight: var(--type-title-weight);
    line-height: 1;
    white-space: nowrap;
  }
</style>
