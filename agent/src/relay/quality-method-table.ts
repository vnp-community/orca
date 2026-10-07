export const QUALITY_METHODS = [
  'quality.listProfiles',
  'quality.run',
  'quality.runStatus',
  'quality.cancel',
  'quality.results',
  'quality.coverage'
]

export const QUALITY_METHOD_SCHEMAS: Record<string, string[]> = {
  'quality.listProfiles': ['workspaceRoot', '_trace'],
  'quality.run': ['workspaceRoot', 'profile', 'profiles', 'suites', 'base', '_trace'],
  'quality.runStatus': ['workspaceRoot', 'runId', '_trace'],
  'quality.cancel': ['workspaceRoot', 'runId', '_trace'],
  'quality.results': ['workspaceRoot', 'runId', 'offset', 'limit', 'view', 'stepId', '_trace'],
  'quality.coverage': ['workspaceRoot', 'runId', '_trace']
}
