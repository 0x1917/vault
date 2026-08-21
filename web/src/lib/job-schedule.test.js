import { describe, it, expect } from 'vitest'
import { effectiveFullSchedule } from './job-schedule.js'

describe('effectiveFullSchedule', () => {
  it('returns the trimmed schedule for incremental/differential jobs', () => {
    expect(effectiveFullSchedule('incremental', '0 3 * * 0')).toBe('0 3 * * 0')
    expect(effectiveFullSchedule('differential', ' 0 3 * * 1 ')).toBe('0 3 * * 1')
  })

  it('returns empty string for full-only jobs', () => {
    expect(effectiveFullSchedule('full', '0 3 * * 0')).toBe('')
  })

  it('returns empty string when no schedule is set', () => {
    expect(effectiveFullSchedule('incremental', '')).toBe('')
    expect(effectiveFullSchedule('incremental', null)).toBe('')
    expect(effectiveFullSchedule('incremental', '   ')).toBe('')
  })
})
