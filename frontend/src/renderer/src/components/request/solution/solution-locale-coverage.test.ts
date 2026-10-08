import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '../../../i18n/locales/en.json'
import es from '../../../i18n/locales/es.json'
import ja from '../../../i18n/locales/ja.json'
import ko from '../../../i18n/locales/ko.json'
import zh from '../../../i18n/locales/zh.json'

const P = 'auto.components.request.'
// Keys built dynamically (criterion rows, banner suffixes) are listed explicitly.
const DYNAMIC = [
  ...['summary', 'pros', 'cons', 'effort', 'risk'].map((c) => `SolutionComparisonTable.row.${c}`),
  ...['readOnly', 'generating', 'awaitingApproval', 'ready', 'chosen', 'rejected', 'superseded', 'unknown'].map(
    (b) => `SolutionPanel.banner.${b}`
  )
].map((k) => P + k)

function lookup(node: unknown, key: string): unknown {
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {
      return undefined
    }
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

const SOURCES = [
  'RejectReasonDialog.tsx', 'SolutionDecisionBar.tsx', 'SolutionPanel.tsx', 'SolutionOptionCard.tsx',
  'SolutionOptionCompare.tsx', 'SolutionComparisonTable.tsx', 'SolutionStatusBanner.tsx', 'DiagnosisView.tsx',
  'FindingsView.tsx', 'AnswerView.tsx', 'SolutionGenerationState.tsx', 'SolutionVersionSwitcher.tsx',
  'RequestMarkdownContent.tsx'
]

describe('solution locale coverage', () => {
  const keys = new Set(DYNAMIC)
  for (const f of SOURCES) {
    const src = readFileSync(join(__dirname, f), 'utf8')
    for (const m of src.matchAll(/'(auto\.components\.request\.[A-Za-z0-9_.]+)'/g)) {
      if (!m[1].endsWith('.')) {
        keys.add(m[1])
      }
    }
  }
  for (const [name, catalog] of Object.entries({ en, es, ja, ko, zh })) {
    it(`${name} has all solution keys`, () => {
      const missing = [...keys].filter((k) => typeof lookup(catalog, k) !== 'string' || lookup(catalog, k) === '')
      expect(missing).toEqual([])
    })
  }
})
