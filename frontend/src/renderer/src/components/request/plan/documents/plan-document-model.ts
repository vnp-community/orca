/**
 * plan-document-model.ts — CR-REQ-045
 *
 * What the Documents view lists for an OpenSpec change: the order of the tabs (proposal, design, specs, tasks, the rest), their
 * labels, and the two readings the body needs (a file the backend kept only as a placeholder, a size in bytes). Pure.
 *
 * @module components/request/plan/documents/plan-document-model
 */

import { translate } from '@/i18n/i18n'
import type {
  PlanBundleFileEntry,
  PlanFileChange
} from '../../../../../../shared/request-plan-revision'

export type DocumentGroup = 'proposal' | 'design' | 'spec' | 'tasks' | 'other'

export type DocumentTab = {
  path: string
  label: string
  group: DocumentGroup
  omitted: boolean
  /** Bytes; the real size of the file also when it was too large to keep. */
  size: number
  change: PlanFileChange
}

const SPEC_PATH = /^specs\/(.+)\/spec\.md$/
const GROUP_ORDER: Record<DocumentGroup, number> = {
  proposal: 0,
  design: 1,
  spec: 2,
  tasks: 3,
  other: 4
}

/** A path from the wire is shown and requested as it is, so only a plain relative path inside the change is accepted. */
export function isSafeDocumentPath(path: string): boolean {
  if (path === '' || path.startsWith('/') || path.startsWith('\\') || /^[A-Za-z]:/.test(path)) {
    return false
  }
  if (path.includes('\0') || path.includes('\\')) {
    return false
  }
  return path.split('/').every((segment) => segment !== '' && segment !== '.' && segment !== '..')
}

function groupOf(path: string): { group: DocumentGroup; label: string } {
  if (path === 'proposal.md') {
    return {
      group: 'proposal',
      label: translate('auto.components.request.plan.documents.tab.proposal', 'Proposal')
    }
  }
  if (path === 'design.md') {
    return {
      group: 'design',
      label: translate('auto.components.request.plan.documents.tab.design', 'Design')
    }
  }
  if (path === 'tasks.md') {
    return {
      group: 'tasks',
      label: translate('auto.components.request.plan.documents.tab.tasks', 'Tasks')
    }
  }
  const spec = SPEC_PATH.exec(path)
  if (spec) {
    return { group: 'spec', label: spec[1] }
  }
  return { group: 'other', label: path }
}

/** The tabs of a change in reading order; a path that could leave the change is left out. */
export function orderDocuments(files: readonly PlanBundleFileEntry[]): DocumentTab[] {
  const seen = new Set<string>()
  const tabs: DocumentTab[] = []
  for (const file of files) {
    if (!isSafeDocumentPath(file.path) || seen.has(file.path)) {
      continue
    }
    seen.add(file.path)
    tabs.push({
      path: file.path,
      ...groupOf(file.path),
      omitted: file.omitted,
      size: file.size,
      change: file.change
    })
  }
  return tabs.sort(
    (a, b) =>
      GROUP_ORDER[a.group] - GROUP_ORDER[b.group] ||
      (a.group === 'spec' || a.group === 'other' ? a.label.localeCompare(b.label, 'en') : 0) ||
      a.path.localeCompare(b.path, 'en')
  )
}

const OMITTED_PLACEHOLDER = /^<omitted\b[^>]*>$/

/** Fallback for a gateway that sends the placeholder as the text without setting `omitted`. */
export function isOmittedPlaceholder(content: string): boolean {
  return OMITTED_PLACEHOLDER.test(content.trim())
}

/** `1.5 MB`, `300 KB`, `12 B`: sizes of documents, so binary units with one decimal at most. */
export function formatBytes(bytes: number): string {
  const n = Number.isFinite(bytes) && bytes > 0 ? bytes : 0
  const number = (value: number): string =>
    new Intl.NumberFormat('en', { maximumFractionDigits: 1 }).format(value)
  if (n < 1024) {
    return `${number(n)} B`
  }
  if (n < 1024 * 1024) {
    return `${number(n / 1024)} KB`
  }
  return `${number(n / (1024 * 1024))} MB`
}
