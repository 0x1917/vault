// Pure helpers for the restore wizard's lazy file tree (issue #323).
// Selection is expressed as an EXCLUSION set: a path present here is
// unchecked ("do not restore"); everything else restores by default.

// normalizePath maps a tar-entry path (forward or back slashes, optional
// leading/trailing slash) to the canonical relative form used by the API.
export function normalizePath(p) {
  return String(p ?? '').replace(/\\/g, '/').replace(/^\/+|\/+$/g, '')
}

// basename returns the final path segment.
export function basename(p) {
  const n = normalizePath(p)
  const i = n.lastIndexOf('/')
  return i === -1 ? n : n.slice(i + 1)
}

// toggleExcluded returns a new Set with `path` toggled in `excluded`.
export function toggleExcluded(excluded, path) {
  const next = new Set(excluded)
  if (next.has(path)) next.delete(path)
  else next.add(path)
  return next
}

// isExcluded reports whether a path is in the excluded set.
export function isExcluded(excluded, path) {
  return excluded.has(path)
}

// summaryLabel builds the affirmative picker summary. When nothing is
// excluded every file restores; otherwise the count of excluded paths is
// shown (a directory exclusion hides its whole subtree, so this is a path
// count, not a file count).
export function summaryLabel(totalFiles, excludedSize) {
  if (excludedSize === 0) {
    return `Restoring all ${totalFiles} ${totalFiles === 1 ? 'file' : 'files'}`
  }
  return `Restoring everything except ${excludedSize} ${excludedSize === 1 ? 'path' : 'paths'}`
}

// flattenVisibleTree produces a depth-annotated flat list of the currently
// visible nodes: every root node, plus the children of every expanded
// directory (recursively). Rendering is a simple indented flat list.
export function flattenVisibleTree(roots, childrenByDir, expanded) {
  const out = []
  const walk = (nodes, depth) => {
    for (const n of nodes) {
      out.push({ ...n, depth })
      if (n.is_dir && expanded.has(n.path)) {
        walk(childrenByDir[n.path] || [], depth + 1)
      }
    }
  }
  walk(roots || [], 0)
  return out
}
