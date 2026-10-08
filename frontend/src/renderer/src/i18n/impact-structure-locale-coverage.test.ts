import { readdirSync, readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const PREFIX = 'auto.components.reviewMap.'
const SUBTREES = ['impact', 'symbolDetail', 'structure', 'overlay']

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
function leaves(node: unknown, path: string): [string, string][] {
  if (typeof node === 'string') {
    return [[path, node]]
  }
  if (typeof node !== 'object' || node === null) {
    return []
  }
  return Object.entries(node).flatMap(([k, v]) => leaves(v, `${path}.${k}`))
}
const placeholders = (s: string): string[] => (s.match(/\{\{\w+\}\}/g) ?? []).sort()

const root = resolve(__dirname, '../components/review-map')
function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    return /\.(ts|tsx)$/.test(e.name) && !e.name.includes('.test.') ? [p] : []
  })
}
const files = [
  ...sources(join(root, 'impact')),
  ...sources(join(root, 'structure')),
  join(root, 'review-overlay-model.ts'),
  join(root, 'ReviewOverlayLegend.tsx')
]

const enLeaves = SUBTREES.flatMap((s) => leaves(lookup(en, PREFIX + s), PREFIX + s))

describe('Impact / symbol detail / structure / overlay locale catalog', () => {
  it('has a non-trivial English subtree', () => {
    expect(enLeaves.length).toBeGreaterThan(80)
  })
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: every key exists, is non-empty and keeps its placeholders`, () => {
      for (const [key, english] of enLeaves) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key}`).not.toBe('')
        expect(placeholders(value as string), `${locale}: ${key}`).toEqual(placeholders(english))
      }
    })
  }
  it('every statically written key in the sources exists in en', () => {
    const re = /['"`](auto\.components\.reviewMap\.(?:impact|symbolDetail|structure|overlay)\.[\w.]+)['"`]/g
    for (const f of files) {
      for (const m of readFileSync(f, 'utf8').matchAll(re)) {
        expect(typeof lookup(en, m[1]), `${f}: ${m[1]}`).toBe('string')
      }
    }
  })
})
