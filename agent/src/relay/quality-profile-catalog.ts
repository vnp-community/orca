import { QualityCheckProfile, QualitySuite, validateProfile } from './quality-profile-schema'

export const BUILTIN_PROFILES: readonly QualityCheckProfile[] = [
  {
    id: 'ts-lint',
    title: 'Oxlint',
    parser: 'oxlint@json',
    cwd: '.',
    argv: ['{bin:oxlint}', '--format', 'json', '{files|.}'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'append-files',
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: false,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'oxlint' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'ts-typecheck-desktop-node',
    title: 'Typecheck Desktop Node',
    parser: 'tsc@json',
    cwd: 'desktop',
    argv: ['{bin:tsc}', '--noEmit', '-p', 'config/tsconfig.node.json', '--pretty', 'false'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'full-run-filter',
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'tsc' }, { type: 'file', path: 'desktop/config/tsconfig.node.json' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'ts-typecheck-desktop-web',
    title: 'Typecheck Desktop Web',
    parser: 'tsc@json',
    cwd: 'desktop',
    argv: ['{bin:tsc}', '--noEmit', '-p', 'config/tsconfig.tc.web.json', '--pretty', 'false'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'full-run-filter',
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'tsc' }, { type: 'file', path: 'desktop/config/tsconfig.tc.web.json' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'ts-typecheck-desktop-cli',
    title: 'Typecheck Desktop CLI',
    parser: 'tsc@json',
    cwd: 'desktop',
    argv: ['{bin:tsc}', '--noEmit', '-p', 'config/tsconfig.tc.cli.json', '--pretty', 'false'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'full-run-filter',
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'tsc' }, { type: 'file', path: 'desktop/config/tsconfig.tc.cli.json' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'ts-typecheck-frontend',
    title: 'Typecheck Frontend',
    parser: 'tsc@json',
    cwd: 'frontend',
    argv: ['{bin:tsc}', '--noEmit', '-p', 'tsconfig.json', '--pretty', 'false'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'full-run-filter',
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'tsc' }, { type: 'file', path: 'frontend/tsconfig.json' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'ts-typecheck-agent',
    title: 'Typecheck Agent',
    parser: 'tsc@json',
    cwd: 'agent',
    argv: ['{bin:tsc}', '--noEmit', '-p', 'tsconfig.json', '--pretty', 'false'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'full-run-filter',
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    enabled: false, // Task 11 didn't see tsc working seamlessly or it's composite
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'tsc' }, { type: 'file', path: 'agent/tsconfig.json' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'ts-unit-desktop',
    title: 'Unit Test Desktop',
    parser: 'vitest@json',
    cwd: 'desktop',
    argv: ['{bin:vitest}', 'run', '--config', 'config/vitest.config.ts', '--reporter=json', '--outputFile={tmp:vitest.json}'],
    scopeArgv: {
      changed: ['{bin:vitest}', 'related', '--run', '--config', 'config/vitest.config.ts', '--reporter=json', '--outputFile={tmp:vitest.json}', '{files}']
    },
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'append-files',
    timeoutMs: 600000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    needsNativeRuntime: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'vitest' }, { type: 'file', path: 'desktop/config/vitest.config.ts' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'ts-unit-agent',
    title: 'Unit Test Agent',
    parser: 'vitest@json',
    cwd: 'agent',
    argv: ['{bin:vitest}', 'run', '--reporter=json', '--outputFile={tmp:vitest.json}'],
    scopeArgv: { changed: ['{bin:vitest}', 'related', '--run', '--reporter=json', '--outputFile={tmp:vitest.json}', '{files}'] },
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'append-files',
    timeoutMs: 600000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'vitest' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'ts-unit-backend',
    title: 'Unit Test Backend Node',
    parser: 'vitest@json',
    cwd: 'backend',
    argv: ['{bin:vitest}', 'run', '--reporter=json', '--outputFile={tmp:vitest.json}'],
    scopeArgv: { changed: ['{bin:vitest}', 'related', '--run', '--reporter=json', '--outputFile={tmp:vitest.json}', '{files}'] },
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'append-files',
    timeoutMs: 600000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'vitest' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'ts-unit-frontend',
    title: 'Unit Test Frontend',
    parser: 'vitest@json',
    cwd: 'frontend',
    argv: ['{bin:vitest}', 'run', '--reporter=json', '--outputFile={tmp:vitest.json}'],
    scopeArgv: {
      changed: ['{bin:vitest}', 'related', '--run', '--reporter=json', '--outputFile={tmp:vitest.json}', '{files}']
    },
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'append-files',
    timeoutMs: 600000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'vitest' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'repo-check-max-lines',
    title: 'Max Lines Ratchet',
    parser: 'raw',
    cwd: '.',
    argv: ['node', 'config/scripts/check-max-lines-ratchet.mjs'],
    scopes: ['worktree'],
    scopeStrategy: 'none',
    timeoutMs: 300000,
    maxOutputBytes: 1024 * 1024,
    heavy: false,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'file', path: 'config/scripts/check-max-lines-ratchet.mjs' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'repo-check-styled-scrollbars',
    title: 'Styled Scrollbars Check',
    parser: 'raw',
    cwd: 'desktop',
    argv: ['node', 'config/scripts/check-styled-scrollbars.mjs'],
    scopes: ['worktree'],
    scopeStrategy: 'none',
    timeoutMs: 300000,
    maxOutputBytes: 1024 * 1024,
    heavy: false,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'file', path: 'desktop/config/scripts/check-styled-scrollbars.mjs' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'repo-check-reliability-gates',
    title: 'Reliability Gates Check',
    parser: 'raw',
    cwd: 'desktop',
    argv: ['node', 'config/scripts/check-reliability-gates.mjs'],
    scopes: ['worktree'],
    scopeStrategy: 'none',
    timeoutMs: 300000,
    maxOutputBytes: 1024 * 1024,
    heavy: false,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'file', path: 'desktop/config/scripts/check-reliability-gates.mjs' }],
    exit: { ok: [0], findings: [1] }
  },
  {
    id: 'go-vet',
    title: 'Go Vet',
    parser: 'go-vet@json',
    cwd: 'backend-go',
    argv: ['{bin:go}', 'vet', '-json', './...'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'go-modules',
    perModule: true,
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'go' }, { type: 'file', path: 'backend-go/go.work' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'go-test',
    title: 'Go Test',
    parser: 'go-test@json',
    cwd: 'backend-go',
    argv: ['{bin:go}', 'test', '-json', './...'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'go-modules',
    perModule: true,
    timeoutMs: 600000,
    maxOutputBytes: 20 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'go' }, { type: 'file', path: 'backend-go/go.work' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'go-lint',
    title: 'GolangCI-Lint',
    parser: 'golangci-lint@json',
    cwd: 'backend-go',
    argv: ['{bin:golangci-lint}', 'run', '--out-format', 'json', './...'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'go-modules',
    perModule: true,
    timeoutMs: 600000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: true,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'golangci-lint' }, { type: 'file', path: 'backend-go/go.work' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'proto-lint',
    title: 'Buf Lint',
    parser: 'buf@json',
    cwd: 'backend-go/proto',
    argv: ['{bin:buf}', 'lint', '--error-format', 'json'],
    scopes: ['worktree'],
    scopeStrategy: 'none',
    timeoutMs: 120000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: false,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'buf' }, { type: 'file', path: 'backend-go/proto/buf.yaml' }],
    exit: { ok: [0], findings: [1, 100] }
  },
  {
    id: 'proto-breaking',
    title: 'Buf Breaking',
    parser: 'buf@json',
    cwd: 'backend-go/proto',
    argv: ['{bin:buf}', 'breaking', '--against', '{gitCommonDir}#branch={base},subdir=backend-go/proto', '--error-format', 'json'],
    scopes: ['changed', 'commitRange'], // Needs {base}
    scopeStrategy: 'none',
    timeoutMs: 300000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: false,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'buf' }, { type: 'file', path: 'backend-go/proto/buf.yaml' }],
    exit: { ok: [0], findings: [1, 100] }
  },
  {
    id: 'opa-test',
    title: 'OPA Test',
    parser: 'opa-test@json',
    cwd: 'backend-go',
    argv: ['{bin:opa}', 'test', 'policy/orca-authz/', '--format', 'json'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'full-run-filter',
    timeoutMs: 120000,
    maxOutputBytes: 10 * 1024 * 1024,
    heavy: false,
    env: { set: {}, allowExtra: [] },
    requires: [{ type: 'bin', name: 'opa' }],
    exit: { ok: [0], findings: [1, 2] }
  },
  {
    id: 'coverage-go',
    title: 'Go Coverage',
    parser: 'coverage@go',
    cwd: 'backend-go',
    argv: ['{bin:go}', 'test', '-covermode=set', '-coverprofile={tmp:coverage.out}', './...'],
    scopes: ['worktree', 'changed', 'commitRange'],
    scopeStrategy: 'go-modules',
    timeoutMs: 900000,
    maxOutputBytes: 20 * 1024 * 1024,
    heavy: true,
    env: { set: { GOFLAGS: '-mod=readonly' }, allowExtra: [] },
    requires: [{ type: 'bin', name: 'go' }, { type: 'file', path: 'backend-go/go.work' }],
    exit: { ok: [0], findings: [1] }
  }
]

export const BUILTIN_SUITES: readonly QualitySuite[] = [
  {
    id: 'fast',
    title: 'Fast Checks',
    profiles: ['ts-lint', 'repo-check-max-lines', 'go-vet']
  },
  {
    id: 'standard',
    title: 'Standard Checks',
    profiles: [
      'ts-lint', 'repo-check-max-lines', 'go-vet',
      'ts-typecheck-desktop-node', 'ts-typecheck-desktop-web', 'ts-typecheck-desktop-cli',
      'ts-typecheck-agent', 'ts-typecheck-frontend',
      'ts-unit-desktop', 'ts-unit-frontend', 'ts-unit-agent', 'ts-unit-backend',
      'go-test', 'proto-lint', 'opa-test',
      'repo-check-styled-scrollbars', 'repo-check-reliability-gates',
      'repo-rules', 'repo-rules-scripts'
    ]
  },
  {
    id: 'full',
    title: 'Full Checks',
    profiles: [
      'ts-lint', 'repo-check-max-lines', 'go-vet',
      'ts-typecheck-desktop-node', 'ts-typecheck-desktop-web', 'ts-typecheck-desktop-cli',
      'ts-typecheck-agent', 'ts-typecheck-frontend',
      'ts-unit-desktop', 'ts-unit-frontend',
      'go-test', 'proto-lint', 'opa-test',
      'repo-check-styled-scrollbars', 'repo-check-reliability-gates',
      'go-lint', 'proto-breaking',
      'repo-rules', 'repo-rules-scripts'
    ]
  }
]

const registry = new Map<string, QualityCheckProfile>()

export function registerBuiltinProfiles(extra: readonly QualityCheckProfile[]) {
  for (const p of extra) {
    if (registry.has(p.id)) throw new Error(`Profile id ${p.id} already registered`)
    registry.set(p.id, p)
  }
}

export function getCatalog(): { profiles: QualityCheckProfile[]; suites: QualitySuite[] } {
  const profiles: QualityCheckProfile[] = []
  
  const all = [...BUILTIN_PROFILES, ...registry.values()]
  for (const p of all) {
    if (p.enabled === false) continue
    const val = validateProfile(p)
    if (!val.ok) {
      console.warn(`Invalid built-in profile ${p.id}: ${val.errors.join(', ')}`)
      continue
    }
    profiles.push(val.value)
  }

  const validIds = new Set(profiles.map(p => p.id))
  
  const suites: QualitySuite[] = []
  for (const s of BUILTIN_SUITES) {
    // only keep profiles that actually exist and are valid/enabled
    const active = s.profiles.filter(id => validIds.has(id))
    suites.push({ ...s, profiles: active })
  }

  return { profiles, suites }
}
