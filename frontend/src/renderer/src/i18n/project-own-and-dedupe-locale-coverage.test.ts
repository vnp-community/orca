import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// SOL-FE-E2E-005 (own project per Add Project, duplicate repo block, Answer view fields): every
// string added for it, in all five locales. Files owned by this feature are scanned so a new key
// cannot ship untranslated.
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const RENDERER_SRC = join(__dirname, '..')

const SHARED_FILE_KEYS: [string, string[]][] = [
  [
    'components/request/solution/AnswerView.tsx',
    [
      'auto.components.request.AnswerView.confidence',
      'auto.components.request.AnswerView.assumptions',
      'auto.components.request.AnswerView.limitations',
      'auto.components.request.AnswerView.followUp',
      'auto.components.request.AnswerView.followUpWithType'
    ]
  ],
  [
    'components/sidebar/AddRepoServerStartStep.tsx',
    [
      'auto.components.sidebar.AddRepoServerStartStep.projectName',
      'auto.components.sidebar.AddRepoServerStartStep.projectNamePlaceholder',
      'auto.components.sidebar.AddRepoServerStartStep.projectNameHint'
    ]
  ],
  [
    'store/slices/repos.ts',
    ['auto.store.slices.repos.repoAlreadyAdded', 'auto.store.slices.repos.openExistingProject']
  ]
]

const OWNED_FILE_PREFIXES: [string, string][] = [
  [
    'components/request/CreateRequestProjectRepoNote.tsx',
    'auto.components.request.CreateRequestProjectRepoNote.'
  ],
  [
    'components/sidebar/AddRepoDuplicateNotice.tsx',
    'auto.components.sidebar.AddRepoDuplicateNotice.'
  ],
  ['lib/own-project-registration.ts', 'auto.lib.ownProjectRegistration.'],
  ['lib/registered-repo-lookup.ts', 'auto.lib.registeredRepoLookup.']
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

function placeholders(text: string): string[] {
  return [...text.matchAll(/{{\s*([A-Za-z0-9_]+)\s*}}/g)].map((match) => match[1]).sort()
}

const readSource = (relativePath: string): string =>
  readFileSync(join(RENDERER_SRC, relativePath), 'utf8')

const ALL_KEYS = [
  ...new Set([
    ...SHARED_FILE_KEYS.flatMap(([, keys]) => keys),
    ...OWNED_FILE_PREFIXES.flatMap(([file, prefix]) =>
      [...readSource(file).matchAll(/'(auto\.[A-Za-z0-9.]+)'/g)]
        .map((match) => match[1])
        .filter((key) => key.startsWith(prefix))
    )
  ])
]

describe('own project and duplicate repo locale coverage', () => {
  it('finds the messages in their source files', () => {
    for (const [file, keys] of SHARED_FILE_KEYS) {
      const source = readSource(file)
      for (const key of keys) {
        expect(source, `${file} should use ${key}`).toContain(`'${key}'`)
      }
    }
    expect(ALL_KEYS.length).toBeGreaterThanOrEqual(20)
  })

  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has a non-empty string for every key`, () => {
      const missing = ALL_KEYS.filter((key) => {
        const value = lookup(catalog, key)
        return typeof value !== 'string' || value.trim() === '' || value === key
      })
      expect(missing).toEqual([])
    })

    it(`${locale} keeps every {{placeholder}} of the English text`, () => {
      const broken = ALL_KEYS.filter((key) => {
        const english = lookup(en, key)
        const translated = lookup(catalog, key)
        return (
          typeof english === 'string' &&
          typeof translated === 'string' &&
          placeholders(english).join() !== placeholders(translated).join()
        )
      })
      expect(broken).toEqual([])
    })
  }

  it('translates the other four locales instead of copying English', () => {
    const copied = ALL_KEYS.filter((key) => {
      const english = lookup(en, key)
      // Short words and pure-placeholder strings are legitimately identical.
      if (typeof english !== 'string' || english.length < 14 || /^{{[^}]+}}/.test(english)) {
        return false
      }
      return ['es', 'ja', 'ko', 'zh'].some((locale) => lookup(CATALOGS[locale], key) === english)
    })
    expect(copied).toEqual([])
  })
})
