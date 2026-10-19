import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const PREFIX = 'auto.components.task.TaskDetail.openInGit.'

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

const source = readFileSync(join(__dirname, '..', 'components', 'task', 'TaskDetail.tsx'), 'utf8')
const KEYS = [
  ...new Set(
    [...source.matchAll(/'(auto\.components\.task\.TaskDetail\.openInGit\.[A-Za-z]+)'/g)].map(
      (m) => m[1]
    )
  )
]

describe('"Open in Git" messages locale coverage', () => {
  it('finds the four messages', () => {
    expect(KEYS.map((k) => k.slice(PREFIX.length)).sort()).toEqual([
      'failed',
      'noProject',
      'notOnHost',
      'unknown'
    ])
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

  it('the failure message keeps its {{message}} placeholder in every language', () => {
    for (const catalog of Object.values(CATALOGS)) {
      expect(lookup(catalog, `${PREFIX}failed`)).toContain('{{message}}')
    }
  })
})
