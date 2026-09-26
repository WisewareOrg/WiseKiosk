<script lang="ts">
  import { ParkWaitTimesState, type ParkWaitTimesPark, type ParkWaitTimesRide } from '../../lib/boundary/client';
  import {
    heldRides,
    hoursText,
    noOpenRides as ridesAreClosed,
    pageCount as pageCountOf,
    pageSlice,
    remainingRides,
    tourPadding as tourPaddingOf,
  } from './park_wait_times';
  import { registerMarquee, unregisterMarquee } from './marquee-clock';
  import type { Action } from 'svelte/action';

  /**
   * One park's card (./README.md § The card), advancing on the placement's shared `tick`
   * (SRS072<!-- The park-wait-times module advances every card's tour together -->).
   */
  const {
    park,
    icon,
    tick,
  }: { park: ParkWaitTimesPark; icon: string | undefined; tick: number } = $props();

  /** Composition, not configuration (./README.md § The card, top to bottom). */
  const HELD_COUNT = 3;
  const TOUR_SIZE = 2;

  /** A full card's rows, not this park's. */
  const SKELETON_LEADERBOARD_ROWS = Array.from({ length: HELD_COUNT }, (_unused, index) => index);
  const SKELETON_TOUR_ROWS = Array.from({ length: TOUR_SIZE }, (_unused, index) => index);

  const rides = $derived(park.rides ?? []);
  const held = $derived(heldRides(rides, HELD_COUNT));
  const noOpenRides = $derived(ridesAreClosed(rides));
  const remaining = $derived(remainingRides(rides, held));
  const pageCount = $derived(pageCountOf(remaining.length, TOUR_SIZE));

  /** Modulo `pageCount`, so a payload that shrinks the pool between ticks cannot point past its
      end. */
  const page = $derived(tick % pageCount);
  const shown = $derived(pageSlice(remaining, page, TOUR_SIZE));
  const pages = $derived(Array.from({ length: pageCount }, (_unused, index) => index));
  const tourPadding = $derived(tourPaddingOf(shown.length, TOUR_SIZE));

  /** SRS071<!-- The park-wait-times module brings the hidden part of a ride name into view -->.
      Measures and registers with marquee-clock.ts. The parameter is unread: `{#each}` keys on
      position, so `update` on `ride.name` is what re-measures after a re-sort. */
  const marquee: Action<HTMLElement, string> = (node) => {
    // Captured now: `destroy` can run with the node already detached.
    const column = node.parentElement as HTMLElement;

    // Next frame: the grid sets `--pwt-card-width` after this row mounts. A measure still queued at
    // teardown reads a detached column as zero and unregisters.
    function measure(): void {
      requestAnimationFrame(() => {
        const distance = column.scrollWidth - column.clientWidth;
        if (distance <= 0) {
          unregisterMarquee(column);
          node.removeAttribute('data-marquee');
          return;
        }
        registerMarquee(column, distance);
        node.setAttribute('data-marquee', '');
      });
    }

    measure();
    return {
      update: measure,
      destroy() {
        unregisterMarquee(column);
      },
    };
  };
</script>

<li class="card" data-pwt-card data-pwt-park={park.name}>
  <div class="header" data-pwt-header>
    <div class="identity">
      {#if icon}
        <!-- A bundled .svg inlined via Vite's `?raw`, never network or configuration input. -->
        <!-- eslint-disable-next-line svelte/no-at-html-tags -->
        <span class="icon" data-pwt-icon aria-hidden="true">{@html icon}</span>
      {/if}
      <span class="name section-label">{park.name}</span>
    </div>
    {#if park.hours}
      <span class="hours" data-pwt-hours>{hoursText(park.hours)}</span>
    {/if}
  </div>

  {#snippet rideRow(ride: ParkWaitTimesRide)}
    <span class="ride-name" data-pwt-ride-name>
      <span class="ride-name-text" use:marquee={ride.name}>{ride.name}</span>
    </span>
    {#if ride.state === ParkWaitTimesState.Operating}
      <span class="wait tabular-figures" data-pwt-wait data-pwt-wait-kind="minutes">{ride.waitMinutes}</span>
    {:else}
      <span class="wait state" data-pwt-wait data-pwt-wait-kind="state">{ride.state}</span>
    {/if}
  {/snippet}

  {#snippet fullCardSkeleton()}
    <ol class="leaderboard skeleton" aria-hidden="true">
      {#each SKELETON_LEADERBOARD_ROWS as index (index)}
        <li class="row"><span class="ride-name">&nbsp;</span></li>
      {/each}
    </ol>
    <div class="more skeleton" aria-hidden="true">
      <div class="more-divider"></div>
      <ol class="tour">
        {#each SKELETON_TOUR_ROWS as index (index)}
          <li class="row"><span class="ride-name">&nbsp;</span></li>
        {/each}
      </ol>
      <div class="footer"><span class="segment"></span></div>
    </div>
  {/snippet}

  {#if !park.available}
    <div class="full-frame">
      {@render fullCardSkeleton()}
      <p class="unavailable" data-pwt-unavailable>{park.message}</p>
    </div>
  {:else if noOpenRides}
    <div class="full-frame">
      {@render fullCardSkeleton()}
      <div class="closed" data-pwt-closed>
        {#if icon}
          <!-- eslint-disable-next-line svelte/no-at-html-tags -->
          <span class="closed-icon" aria-hidden="true">{@html icon}</span>
        {/if}
        <span class="closed-label section-label" data-pwt-closed-label>Closed</span>
      </div>
    </div>
  {:else}
    <ol class="leaderboard" data-pwt-leaderboard>
      {#each held as ride, index (index)}
        <li class="row" data-pwt-leaderboard-row>
          {@render rideRow(ride)}
        </li>
      {/each}
    </ol>

    {#if remaining.length > 0}
      <div class="more" data-pwt-more-waits>
        <div class="more-divider" data-pwt-more-divider aria-hidden="true"></div>
        <ol class="tour">
          {#each shown as ride, index (index)}
            <li class="row" data-pwt-tour-row>
              {@render rideRow(ride)}
            </li>
          {/each}
          {#each tourPadding as index (index)}
            <li class="row" data-pwt-tour-placeholder aria-hidden="true">
              <span class="ride-name">&nbsp;</span>
            </li>
          {/each}
        </ol>
        <div class="footer" data-pwt-footer>
          {#each pages as index (index)}
            <span class="segment" class:filled={index === page} data-pwt-footer-segment></span>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</li>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    padding: var(--space-md);
    border: calc(var(--divider-stroke-width) * 2) solid var(--emission-stroke);
    border-radius: var(--space-sm);
    /* `--pwt-card-width` is a border-box measure. */
    box-sizing: border-box;
  }

  .unavailable {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    margin: 0;
    font-size: var(--type-section-header);
    font-weight: var(--type-section-header-weight);
  }

  .full-frame {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  .skeleton {
    visibility: hidden;
  }

  .closed {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-sm);
    text-align: center;
  }

  .closed-icon {
    width: var(--type-annotation);
    height: var(--type-annotation);
    color: var(--emission-content);
  }

  .closed-icon :global(svg) {
    width: 100%;
    height: 100%;
  }

  .closed-label {
    font-size: var(--type-body);
    font-weight: var(--type-section-header-weight);
  }

  .header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-sm);
    padding-bottom: var(--space-sm);
    border-bottom: var(--divider-stroke-width) solid var(--emission-stroke);
  }

  .identity {
    display: flex;
    align-items: baseline;
    gap: var(--space-xs);
  }

  .icon {
    display: inline-flex;
    align-items: center;
    width: var(--type-body);
    height: var(--type-body);
    color: var(--emission-content);
  }

  .icon :global(svg) {
    width: 100%;
    height: 100%;
  }

  .name {
    font-size: var(--type-body);
    /* ./README.md § Type and spacing */
    font-weight: var(--type-section-header-weight);
    white-space: nowrap;
  }

  .hours {
    font-size: var(--type-section-header);
    /* ./README.md § Header — icon, park, hours */
    font-weight: var(--type-body-weight);
    white-space: nowrap;
  }

  .leaderboard,
  .tour {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .tour {
    gap: var(--space-sm);
  }

  .row {
    display: flex;
    align-items: baseline;
    gap: var(--space-sm);
    font-size: var(--type-section-header);
    /* ./README.md § Type and spacing */
    font-weight: var(--type-body-weight);
  }

  .ride-name {
    /* Clips the name and is the scroll container marquee-clock.ts moves; `.wait` opts back to
       `right`. */
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
  }

  .ride-name-text {
    display: inline-block;
    white-space: nowrap;
  }

  .wait {
    /* Wide enough for a caption-step "REFURB"; sized off the caption token so a figure and a state
       share one width (SRS058<!-- The park-wait-times module keeps each park's longest current waits
       in view -->, SRS061<!-- The park-wait-times module draws a wait as the time or the
       not-operating state it is handed -->). */
    flex: 0 0 auto;
    min-width: calc(var(--type-caption) * 4.75);
    text-align: right;
    font-weight: 700;
    white-space: nowrap;
  }

  /* ./README.md § The wait slot — a number, or a state */
  .wait.state {
    font-size: var(--type-caption);
    font-weight: var(--type-caption-weight);
    text-transform: uppercase;
    letter-spacing: var(--type-section-header-tracking);
  }

  .more {
    display: flex;
    flex-direction: column;
    /* ./README.md § Type and spacing */
    gap: var(--space-md);
  }

  .more-divider {
    /* ./README.md § More waits — the rotation */
    border-bottom: var(--divider-stroke-width) solid var(--emission-stroke);
  }

  .footer {
    display: flex;
    gap: var(--space-xs);
  }

  .segment {
    flex: 1;
    height: var(--divider-stroke-width);
    background: var(--emission-stroke);
  }

  .segment.filled {
    background: var(--emission-content);
  }
</style>
