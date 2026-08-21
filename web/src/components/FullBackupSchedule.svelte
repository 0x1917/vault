<script>
  import ScheduleBuilder from './ScheduleBuilder.svelte'
  import Tooltip from './Tooltip.svelte'

  /**
   * FullBackupSchedule – control for scheduling a job's referential FULL
   * backup (issue #322). Renders only for incremental/differential jobs.
   *
   * Props:
   * - backupTypeChain: 'full' | 'incremental' | 'differential'
   * - fullSchedule: cron expression (bindable); '' means disabled
   */
  let {
    backupTypeChain = 'full',
    fullSchedule = $bindable(''),
  } = $props()

  function toggleFullSchedule() {
    fullSchedule = fullSchedule === '' ? '0 3 * * 0' : ''
  }
</script>

{#if backupTypeChain !== 'full'}
  <div class="bg-surface-3/50 border border-border rounded-lg p-3 space-y-3">
    <label class="flex items-center gap-2 text-sm text-text cursor-pointer">
      <input type="checkbox" checked={fullSchedule !== ''} onchange={toggleFullSchedule} class="accent-vault" />
      Run a full backup on a separate schedule
      <Tooltip text="Differential and incremental backups depend on a full backup to restore from. Run a full backup on its own schedule so the chain always has a fresh anchor." />
    </label>
    {#if fullSchedule !== ''}
      <ScheduleBuilder bind:value={fullSchedule} />
      <p class="text-xs text-text-dim">
        This job runs a full backup on this schedule; the main schedule above keeps capturing only the changes.
      </p>
    {/if}
  </div>
{/if}
