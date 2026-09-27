<script lang="ts">
  import type { ModuleOptions } from '../config/types';
  import { REQUEST_TIMEOUT_MS } from './liveness';
  import type { ModuleEntry } from './modules';
  import type { Payload, PayloadFailure } from './payload';

  /** One placement of one module (docs/contracts/module-contract.md § Dependency direction). */
  const {
    entry,
    config,
    reachable,
  }: { entry: ModuleEntry; config: ModuleOptions; reachable: boolean } = $props();

  const UNANSWERED = 'The reading did not come back in time.';

  /** SRS069<!-- A module that stops drawing says so in its own place --> */
  const FAULTED = 'This part of the display stopped working.';

  const Module = $derived(entry.component);

  let payload = $state<Payload<unknown>>({ state: 'loading' });

  async function read(ask: NonNullable<ModuleEntry['read']>): Promise<Payload<unknown>> {
    let answer;
    try {
      answer = await ask(config, { signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
    } catch {
      return { state: 'unavailable', failure: { message: UNANSWERED } };
    }

    if (answer.status === 200) {
      return { state: 'ok', data: answer.data };
    }

    // docs/contracts/module-contract.md § An unavailable module and an unreachable backend are
    // different states
    const failure = answer.data as Partial<PayloadFailure>;
    const message = typeof failure?.message === 'string' ? failure.message : UNANSWERED;
    return { state: 'unavailable', failure: { message } };
  }

  $effect(() => {
    const ask = entry.read;

    if (ask === undefined) {
      return;
    }

    // docs/contracts/module-contract.md § An unavailable module and an unreachable backend are
    // different states
    if (!reachable) {
      payload = { state: 'loading' };
      return;
    }

    // A read settling after teardown is discarded.
    let current = true;
    const once = async () => {
      const settled = await read(ask);
      if (current) {
        payload = settled;
      }
    };

    void once();
    // docs/contracts/module-contract.md § Cadence and TTL are chosen together
    const polling = setInterval(() => void once(), entry.readIntervalMs);
    return () => {
      current = false;
      clearInterval(polling);
    };
  });

  /** Plain, not `$state`: a reactive read would re-run the effect below on its own write. */
  let clearFault: (() => void) | undefined;

  /** SRS069<!-- A module that stops drawing says so in its own place --> */
  function holdFault(_error: unknown, reset: () => void): void {
    clearFault = reset;
  }

  /** SRS070<!-- The display comes back on its own when the backend does --> */
  $effect(() => {
    if (!reachable) return;
    const reset = clearFault;
    clearFault = undefined;
    reset?.();
  });
</script>

<svelte:boundary onerror={holdFault}>
  {#if entry.read === undefined}
    <Module {reachable} {config} />
  {:else}
    <Module {reachable} {config} {payload} />
  {/if}

  {#snippet failed()}
    <p class="faulted" data-module-faulted role="alert">{FAULTED}</p>
  {/snippet}
</svelte:boundary>

<style>
  .faulted {
    margin: 0;
    font-size: var(--type-body);
    font-weight: var(--type-body-weight);
  }
</style>
