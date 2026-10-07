import fs from 'fs'
import path from 'path'
import { AgentConfig } from './agent-config'
import { CodeIntelError } from './codeintel-errors'
import { readGitNexusRegistry, findRegistryEntry } from './gitnexus-registry-reader'
import { runGit } from './codeintel-git-exec'
import { getCodeIntelGitCapabilities } from './codeintel-git-capabilities'
import {
  isUnsupportedRevParsePathFormatError,
  hasUnsupportedRevParsePathFormatEcho
} from '../shared/git-worktree-command-capabilities'

export type CodeIntelRepoBinding = {
  toplevel: string
  gitNexusRegistryPath?: string
  codeGraphDbPath?: string
  linkedWorktree: boolean
  worktreeMismatch: boolean
  stale: boolean
}

type CachedBinding = {
  binding: CodeIntelRepoBinding
  timestamp: number
}
const cache = new Map<string, CachedBinding>()

export function invalidateRepoBindings(workspaceRoot?: string) {
  if (workspaceRoot) {
    cache.delete(workspaceRoot)
  } else {
    cache.clear()
  }
}

// Exported for agent-rpc-dispatch-quality and worktree verification
export async function resolveWorktreeRoot(
  workspaceRoot: string,
  config?: any,
  log?: any
): Promise<CodeIntelRepoBinding> {
  return resolveCodeIntelRepo(workspaceRoot, config ?? ({ workDir: workspaceRoot } as any), log)
}

export async function resolveCodeIntelRepo(workspaceRoot: string, config: AgentConfig, log?: any): Promise<CodeIntelRepoBinding> {
  if (!path.isAbsolute(workspaceRoot) || workspaceRoot.includes('\0')) {
    throw new CodeIntelError('CODEINTEL_PATH_NOT_ALLOWED', 'Invalid workspace root')
  }

  const cached = cache.get(workspaceRoot)
  if (cached && Date.now() - cached.timestamp < 30000) {
    return cached.binding
  }

  let realRoot: string
  try {
    const stat = fs.statSync(workspaceRoot)
    if (!stat.isDirectory()) throw new Error()
    realRoot = fs.realpathSync.native(workspaceRoot)
  } catch {
    throw new CodeIntelError('CODEINTEL_PATH_NOT_ALLOWED', 'Path does not exist or is not a directory')
  }

  const allowedRoots = process.env.ORCA_CODEINTEL_ALLOWED_ROOTS
  if (allowedRoots) {
    const roots = allowedRoots.split(path.delimiter).map(r => {
      try { return fs.realpathSync.native(r) } catch { return r }
    })
    const isAllowed = roots.some(r => realRoot === r || realRoot.startsWith(r + path.sep))
    if (!isAllowed) {
      throw new CodeIntelError('CODEINTEL_PATH_NOT_ALLOWED', 'Path is outside allowed roots')
    }
  }

  let toplevel: string
  try {
    const capCache = getCodeIntelGitCapabilities()
    
    const runPreferred = async () => {
      const res = await runGit(['rev-parse', '--path-format=absolute', '--show-toplevel'], realRoot)
      if (hasUnsupportedRevParsePathFormatEcho(res.stdout)) {
        throw new Error('unknown option --path-format')
      }
      return fs.realpathSync.native(res.stdout.trim())
    }

    const runFallback = async () => {
      const res = await runGit(['rev-parse', '--show-toplevel'], realRoot)
      const line = res.stdout.trim()
      return fs.realpathSync.native(path.resolve(realRoot, line))
    }

    toplevel = await capCache.runWithFallback(
      'rev-parse-path-format',
      runPreferred,
      runFallback,
      isUnsupportedRevParsePathFormatError
    )
  } catch (err: any) {
    throw new CodeIntelError('CODEINTEL_PATH_NOT_ALLOWED', 'Failed to read git repository', { hint: 'workspaceRoot must be a git work tree root' })
  }

  if (toplevel !== realRoot) {
    throw new CodeIntelError('CODEINTEL_PATH_NOT_ALLOWED', 'Path is not the top level of a worktree', { hint: 'workspaceRoot must be a git work tree root' })
  }

  let mainWorktree = toplevel
  let linkedWorktree = false
  try {
    const res = await runGit(['worktree', 'list', '--porcelain'], realRoot)
    const lines = res.stdout.split('\n')
    for (const line of lines) {
      if (line.startsWith('worktree ')) {
        const wtPath = line.substring('worktree '.length).trim()
        try {
          mainWorktree = fs.realpathSync.native(wtPath)
        } catch {
          mainWorktree = wtPath
        }
        break
      }
    }
    linkedWorktree = (mainWorktree !== toplevel)
  } catch {}

  let registryPath: string | undefined
  let worktreeMismatch = false
  let stale = false

  try {
    const registry = readGitNexusRegistry(config)
    let match = findRegistryEntry(registry, toplevel, log)
    if (match) {
      registryPath = match.path
    } else if (linkedWorktree) {
      match = findRegistryEntry(registry, mainWorktree, log)
      if (match) {
        registryPath = match.path
        worktreeMismatch = true
        stale = true
      }
    }
  } catch {}

  let codeGraphDbPath: string | undefined
  const checkCg = (p: string) => {
    const db = path.join(p, '.codegraph', 'codegraph.db')
    try {
      if (fs.statSync(db).isFile()) return db
    } catch {}
    return undefined
  }

  codeGraphDbPath = checkCg(toplevel)
  if (!codeGraphDbPath && linkedWorktree) {
    codeGraphDbPath = checkCg(mainWorktree)
    if (codeGraphDbPath) {
      worktreeMismatch = true
      stale = true
    }
  }

  if (!registryPath && !codeGraphDbPath) {
    throw new CodeIntelError('CODEINTEL_REPO_NOT_REGISTERED', 'Repository not indexed', { hint: 'run codeintel.reindex' })
  }

  const binding: CodeIntelRepoBinding = {
    toplevel,
    gitNexusRegistryPath: registryPath,
    codeGraphDbPath,
    linkedWorktree,
    worktreeMismatch,
    stale
  }

  cache.set(workspaceRoot, { binding, timestamp: Date.now() })
  return binding
}
