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
const SUBTREES = ['shell', 'readingOrder', 'lens']

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

const reviewMapDir = resolve(__dirname, '../components/review-map')
function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) {
      return ['shell', 'reading-order'].includes(e.name) ? sourceFiles(p) : []
    }
    return /\.(ts|tsx)$/.test(e.name) && !e.name.includes('.test.') ? [p] : []
  })
}

const enLeaves = SUBTREES.flatMap((s) => leaves(lookup(en, PREFIX + s), PREFIX + s))
const placeholders = (s: string): string[] => (s.match(/\{\{\w+\}\}/g) ?? []).sort()

describe('Review shell + reading order locale catalog', () => {
  it('has a non-trivial English subtree', () => {
    expect(enLeaves.length).toBeGreaterThan(100)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale}: every shell/readingOrder/lens key exists, is non-empty and keeps its placeholders`, () => {
      for (const [key, english] of enLeaves) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key} empty`).not.toBe('')
        expect(placeholders(value as string), `${locale}: ${key} placeholders`).toEqual(
          placeholders(english)
        )
      }
    })
  }

  it('keys composed at runtime (prefix + id) exist for every id', () => {
    const composed = [
      ...['invalid-base', 'unborn-head', 'no-merge-base', 'error'].flatMap((r) => [
        `shell.screen.scope.${r}.title`,
        `shell.screen.scope.${r}.body`
      ]),
      ...['low', 'medium', 'high', 'critical'].map((l) => `shell.risk.${l}`),
      ...['files', 'symbols', 'flows', 'tables', 'contracts', 'untested', 'violations'].map(
        (c) => `shell.chip.${c}`
      ),
      ...[
        'contract',
        'dependencyOf',
        'leaf',
        'cycle',
        'noEdges',
        'test',
        'doc',
        'generated',
        'overflow'
      ].map((r) => `readingOrder.reason.${r}`),
      ...['impact', 'architecture', 'dataflow', 'erd', 'storage', 'structure', 'contract'].map(
        (l) => `lens.${l}.label`
      )
    ]
    for (const catalog of Object.values(CATALOGS)) {
      expect(composed.filter((k) => typeof lookup(catalog, PREFIX + k) !== 'string')).toEqual([])
    }
  })

  it('every literal translate() key used by the shell sources exists in English', () => {
    const used = new Set<string>()
    for (const file of sourceFiles(reviewMapDir)) {
      const src = readFileSync(file, 'utf8')
      for (const m of src.matchAll(
        /'(auto\.components\.reviewMap\.(?:shell|readingOrder|lens)\.[\w.-]*[\w-])'/g
      )) {
        used.add(m[1])
      }
    }
    expect(used.size).toBeGreaterThan(50)
    const missing = [...used].filter((k) => typeof lookup(en, k) !== 'string')
    expect(missing).toEqual([])
  })
})
