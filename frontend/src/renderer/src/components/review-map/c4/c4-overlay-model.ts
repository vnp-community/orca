/**
 * c4-overlay-model.ts — FE-CV-TASK-055-01
 *
 * Maps the change overlay onto C4 components and relations. Pure: callers pass the overlay.
 */

import type { C4Component, C4Relation } from '../../../../../shared/code-intel-architecture-types'
import type { ChangeOverlayView } from '../review-wire-types'

export type C4ImpactInput = { affectedFiles: readonly string[] }

export type C4OverlayFlags = {
  changed: boolean
  untested: boolean
  violation: boolean
  /** Only meaningful when impact data was loaded; false otherwise. */
  affected: boolean
}

/** Windows and POSIX separators compare equal; leading ./ and trailing / never matter. */
export function normalizeRepoPath(path: string): string {
  return path.replace(/\\/g, '/').replace(/^\.\//, '').replace(/\/+$/, '')
}

function isUnder(file: string, prefix: string): boolean {
  if (prefix === '' || prefix === '.') {
    return true
  }
  return file === prefix || file.startsWith(`${prefix}/`)
}

/** Component paths are container-relative in some payloads and repo-relative in others. */
export function c4ComponentPrefixes(
  component: Pick<C4Component, 'path' | 'packagePaths'>,
  containerPath?: string
): string[] {
  const out = new Set<string>()
  const base = containerPath ? normalizeRepoPath(containerPath) : ''
  for (const raw of [component.path, ...component.packagePaths]) {
    if (!raw) {
      continue
    }
    const p = normalizeRepoPath(raw)
    out.add(p)
    if (base && p !== base && !p.startsWith(`${base}/`)) {
      out.add(`${base}/${p}`)
    }
  }
  return [...out]
}

function anyUnder(files: readonly string[], prefixes: readonly string[]): boolean {
  return files.some((f) => {
    const nf = normalizeRepoPath(f)
    return prefixes.some((p) => isUnder(nf, p))
  })
}

export function computeC4OverlayFlags(
  component: Pick<C4Component, 'path' | 'packagePaths'>,
  overlay: Pick<ChangeOverlayView, 'changedFiles' | 'uncoveredSymbols' | 'violations'>,
  impact?: C4ImpactInput | null,
  containerPath?: string
): C4OverlayFlags {
  const prefixes = c4ComponentPrefixes(component, containerPath)
  return {
    changed: anyUnder(
      overlay.changedFiles.map((f) => f.path),
      prefixes
    ),
    untested: anyUnder(
      overlay.uncoveredSymbols.map((s) => s.filePath),
      prefixes
    ),
    violation: anyUnder(
      overlay.violations.map((v) => v.file),
      prefixes
    ),
    affected: impact ? anyUnder(impact.affectedFiles, prefixes) : false
  }
}

export type C4RelationFlags = { touchesChange: boolean; violation: boolean }

export function computeC4RelationFlags(
  relation: Pick<C4Relation, 'evidence' | 'violatesLayering'>,
  overlay: Pick<ChangeOverlayView, 'changedSymbols'>
): C4RelationFlags {
  const changed = new Set(overlay.changedSymbols.map((c) => c.symbol.key))
  return {
    touchesChange: relation.evidence.some((e) => changed.has(e.key)),
    violation: relation.violatesLayering === true
  }
}
