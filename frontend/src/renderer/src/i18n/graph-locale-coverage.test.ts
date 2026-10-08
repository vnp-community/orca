import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const PREFIX = 'auto.components.graph.'

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {return undefined}
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

// Keys composed at runtime (`${T}${key}` tables) that the static scan cannot see.
const DYNAMIC_KEYS = [
  ...['noAssessment', 'noPlan', 'notExecuting', 'unsupported'].map((k) => `LensDisabled.${k}`),
  ...['added', 'removed', 'unchanged'].map((k) => `List.Change.${k}`),
  ...['kind', 'label', 'group', 'risk', 'status', 'change', 'relations'].map((k) => `List.col.${k}`),
  ...['flow', 'architecture', 'contract', 'data', 'impact', 'plan', 'execution'].map((k) => `lens.${k}`),
  ...['low', 'medium', 'high', 'critical', 'unknown'].map((k) => `RiskLevel.${k}`),
  ...['added', 'removed', 'modified', 'breaking', 'irreversible', 'open', 'in_progress', 'review', 'done', 'blocked', 'cancelled'].map((k) => `Status.${k}`),
  'Edge.added', 'Edge.removed', 'RiskTooltip', 'RiskTooltipUnassessed'
].map((k) => PREFIX + k)

function collectSourceKeys(): string[] {
  const keys = new Set<string>()
  const roots = [join(__dirname, '..', 'components', 'graph'), join(__dirname, '..', 'components', 'request')]
  const walk = (dir: string): void => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name)
      if (entry.isDirectory()) {walk(full)}
      else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\./.test(entry.name)) {
        const src = readFileSync(full, 'utf8')
        for (const m of src.matchAll(/['"`](auto\.components\.graph\.[A-Za-z0-9_.]+)['"`]/g)) {
          if (!m[1].endsWith('.')) {keys.add(m[1])}
        }
        const prefix = /const [TP] = '(auto\.components\.graph\.[A-Za-z0-9_.]*\.)'/.exec(src)?.[1]
        if (prefix) {
          for (const m of src.matchAll(/`\$\{[TP]\}([A-Za-z0-9_.]+)`/g)) {keys.add(prefix + m[1])}
        }
      }
    }
  }
  roots.forEach(walk)
  return [...keys]
}

const ALL_KEYS = [...new Set([...collectSourceKeys(), ...DYNAMIC_KEYS])]

describe('graph locale coverage', () => {
  it('scans a non-trivial number of keys', () => {
    expect(ALL_KEYS.length).toBeGreaterThan(60)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every graph key with a non-empty translation`, () => {
      const missing = ALL_KEYS.filter((k) => {
        const v = lookup(catalog, k)
        return typeof v !== 'string' || v.trim() === '' || v === k
      })
      expect(missing).toEqual([])
    })

    it(`${locale} keeps interpolation placeholders`, () => {
      const bad = ALL_KEYS.filter((k) => {
        const base = lookup(en, k)
        const v = lookup(catalog, k)
        if (typeof base !== 'string' || typeof v !== 'string') {return false}
        const names = base.match(/\{\{\w+\}\}/g) ?? []
        return names.some((n) => !v.includes(n))
      })
      expect(bad).toEqual([])
    })
  }

  it('never uses the word "safe" for risk copy in English', () => {
    const risky = ALL_KEYS.filter((k) => /safe/i.test(String(lookup(en, k))))
    expect(risky).toEqual([])
  })
})
