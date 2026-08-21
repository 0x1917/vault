<script>
  /**
   * ContainerScopeSelector – radio cards choosing whether the job backs up
   * ALL containers or a CUSTOM selection (#324).
   *
   * Props:
   * - scope: 'all' | 'custom' (bindable)
   */
  let { scope = $bindable('custom') } = $props()

  import Tooltip from './Tooltip.svelte'

  const options = [
    {
      value: 'all',
      label: 'All containers',
      description: 'Back up every container on this server. New containers are added automatically, and containers you delete are removed automatically.',
      icon: 'M17 14h-5v5h5m-5-5H6a2 2 0 01-2-2V6a2 2 0 012-2h12a2 2 0 012 2v4m-6 8h6',
    },
    {
      value: 'custom',
      label: 'Custom selection',
      description: 'Back up only the containers you pick below. New containers are not added until you select them.',
      icon: 'M4 6h16M4 10h16M4 14h10',
    },
  ]
</script>

<div>
  <span class="block text-sm font-medium text-text mb-2">Container Selection <Tooltip text="All containers keeps the backup in step with your server automatically. Custom selection backs up exactly the containers you choose." /></span>
  <div class="grid grid-cols-1 gap-2">
    {#each options as opt (opt.value)}
      <button
        type="button"
        onclick={() => (scope = opt.value)}
        class="flex items-start gap-3 p-3 rounded-lg border-2 transition-all text-left {scope === opt.value
          ? 'border-vault bg-vault/5'
          : 'border-border hover:border-border-hover bg-surface-3/50'}"
      >
        <div
          class="w-5 h-5 mt-0.5 rounded-full border-2 flex items-center justify-center shrink-0 transition-colors {scope === opt.value
            ? 'border-vault'
            : 'border-border'}"
        >
          {#if scope === opt.value}
            <div class="w-2.5 h-2.5 rounded-full bg-vault"></div>
          {/if}
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <svg aria-hidden="true" class="w-4 h-4 text-text-muted shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"
              ><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={opt.icon} /></svg
            >
            <span class="text-sm font-medium text-text">{opt.label}</span>
          </div>
          <p class="text-xs text-text-muted mt-1 leading-relaxed">{opt.description}</p>
        </div>
      </button>
    {/each}
  </div>
</div>
