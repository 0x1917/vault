// Shared, pure helpers for describing a restore point's position in a backup
// chain. No Svelte or DOM dependencies, so vitest can cover them in the node
// environment (see lib/*.test.js conventions).

export function chainDeps(rp) {
  return Math.max(0, (rp?.chain_depth || 1) - 1)
}

// "based on the full backup from <date>" — the reference point a differential
// or incremental restore is built on. Returns '' when the point is standalone,
// the chain is broken, or the API response predates base_full_restore_point_at.
export function baseFullReference(rp, formatDate) {
  if (!rp?.base_full_restore_point_at) return ''
  return `based on the full backup from ${formatDate(rp.base_full_restore_point_at)}`
}

// Full timeline sentence, including the base-full reference when known.
export function chainReplayText(rp, formatDate) {
  const deps = chainDeps(rp)
  const plural = deps === 1 ? '' : 's'
  const base = baseFullReference(rp, formatDate)
  return `Restore replays ${deps} earlier backup${plural} in this chain${base ? `, ${base}` : ''}.`
}
