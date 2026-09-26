<script lang="ts">
  import { untrack } from 'svelte';

  import type { ParkWaitTimesOptions } from '../../config/types';
  import type { ParkWaitTimesPayload } from '../../lib/boundary/client';
  import type { CommonProps } from '../../lib/modules';
  import type { Payload } from '../../lib/payload';

  import { startMarqueeCycle } from './marquee-clock';
  import { iconFor, uniformCardWidth } from './park_wait_times';
  import ParkCard from './ParkCard.svelte';

  /** docs/contracts/module-contract.md § The six parts (part 1); § An unavailable module and an
      unreachable backend are different states. */
  const { reachable, config, payload }: CommonProps = $props();

  const pwtConfig = $derived(config as ParkWaitTimesOptions);
  const pwtPayload = $derived(payload as Payload<ParkWaitTimesPayload>);

  /** Schema default filled by ajv's `useDefaults` (vite-plugin-config-validator.ts). */
  const rotationSeconds = $derived(pwtConfig.rotation_interval_seconds as number);

  /** SRS072<!-- The park-wait-times module advances every card's tour together --> */
  let tick = $state(0);
  $effect(() => {
    startMarqueeCycle();
    const toggle = setInterval(() => {
      tick++;
      startMarqueeCycle();
    }, rotationSeconds * 1000);
    return () => clearInterval(toggle);
  });

  /** Read once: fixed at config load. */
  const gridColumns = untrack(() => pwtConfig.columns);
  const gridRows = untrack(() => pwtConfig.rows);

  function gridShape(node: HTMLElement, shape: { columns: number; rows: number }): void {
    node.style.setProperty('--pwt-columns', String(shape.columns));
    node.style.setProperty('--pwt-rows', String(shape.rows));
  }

  /** `null` as well: `bind:this` writes it as the `{#if reachable}` block is destroyed. */
  let gridEl: HTMLElement | null | undefined = $state();

  /** The widest header, measured off the rendered boxes (`uniformCardWidth`, park_wait_times.ts;
      ./README.md § The grid — constant card, configured shape). */
  $effect(() => {
    void pwtPayload;
    const grid = gridEl;
    if (grid === undefined || grid === null) return;
    // `.hours` is drawn only where the park has hours.
    const measures = Array.from(grid.querySelectorAll<HTMLElement>('[data-pwt-card]')).map((card) => {
      const identity = card.querySelector('.identity') as HTMLElement;
      const header = card.querySelector('[data-pwt-header]') as HTMLElement;
      const hours = card.querySelector('[data-pwt-hours]');
      const cardStyle = getComputedStyle(card);
      return {
        identityWidth: identity.getBoundingClientRect().width,
        hoursWidth: hours === null ? null : hours.getBoundingClientRect().width,
        gap: parseFloat(getComputedStyle(header).columnGap),
        chrome:
          parseFloat(cardStyle.paddingLeft) +
          parseFloat(cardStyle.paddingRight) +
          parseFloat(cardStyle.borderLeftWidth) +
          parseFloat(cardStyle.borderRightWidth),
      };
    });
    grid.style.setProperty('--pwt-card-width', `${uniformCardWidth(measures)}px`);
  });
</script>

{#if reachable}
  <div class="park-wait-times" data-park-wait-times>
    {#if pwtPayload.state === 'loading'}
      <p class="waiting" data-module-loading>Reading wait times…</p>
    {:else if pwtPayload.state === 'unavailable'}
      <p class="waiting" data-module-unavailable>{pwtPayload.failure.message}</p>
    {:else}
      <ol
        class="grid"
        data-pwt-grid
        bind:this={gridEl}
        use:gridShape={{ columns: gridColumns, rows: gridRows }}
      >
        {#each pwtPayload.data.parks as park, index (index)}
          <ParkCard {park} icon={iconFor(park.name)} {tick} />
        {/each}
      </ol>
    {/if}
  </div>
{/if}

<style>
  .park-wait-times {
    min-width: 0;
    /* `.wait` and the Closed state opt back to right and centre. */
    text-align: left;
  }

  .waiting {
    margin: 0;
    font-size: var(--type-section-header);
    font-weight: var(--type-section-header-weight);
  }

  .grid {
    display: grid;
    /* Falls back to content width until `--pwt-card-width` is measured. */
    grid-template-columns: repeat(var(--pwt-columns), var(--pwt-card-width, max-content));
    grid-template-rows: repeat(var(--pwt-rows), auto);
    gap: var(--space-lg);
    margin: 0;
    padding: 0;
    list-style: none;
    /* Each card at its natural height, not stretched to the row. */
    align-items: start;
  }
</style>
