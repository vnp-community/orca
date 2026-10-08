import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const GROUPS = ['clarification', 'decision', 'impact', 'readiness', 'execution'] as const

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {return undefined}
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

// Keys composed at runtime that the static scan cannot see.
const DYNAMIC: Record<(typeof GROUPS)[number], string[]> = {
  clarification: ['source.readiness', 'source.solution_open_question', 'source.plan_assumption', 'source.task_blocked'],
  decision: ['status.open', 'status.chosen', 'status.effective', 'status.superseded', 'status.unknown'],
  impact: [
    ...['architecture', 'contract', 'data', 'blast_radius', 'security', 'operations', 'quality', 'uncertainty', 'size'].map((d) => `dim.${d}`),
    'confidence.low', 'confidence.medium', 'confidence.high'
  ],
  readiness: ['tier.structure', 'tier.semantic', 'tier.environment'],
  execution: ['status.done', 'status.failed', 'status.blocked', 'status.needs_info', 'secrets.passed', 'secrets.failed', 'secrets.notRun',
    ...['retryable', 'spec_defect', 'needs_info', 'env_defect', 'agent_defect', 'unknown'].map((c) => `route.${c}`)]
}

function scanGroup(group: string): string[] {
  const root = join(__dirname, '..', 'components', 'request', group)
  const keys = new Set<string>()
  for (const entry of readdirSync(root)) {
    if (!/\.(ts|tsx)$/.test(entry) || /\.test\./.test(entry)) {continue}
    const src = readFileSync(join(root, entry), 'utf8')
    for (const m of src.matchAll(/['"`](auto\.components\.request\.[A-Za-z0-9_.]+)['"`]/g)) {
      if (!m[1].endsWith('.')) {keys.add(m[1])}
    }
    const prefix = /const T = '(auto\.components\.request\.[A-Za-z0-9_.]+\.)'/.exec(src)?.[1]
    if (prefix) {
      for (const m of src.matchAll(/`\$\{T\}([A-Za-z0-9_.]+)`/g)) {keys.add(prefix + m[1])}
    }
  }
  return [...keys]
}

export const REQUEST_ARTIFACT_LOCALE_KEYS: string[] = [
  ...new Set(
    GROUPS.flatMap((g) => [...scanGroup(g), ...DYNAMIC[g].map((k) => `auto.components.request.${g}.${k}`)])
  )
]

describe('request artifact locale coverage', () => {
  it('scans a non-trivial set of keys', () => {
    expect(REQUEST_ARTIFACT_LOCALE_KEYS.length).toBeGreaterThan(120)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has a non-empty string for every key`, () => {
      const missing = REQUEST_ARTIFACT_LOCALE_KEYS.filter((k) => {
        const v = lookup(catalog, k)
        return typeof v !== 'string' || v.trim() === '' || v === k
      })
      expect(missing).toEqual([])
    })

    it(`${locale} keeps the same interpolation variables as en`, () => {
      const bad = REQUEST_ARTIFACT_LOCALE_KEYS.filter((k) => {
        const base = lookup(en, k)
        const v = lookup(catalog, k)
        if (typeof base !== 'string' || typeof v !== 'string') {return false}
        return (base.match(/\{\{\w+\}\}/g) ?? []).some((n) => !v.includes(n))
      })
      expect(bad).toEqual([])
    })
  }

  it('non-English locales are translated, not copies of English', () => {
    const copies = REQUEST_ARTIFACT_LOCALE_KEYS.filter((k) => {
      const base = lookup(en, k)
      return typeof base === 'string' && base.length > 12 && ['es', 'ja', 'ko', 'zh'].every((l) => lookup(CATALOGS[l], k) === base)
    })
    expect(copies).toEqual([])
  })

  it('English copy never claims safety or verification', () => {
    const offenders = REQUEST_ARTIFACT_LOCALE_KEYS.filter((k) => /\b(safe|verified)\b/i.test(String(lookup(en, k))))
    expect(offenders).toEqual([])
  })
})
