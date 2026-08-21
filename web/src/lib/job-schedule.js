/**
 * Resolve the full-backup schedule a job should submit. `full_schedule`
 * only means anything for incremental/differential jobs (those that run a
 * non-full backup by default); a full-only job always submits '' so the
 * server never stores a full schedule for a job that can only run full
 * backups anyway.
 *
 * @param {string} backupTypeChain 'full' | 'incremental' | 'differential'
 * @param {string | null | undefined} fullSchedule cron expression or empty
 * @returns {string} the full schedule to submit, or '' to disable
 */
export function effectiveFullSchedule(backupTypeChain, fullSchedule) {
  if (backupTypeChain === 'full') return ''
  return (fullSchedule || '').trim()
}
