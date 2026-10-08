import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '../../../i18n/locales/en.json'
import es from '../../../i18n/locales/es.json'
import ja from '../../../i18n/locales/ja.json'
import ko from '../../../i18n/locales/ko.json'
import zh from '../../../i18n/locales/zh.json'

// Plan components build keys as `${P}name` with a per-file prefix constant, which the
// generic request coverage scan cannot see; resolve them here.
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }

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

function collectPlanKeys(): string[] {
  const keys = new Set<string>()
  for (const name of readdirSync(__dirname)) {
    if (!/\.(ts|tsx)$/.test(name) || /\.test\./.test(name)) {
      continue
    }
    const src = readFileSync(join(__dirname, name), 'utf8')
    const prefix = /const P = '([^']+)'/.exec(src)?.[1]
    if (prefix) {
      for (const m of src.matchAll(/\$\{P\}([A-Za-z0-9_.]+)/g)) {
        keys.add(prefix + m[1])
      }
    }
    for (const m of src.matchAll(/'(auto\.components\.request\.plan\.[A-Za-z0-9_.]+)'/g)) {
      if (!m[1].endsWith('.')) {
        keys.add(m[1])
      }
    }
  }
  keys.add('auto.components.task.TaskGraph.showPlanning')
  keys.add('auto.components.task.TaskDetail.phaseNotApproved')
  return [...keys]
}

describe('plan locale coverage', () => {
  const keys = collectPlanKeys()
  it('finds the plan keys', () => {
    expect(keys.length).toBeGreaterThan(40)
  })
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every plan key as a non-empty string`, () => {
      const missing = keys.filter((k) => {
        const v = lookup(catalog, k)
        return typeof v !== 'string' || v.trim() === ''
      })
      expect(missing).toEqual([])
    })
  }
})
