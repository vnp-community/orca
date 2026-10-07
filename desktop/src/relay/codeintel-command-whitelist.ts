import { assertSafeClientString, assertRelativeRepoPath } from './codeintel-params-validation'
import { CodeIntelError } from './codeintel-errors'

export const GITNEXUS_VERBS = ['status', 'context', 'cypher', 'impact', 'query', 'detect_changes', 'check-cycles'] as const
export type GitNexusVerb = typeof GITNEXUS_VERBS[number]

export const CODEGRAPH_VERBS = ['status', 'files', 'search', 'node', 'affected', 'reindex'] as const
export type CodeGraphVerb = typeof CODEGRAPH_VERBS[number]

export const FORBIDDEN_SUBCOMMANDS = [
  'analyze', 'clean', 'remove', 'uninstall', 'publish', 'setup', 'index', 'init', 'uninit', 
  'sync', 'serve', 'mcp', 'wiki', 'group', 'daemon', 'unlock', 'install', 'upgrade', 
  'telemetry', 'eval-server', 'check'
]

export type GitNexusCommand = 
  | { verb: 'status' }
  | { verb: 'context', symbol: string }
  | { verb: 'cypher', query: string }
  | { verb: 'impact', symbol: string, direction?: 'upstream' | 'downstream' }
  | { verb: 'query', filter: string, kind?: string, limit?: number }
  | { verb: 'detect_changes', base?: string, head?: string }
  | { verb: 'check-cycles' }

export type CodeGraphCommand =
  | { verb: 'status' }
  | { verb: 'files' }
  | { verb: 'search', query: string }
  | { verb: 'node', id: string }
  | { verb: 'affected', files: string[] }
  | { verb: 'reindex' }

export function buildGitNexusArgv(cmd: GitNexusCommand, registryPath: string): string[] {
  if (!GITNEXUS_VERBS.includes(cmd.verb as any)) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Invalid GitNexus verb: ${cmd.verb}`)
  }
  
  if (cmd.verb === 'check-cycles') {
    return ['check', '--cycles', '--json', '-r', registryPath]
  }

  const args: string[] = [cmd.verb]

  switch (cmd.verb) {
    case 'context':
      assertSafeClientString(cmd.symbol, 'symbol')
      args.push(cmd.symbol)
      break
    case 'cypher':
      assertSafeClientString(cmd.query, 'query')
      args.push(cmd.query)
      break
    case 'impact':
      assertSafeClientString(cmd.symbol, 'symbol')
      args.push(cmd.symbol)
      if (cmd.direction) {
        if (cmd.direction !== 'upstream' && cmd.direction !== 'downstream') {
           throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Invalid impact direction: ${cmd.direction}`)
        }
        args.push(`--${cmd.direction}`)
      }
      break
    case 'query':
      assertSafeClientString(cmd.filter, 'filter')
      args.push(cmd.filter)
      if (cmd.kind) {
        assertSafeClientString(cmd.kind, 'kind')
        args.push('--kind', cmd.kind)
      }
      if (cmd.limit !== undefined) {
        if (!Number.isInteger(cmd.limit) || cmd.limit < 1) {
          throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'limit must be a positive integer')
        }
        args.push('--limit', String(cmd.limit))
      }
      break
    case 'detect_changes':
      if (cmd.base) {
        assertSafeClientString(cmd.base, 'base')
        args.push('--base', cmd.base)
      }
      if (cmd.head) {
        assertSafeClientString(cmd.head, 'head')
        args.push('--head', cmd.head)
      }
      break
  }

  args.push('-r', registryPath)
  return args
}

export function buildCodeGraphArgv(cmd: CodeGraphCommand, projectPath: string): string[] {
  if (!CODEGRAPH_VERBS.includes(cmd.verb as any)) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Invalid CodeGraph verb: ${cmd.verb}`)
  }

  const args: string[] = [cmd.verb]

  if (cmd.verb === 'status') {
    args.push(projectPath, '-j')
    return args
  }

  if (cmd.verb !== 'node') {
    args.push('-j')
  }

  switch (cmd.verb) {
    case 'search':
      assertSafeClientString(cmd.query, 'query')
      args.push(cmd.query)
      break
    case 'node':
      assertSafeClientString(cmd.id, 'id')
      args.push(cmd.id)
      break
    case 'affected':
      for (let i = 0; i < cmd.files.length; i++) {
        assertRelativeRepoPath(cmd.files[i], `files[${i}]`)
        args.push(cmd.files[i])
      }
      break
  }

  args.push('-p', projectPath)
  return args
}
