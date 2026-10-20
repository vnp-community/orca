import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Outcome Review keys are read by name: this keeps every locale in step with what the review components reference, and keeps
// the words of the review from overclaiming (nothing "verified", "safe to release" or "requirements met").
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const PREFIX = 'auto.components.request.outcomeReview.'
const EXTRA_KEYS = [
  'auto.components.request.StageTimeline.step.review',
  'auto.components.request.RequestDetailPane.tab.outcomeReview'
]

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

function sourceFiles(): string[] {
  const root = join(__dirname, '..', 'components', 'request')
  const out: string[] = [join(root, 'next-action-review.ts'), join(root, 'RequestDetailTabs.tsx')]
  const walk = (dir: string): void => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name)
      if (entry.isDirectory()) {
        walk(full)
      } else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\.|\.fixture\./.test(entry.name)) {
        out.push(full)
      }
    }
  }
  walk(join(root, 'outcome-review'))
  return out
}

function collectKeys(): string[] {
  const keys = new Set<string>()
  for (const file of sourceFiles()) {
    const src = readFileSync(file, 'utf8')
    for (const m of src.matchAll(
      /['"`](auto\.components\.request\.outcomeReview\.[A-Za-z0-9_.]+)['"`]/g
    )) {
      if (!m[1].endsWith('.')) {
        keys.add(m[1])
      }
    }
    // Keys written as `${T}name` where the file declares `const T = 'auto.components.request.outcomeReview.X.'`.
    const prefix =
      /const T = '(auto\.components\.request\.outcomeReview\.(?:[A-Za-z0-9_]+\.)*)'/.exec(src)?.[1]
    if (prefix) {
      for (const m of src.matchAll(/`\$\{T\}([A-Za-z0-9_.]+)`/g)) {
        keys.add(prefix + m[1])
      }
    }
  }
  return [...keys]
}

// Words an overclaiming review would use. "Unverified" is the honest label and is allowed.
const FORBIDDEN = [
  /safe to release/i,
  /(?<![A-Za-z])verified/i,
  /requirements met/i,
  /ready to ship/i,
  /(?<![A-Za-z])confirmed/i,
  /all clear/i
]

describe('outcome review locale coverage', () => {
  const keys = [...new Set([...collectKeys(), ...EXTRA_KEYS])]

  it('finds the keys of the review components', () => {
    expect(keys.filter((k) => k.startsWith(PREFIX)).length).toBeGreaterThan(100)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every outcome review key as a non-empty string`, () => {
      const missing = keys.filter((key) => {
        const value = lookup(catalog, key)
        return typeof value !== 'string' || value.trim() === ''
      })
      expect(missing).toEqual([])
    })
  }

  it('keeps the same placeholders in every locale', () => {
    const placeholders = (text: string): string[] =>
      [...text.matchAll(/\{\{(\w+)\}\}/g)].map((m) => m[1]).sort()
    const broken: string[] = []
    for (const key of keys) {
      const base = placeholders(String(lookup(en, key)))
      for (const [locale, catalog] of Object.entries(CATALOGS)) {
        if (placeholders(String(lookup(catalog, key))).join() !== base.join()) {
          broken.push(`${locale}:${key}`)
        }
      }
    }
    expect(broken).toEqual([])
  })

  it('says nothing the review cannot back up (English)', () => {
    const offenders = keys.filter((key) => FORBIDDEN.some((re) => re.test(String(lookup(en, key)))))
    expect(offenders).toEqual([])
  })
})
