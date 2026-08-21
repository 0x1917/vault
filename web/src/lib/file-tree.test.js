import { describe, it, expect } from 'vitest'
import {
  normalizePath,
  basename,
  toggleExcluded,
  isExcluded,
  summaryLabel,
  flattenVisibleTree,
} from './file-tree.js'

describe('normalizePath', () => {
  it('strips slashes and normalizes backslashes', () => {
    expect(normalizePath('/etc/conf/')).toBe('etc/conf')
    expect(normalizePath('data\\notes.txt')).toBe('data/notes.txt')
    expect(normalizePath('')).toBe('')
  })
})

describe('basename', () => {
  it('returns the last segment', () => {
    expect(basename('a/b/c.txt')).toBe('c.txt')
    expect(basename('top.txt')).toBe('top.txt')
  })
})

describe('toggleExcluded / isExcluded', () => {
  it('toggles immutably', () => {
    const a = new Set()
    const b = toggleExcluded(a, 'x')
    expect(isExcluded(b, 'x')).toBe(true)
    expect(isExcluded(a, 'x')).toBe(false)
    const c = toggleExcluded(b, 'x')
    expect(isExcluded(c, 'x')).toBe(false)
  })
})

describe('summaryLabel', () => {
  it('affirmatively reports all-selected vs exclusions', () => {
    expect(summaryLabel(10, 0)).toBe('Restoring all 10 files')
    expect(summaryLabel(1, 0)).toBe('Restoring all 1 file')
    expect(summaryLabel(10, 2)).toBe('Restoring everything except 2 paths')
    expect(summaryLabel(10, 1)).toBe('Restoring everything except 1 path')
  })
})

describe('flattenVisibleTree', () => {
  it('flattens expanded directories depth-first with depth annotations', () => {
    const roots = [
      { path: 'a', is_dir: true },
      { path: 'z.txt', is_dir: false },
    ]
    const childrenByDir = {
      a: [
        { path: 'a/b', is_dir: true },
        { path: 'a/x.txt', is_dir: false },
      ],
      'a/b': [{ path: 'a/b/c.txt', is_dir: false }],
    }
    const expanded = new Set(['a', 'a/b'])
    const out = flattenVisibleTree(roots, childrenByDir, expanded)
    expect(out.map(n => [n.path, n.depth])).toEqual([
      ['a', 0], ['a/b', 1], ['a/b/c.txt', 2], ['a/x.txt', 1], ['z.txt', 0],
    ])
  })

  it('does not descend unexpanded directories', () => {
    const roots = [{ path: 'a', is_dir: true }, { path: 'z.txt', is_dir: false }]
    const childrenByDir = { a: [{ path: 'a/x.txt', is_dir: false }] }
    const out = flattenVisibleTree(roots, childrenByDir, new Set())
    expect(out.map(n => n.path)).toEqual(['a', 'z.txt'])
  })
})
