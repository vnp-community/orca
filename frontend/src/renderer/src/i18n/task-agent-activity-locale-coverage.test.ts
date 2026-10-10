import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const SOURCES = ['agent-activity/TaskAgentActivityPanel.tsx', 'TaskDispatchStatusPanel.tsx']

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {
      return undefined
    }
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

function scanKeys(): string[] {
  const keys = new Set<string>()
  for (const file of SOURCES) {
    const src = readFileSync(join(__dirname, '..', 'components', 'task', file), 'utf8')
    const prefix = /const T = '(auto\.components\.task\.[A-Za-z0-9_.]+\.)'/.exec(src)?.[1]
    for (const m of src.matchAll(/`\$\{T\}([A-Za-z0-9_.]+)`/g)) {
      keys.add(`${prefix}${m[1]}`)
    }
  }
  return [...keys]
}

const KEYS = scanKeys()

describe('task agent activity locale coverage', () => {
  it('finds the keys used by the panels', () => {
    expect(KEYS.length).toBeGreaterThanOrEqual(15)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has a non-empty string for every key`, () => {
      const missing = KEYS.filter((k) => {
        const v = lookup(catalog, k)
        return typeof v !== 'string' || v.trim() === '' || v === k
      })
      expect(missing).toEqual([])
    })
  }

  it('non-English locales are translated, not copies of English', () => {
    const copies = KEYS.filter((k) => {
      const base = lookup(en, k)
      return (
        typeof base === 'string' &&
        base.length > 12 &&
        ['es', 'ja', 'ko', 'zh'].every((l) => lookup(CATALOGS[l], k) === base)
      )
    })
    expect(copies).toEqual([])
  })
})
