export function getGolangciSeverity(linter: string): 'error' | 'warning' | 'info' {
  const errors = new Set([
    'typecheck',
    'errcheck',
    'staticcheck',
    'govet',
    'bodyclose',
    'noctx',
    'errorlint'
  ])

  const warnings = new Set([
    'ineffassign',
    'unused',
    'gosimple',
    'gocritic'
  ])

  const infos = new Set([
    'gofmt',
    'goimports'
  ])

  if (errors.has(linter)) return 'error'
  if (warnings.has(linter)) return 'warning'
  if (infos.has(linter)) return 'info'

  return 'warning' // unknown
}
