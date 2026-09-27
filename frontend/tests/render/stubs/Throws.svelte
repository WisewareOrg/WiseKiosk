<script lang="ts">
  /**
   * Throws while unreachable: `bind:this` writes `null` as the `{#if}` block is destroyed, the guard
   * tests only `undefined`, and the throw lands mid-flush of that teardown. That shape stops the page,
   * where a plain throwing `$effect` does not, so do not simplify it or widen the type.
   */
  const { reachable }: { reachable: boolean } = $props();

  let el: HTMLElement | undefined = $state();

  $effect(() => {
    const node = el;
    if (node === undefined) return;
    node.querySelectorAll('.line');
  });
</script>

{#if reachable}
  <div class="stub" data-stub="throws" bind:this={el}>
    <p class="line">Throws</p>
  </div>
{/if}

<style>
  .stub {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .line {
    margin: 0;
    font-size: var(--type-body);
    font-weight: var(--type-body-weight);
  }
</style>
