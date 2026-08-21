import { describe, it, expect } from 'vitest'

import {
  isAllScope,
  pickerTypesForScope,
  containersCoveredByScope,
  hasEffectiveItems,
} from './container-scope.js'

describe('container-scope helpers', () => {
  it('isAllScope', () => {
    expect(isAllScope('all')).toBe(true)
    expect(isAllScope('custom')).toBe(false)
    expect(isAllScope(undefined)).toBe(false)
  })

  it('pickerTypesForScope drops containers in all scope', () => {
    const types = ['containers', 'vms', 'folders']
    expect(pickerTypesForScope(types, 'all')).toEqual(['vms', 'folders'])
    expect(pickerTypesForScope(types, 'custom')).toEqual(types)
  })

  it('containersCoveredByScope', () => {
    expect(containersCoveredByScope(['containers'], 'all')).toBe(true)
    expect(containersCoveredByScope(['containers'], 'custom')).toBe(false)
    expect(containersCoveredByScope(['vms'], 'all')).toBe(false)
  })

  it('hasEffectiveItems', () => {
    expect(hasEffectiveItems([], ['containers'], 'all')).toBe(true)
    expect(hasEffectiveItems([], ['containers'], 'custom')).toBe(false)
    expect(hasEffectiveItems([{ item_type: 'vm', item_name: 'win10' }], ['vms'], 'custom')).toBe(true)
  })
})
