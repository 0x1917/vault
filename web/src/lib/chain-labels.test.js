import { describe, it, expect } from 'vitest'

import { chainDeps, baseFullReference, chainReplayText } from './chain-labels.js'

// Deterministic formatter so assertions never depend on the runner's
// timezone or locale.
const fmt = (iso) => new Date(iso).toISOString().slice(0, 10)

describe('chainDeps', () => {
  it('returns 0 for a standalone full or missing depth', () => {
    expect(chainDeps({ chain_depth: 1 })).toBe(0)
    expect(chainDeps({})).toBe(0)
  })

  it('returns the count of earlier backups in the chain', () => {
    expect(chainDeps({ chain_depth: 2 })).toBe(1)
    expect(chainDeps({ chain_depth: 3 })).toBe(2)
  })
})

describe('baseFullReference', () => {
  it('is empty when there is no base-full reference', () => {
    expect(baseFullReference({}, fmt)).toBe('')
    expect(baseFullReference({ base_full_restore_point_at: null }, fmt)).toBe('')
  })

  it('names the full backup date', () => {
    const at = '2026-08-18T09:00:00Z'
    expect(baseFullReference({ base_full_restore_point_at: at }, fmt))
      .toBe(`based on the full backup from ${fmt(at)}`)
  })
})

describe('chainReplayText', () => {
  it('singular, with a known base full', () => {
    const at = '2026-08-18T09:00:00Z'
    expect(chainReplayText({ chain_depth: 2, base_full_restore_point_at: at }, fmt))
      .toBe(`Restore replays 1 earlier backup in this chain, based on the full backup from ${fmt(at)}.`)
  })

  it('plural, legacy point with no base full', () => {
    expect(chainReplayText({ chain_depth: 3 }, fmt))
      .toBe('Restore replays 2 earlier backups in this chain.')
  })
})
