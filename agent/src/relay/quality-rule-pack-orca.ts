import { RulePack } from './quality-rule-pack-schema'

export const ORCA_RULE_PACK: RulePack = {
  version: 1,
  pack: 'orca-conventions',
  rules: [
    {
      id: 'ORCA-001',
      title: 'Ratchet max-lines',
      kind: 'profile-ref',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: [], exclude: [], fileStatus: [] },
      ref: { profileId: 'repo-check-max-lines' },
      message: 'New max-lines bypass not allowed',
      fixHint: 'Split the file or seek a waiver',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-002',
      title: 'Styled scrollbars',
      kind: 'profile-ref',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: [], exclude: [], fileStatus: [] },
      ref: { profileId: 'repo-check-styled-scrollbars' },
      message: 'Unstyled scrollbar detected',
      fixHint: 'Use styled scrollbars component',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-003',
      title: 'Reliability gates',
      kind: 'profile-ref',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: [], exclude: [], fileStatus: [] },
      ref: { profileId: 'repo-check-reliability-gates' },
      message: 'Reliability gate manifest check failed',
      fixHint: 'Fix the manifest according to the error',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-004',
      title: 'Verify localization catalog',
      kind: 'script',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['**/i18n/**'], exclude: [], fileStatus: ['added', 'modified', 'renamed'] },
      script: {
        name: 'verify-localization-catalog',
        locate: ['desktop/config/scripts/verify-localization-catalog.mjs'],
        cwd: 'desktop',
        args: []
      },
      message: 'Localization catalog is invalid',
      fixHint: 'Run verify-localization-catalog locally and fix errors',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-005',
      title: 'Audit localization coverage',
      kind: 'script',
      category: 'convention',
      severity: 'warning',
      enabled: true,
      scope: { include: ['desktop/src/renderer/**/*.tsx'], exclude: [], fileStatus: ['added', 'modified', 'renamed'] },
      script: {
        name: 'audit-localization-coverage',
        locate: ['desktop/config/scripts/audit-localization-coverage.mjs'],
        cwd: 'desktop',
        args: []
      },
      message: 'Localization coverage missing',
      fixHint: 'Wrap user-facing strings in t() or translate()',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-006',
      title: 'Feature wall assets check',
      kind: 'script',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['resources/onboarding/feature-wall/**'], exclude: [], fileStatus: ['added', 'modified', 'renamed'] },
      script: {
        name: 'check-feature-wall-assets',
        locate: ['desktop/config/scripts/check-feature-wall-assets.mjs'],
        cwd: 'desktop',
        args: []
      },
      message: 'Feature wall assets check failed',
      fixHint: 'Fix assets per the script output',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-007',
      title: 'No new .d.ts files',
      kind: 'diff',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['**/{preload,shared}/**/*.d.ts'], exclude: [], fileStatus: ['added'] },
      match: {
        type: 'added-file-name',
        pattern: '\\.d\\.ts$',
        maxLineLength: 200
      },
      message: 'New .d.ts files are not allowed here',
      fixHint: 'Use .ts instead of .d.ts for type definitions in preload/shared',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-010',
      title: 'No new max-lines disable',
      kind: 'diff',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['**/*'], exclude: ['**/*.test.*', '**/*.md'], fileStatus: ['added', 'modified', 'renamed'] },
      match: {
        type: 'added-line-regex',
        pattern: '(eslint-disable|oxlint-disable-next-line)\\s+max-lines',
        maxLineLength: 2000
      },
      message: 'Adding new max-lines disable comments is prohibited',
      fixHint: 'Refactor to reduce line count',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-011',
      title: 'Mobile config max-lines raised',
      kind: 'diff',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['mobile/.oxlintrc.json'], exclude: [], fileStatus: ['added', 'modified'] },
      match: {
        type: 'file-content-regex',
        pattern: '"max-lines"\\s*:',
        maxLineLength: 2000
      },
      message: 'Mobile config max-lines was raised',
      fixHint: 'Do not increase max-lines budgets',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-012',
      title: 'Ambiguous file or folder name',
      kind: 'diff',
      category: 'convention',
      severity: 'warning',
      enabled: true,
      scope: { include: ['**/*'], exclude: [], fileStatus: ['added'] },
      match: {
        type: 'added-file-name',
        pattern: '(helpers|utils|common|misc|shared-stuff|\\w+-helpers\\.ts|\\w+-utils\\.ts)',
        maxLineLength: 200
      },
      message: 'Ambiguous naming detected',
      fixHint: 'Use specific domain names instead of generic ones like utils/helpers',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-013',
      title: 'Hardcoded hex in UI',
      kind: 'diff',
      category: 'convention',
      severity: 'warning',
      enabled: true,
      scope: { include: ['desktop/src/renderer/**/*.tsx', 'desktop/src/renderer/**/*.css'], exclude: [], fileStatus: ['added', 'modified', 'renamed'] },
      match: {
        type: 'added-line-regex',
        pattern: '#[0-9a-fA-F]{3,6}',
        maxLineLength: 500
      },
      message: 'Hardcoded hex colors are discouraged',
      fixHint: 'Use theme variables from the design system',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-014',
      title: 'Hardcoded metaKey',
      kind: 'diff',
      category: 'convention',
      severity: 'warning',
      enabled: true,
      scope: { include: ['desktop/src/renderer/**/*.tsx', 'desktop/src/renderer/**/*.ts'], exclude: [], fileStatus: ['added', 'modified', 'renamed'] },
      match: {
        type: 'added-line-regex',
        pattern: '\\.metaKey\\b',
        maxLineLength: 500
      },
      message: 'Hardcoded metaKey detected',
      fixHint: 'Use cross-platform key checks if applicable',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    },
    {
      id: 'ORCA-015',
      title: 'New Git command flags',
      kind: 'diff',
      category: 'convention',
      severity: 'info',
      enabled: true,
      scope: { include: ['desktop/src/main/git/**/*.ts', 'backend/**/*.go'], exclude: [], fileStatus: ['added', 'modified', 'renamed'] },
      match: {
        type: 'added-line-regex',
        pattern: 'git\\s+(?!status|log|add|commit|push|pull|fetch)',
        maxLineLength: 2000
      },
      message: 'Ensure new Git flags are compatible with older clients',
      fixHint: 'Verify backwards compatibility',
      source: { doc: 'docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md' }
    }
  ]
}
