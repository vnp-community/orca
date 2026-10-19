import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// SOL-FE-E2E-004 (web repo host binding): every string added for choosing, showing and changing a
// repo's dev server, plus the bounded Browse folder messages. Shared files list their keys
// explicitly; files owned by this feature are also scanned so a new key cannot ship untranslated.
const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }
const RENDERER_SRC = join(__dirname, '..')

const SHARED_FILE_KEYS: [string, string[]][] = [
  [
    'components/project/CreateProjectDialog.tsx',
    ['auto.components.project.CreateProjectDialog.repoAddFailed']
  ],
  [
    'components/sidebar/AddRepoHostSelector.tsx',
    ['auto.components.sidebar.AddRepoHostSelector.chooseHost']
  ],
  [
    'components/sidebar/RemoteFileBrowser.tsx',
    [
      'auto.components.sidebar.RemoteFileBrowser.hostReconnecting',
      'auto.components.sidebar.RemoteFileBrowser.retry',
      'auto.components.sidebar.RemoteFileBrowser.truncated',
      'auto.components.sidebar.RemoteFileBrowser.hostOffline'
    ]
  ],
  [
    'components/worktree-creation/WorktreeCreationPanel.tsx',
    ['auto.components.worktree.creation.WorktreeCreationPanel.chooseHost']
  ],
  [
    'lib/workspace-create-error-format.ts',
    [
      'auto.lib.workspace.create.error.format.hostUnbound.title',
      'auto.lib.workspace.create.error.format.hostUnbound.message',
      'auto.lib.workspace.create.error.format.hostUnreachable.title',
      'auto.lib.workspace.create.error.format.hostUnreachable.message',
      'auto.lib.workspace.create.error.format.hostCheckFailed.title',
      'auto.lib.workspace.create.error.format.hostCheckFailed.message'
    ]
  ],
  ['store/slices/repos.ts', ['auto.store.slices.repos.setupProjectCloneDevServerUnsupported']],
  [
    'components/settings/RepositoryDevServerHostSection.tsx',
    [
      'auto.components.settings.RepositoryDevServerHostSection.title',
      'auto.components.settings.RepositoryDevServerHostSection.description',
      'auto.components.settings.RepositoryDevServerHostSection.unbound',
      'auto.components.settings.RepositoryDevServerHostSection.unknownStatus',
      'auto.components.settings.RepositoryDevServerHostSection.fromProject',
      'auto.components.settings.RepositoryDevServerHostSection.change'
    ]
  ],
  [
    'components/sidebar/AddRepoHostGateNotice.tsx',
    [
      'auto.components.sidebar.AddRepoHostGateNotice.reconnecting',
      'auto.components.sidebar.AddRepoHostGateNotice.removed',
      'auto.components.sidebar.AddRepoHostGateNotice.offline',
      'auto.components.sidebar.AddRepoHostGateNotice.noHostTitle',
      'auto.components.sidebar.AddRepoHostGateNotice.noHostBody',
      'auto.components.sidebar.AddRepoHostGateNotice.openDevServers',
      'auto.components.sidebar.AddRepoHostGateNotice.chooseHostBody'
    ]
  ],
  ['components/sidebar/RepoHostBadge.tsx', ['auto.components.sidebar.RepoHostBadge.noHost']],
  [
    'components/sidebar/RepoHostPickerDialog.tsx',
    [
      'auto.components.sidebar.RepoHostPickerDialog.updated',
      'auto.components.sidebar.RepoHostPickerDialog.title',
      'auto.components.sidebar.RepoHostPickerDialog.description',
      'auto.components.sidebar.RepoHostPickerDialog.hostLabel',
      'auto.components.sidebar.RepoHostPickerDialog.placeholder',
      'auto.components.sidebar.RepoHostPickerDialog.unavailableHost',
      'auto.components.sidebar.RepoHostPickerDialog.noConnectedHost',
      'auto.components.sidebar.RepoHostPickerDialog.openDevServers',
      'auto.components.sidebar.RepoHostPickerDialog.cancel',
      'auto.components.sidebar.RepoHostPickerDialog.saving',
      'auto.components.sidebar.RepoHostPickerDialog.confirm'
    ]
  ],
  ['lib/repo-host-binding.ts', ['auto.lib.repoHostBinding.hostRequired']],
  [
    'lib/repo-host-rebind.ts',
    [
      'auto.lib.repoHostRebind.activeWorkflows',
      'auto.lib.repoHostRebind.devServerNotFound',
      'auto.lib.repoHostRebind.forbidden',
      'auto.lib.repoHostRebind.failed'
    ]
  ]
]

const OWNED_FILE_PREFIXES: [string, string][] = [
  [
    'components/sidebar/AddRepoHostGateNotice.tsx',
    'auto.components.sidebar.AddRepoHostGateNotice.'
  ],
  ['components/sidebar/RepoHostBadge.tsx', 'auto.components.sidebar.RepoHostBadge.'],
  ['components/sidebar/RepoHostPickerDialog.tsx', 'auto.components.sidebar.RepoHostPickerDialog.'],
  [
    'components/settings/RepositoryDevServerHostSection.tsx',
    'auto.components.settings.RepositoryDevServerHostSection.'
  ],
  ['lib/repo-host-binding.ts', 'auto.lib.repoHostBinding.'],
  ['lib/repo-host-rebind.ts', 'auto.lib.repoHostRebind.']
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

describe('repo host binding locale coverage', () => {
  it('finds the messages in their source files', () => {
    for (const [file, keys] of SHARED_FILE_KEYS) {
      const source = readSource(file)
      for (const key of keys) {
        expect(source, `${file} should use ${key}`).toContain(`'${key}'`)
      }
    }
    expect(ALL_KEYS.length).toBeGreaterThan(40)
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
