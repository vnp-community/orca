// src/relay/agent-git-base-ref-handler.ts
// git.baseRefDefault / git.searchRefs — split out of agent-git-handler.ts
// (which crossed the repo's oxlint max-lines budget) to keep both files
// under it. Pure mechanical extraction, bodies unchanged.
//
// Mirrors localgit/executor.go's BaseRefDefault/SearchRefs so both host
// paths (local executor, this relay handler) agree.

import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

// defaultBranchFromRemoteHead: fallback for a repo with no
// refs/remotes/origin/HEAD (only `git clone`/`remote set-head -a` set it —
// never `git init` + `git remote add`, even after a fetch). Asks the
// remote directly via `git ls-remote --symref origin HEAD`:
//   ref: refs/heads/main	HEAD
async function defaultBranchFromRemoteHead(cwd: string): Promise<string> {
  const { execFile } = await import('node:child_process')
  const { promisify } = await import('node:util')
  const execAsync = promisify(execFile)
  const { stdout } = await execAsync('git', ['ls-remote', '--symref', 'origin', 'HEAD'], {
    cwd,
    timeout: 10_000
  })
  const prefix = 'ref: refs/heads/'
  for (const line of stdout.split('\n')) {
    if (!line.startsWith(prefix)) {
      continue
    }
    const rest = line.slice(prefix.length).split('\t')[0]?.trim()
    if (rest) {
      return rest
    }
  }
  return ''
}

export async function handleGitBaseRefDefault(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const cwd = typeof params.repoPath === 'string' ? params.repoPath : config.workDir
  try {
    const { execFile } = await import('node:child_process')
    const { promisify } = await import('node:util')
    const execAsync = promisify(execFile)
    try {
      const { stdout } = await execAsync('git', ['symbolic-ref', 'refs/remotes/origin/HEAD'], {
        cwd,
        timeout: 10_000
      })
      // "refs/remotes/origin/main" -> "main"
      const ref = stdout.trim().split('/').pop() ?? ''
      log.info(`git.baseRefDefault: cwd=${cwd} ref=${ref}`)
      return { jsonrpc: '2.0', id, result: { ref } }
    } catch (symbolicRefErr: unknown) {
      const ref = await defaultBranchFromRemoteHead(cwd)
      if (!ref) {
        throw symbolicRefErr
      }
      log.info(`git.baseRefDefault: cwd=${cwd} ref=${ref} (via ls-remote fallback)`)
      return { jsonrpc: '2.0', id, result: { ref } }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.ServerError, message: `git.baseRefDefault failed: ${msg}` }
    }
  }
}

// ─── git.searchRefs ──────────────────────────────────────────────────────────
// Substring match over ref short names, same as localgit/executor.go's
// SearchRefs (`git for-each-ref --format=%(refname:short)`, filtered client-side).

export async function handleGitSearchRefs(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const cwd = typeof params.repoPath === 'string' ? params.repoPath : config.workDir
  const query = typeof params.query === 'string' ? params.query : ''
  try {
    const { execFile } = await import('node:child_process')
    const { promisify } = await import('node:util')
    const execAsync = promisify(execFile)
    const { stdout } = await execAsync('git', ['for-each-ref', '--format=%(refname:short)'], {
      cwd,
      timeout: 10_000
    })
    const refs = stdout
      .trim()
      .split('\n')
      .filter((line) => line !== '' && (query === '' || line.includes(query)))
    log.info(`git.searchRefs: cwd=${cwd} query=${query} count=${refs.length}`)
    return { jsonrpc: '2.0', id, result: { refs } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.ServerError, message: `git.searchRefs failed: ${msg}` }
    }
  }
}
