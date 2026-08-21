// Container-selection scope helpers (#324). A job's container_scope is either
// 'all' (every current container is backed up automatically) or 'custom' (only
// the explicitly selected containers are backed up). These pure functions keep
// the wizard's scope-aware logic unit-testable in the node-only vitest setup.

export const CONTAINER_SCOPE_ALL = 'all'
export const CONTAINER_SCOPE_CUSTOM = 'custom'

// True when the scope is 'all'.
export function isAllScope(scope) {
  return scope === CONTAINER_SCOPE_ALL
}

// The item types the ItemPicker should still offer given the scope. In 'all'
// scope containers are auto-included at run time, so they are removed from the
// picker (and the picker's existing prune effect drops any explicit selection).
export function pickerTypesForScope(selectedTypes, scope) {
  if (isAllScope(scope)) {
    return selectedTypes.filter((t) => t !== 'containers')
  }
  return selectedTypes
}

// True when containers are automatically covered (all scope + containers type
// selected), i.e. no explicit container items are required.
export function containersCoveredByScope(selectedTypes, scope) {
  return isAllScope(scope) && selectedTypes.includes('containers')
}

// Whether the job has at least one effective item to back up. In 'all' scope
// the container set is resolved at run time, so selecting "containers" counts
// even with zero explicit items in the list.
export function hasEffectiveItems(items, selectedTypes, scope) {
  return items.length > 0 || containersCoveredByScope(selectedTypes, scope)
}
