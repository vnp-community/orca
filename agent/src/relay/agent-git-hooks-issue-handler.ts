// src/relay/agent-git-hooks-issue-handler.ts
// git.checkHooks / git.readIssueCommand / git.writeIssueCommand /
// git.scanSetupScriptImports — dev-server-agent side of the 4 usecases
// git-gateway-service's dispatchExecutorForRepo migration (see
// check_hooks.go et al.) started routing to a repo's real host instead of
// its worktree. That fix exposed a pre-existing gap: these 4 methods were
// never implemented here, only on localgit/executor.go's local-execution
// path — every call for a dev-server-hosted repo failed with "Method not
// found". Mirrors localgit/executor.go's CheckHooks/ReadIssueCommand/
// WriteIssueCommand/ScanSetupScriptImports 1:1 so both host paths agree.
//
// Wire contract (fixed by relay_executor.go, already live — do not change):
//   git.checkHooks             {repoPath} -> {installedHooks, orcaHooksCurrent}
//   git.readIssueCommand       {repoPath} -> {content, exists}
//   git.writeIssueCommand      {repoPath, content} -> {}
//   git.scanSetupScriptImports {repoPath} -> {importedPaths}

import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

function repoPathOf(params: Record<string, unknown>, config: AgentConfig): string {
  return typeof params.repoPath === 'string' && params.repoPath ? params.repoPath : config.workDir
}

// ─── git.checkHooks ──────────────────────────────────────────────────────────
// Lists installed hooks under .git/hooks and reports whether orca's own
// hooks (pre-commit, post-checkout) are present — name-only check, no
// content diffing against a known-good version (matches executor.go).

export async function handleGitCheckHooks(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const repoPath = repoPathOf(params, config)
  try {
    const entries = await fs.readdir(path.join(repoPath, '.git', 'hooks'), { withFileTypes: true })
    const installedHooks: string[] = []
    let hasPreCommit = false
    let hasPostCheckout = false
    for (const entry of entries) {
      if (entry.isDirectory() || entry.name.endsWith('.sample')) {
        continue
      }
      installedHooks.push(entry.name)
      if (entry.name === 'pre-commit') {
        hasPreCommit = true
      } else if (entry.name === 'post-checkout') {
        hasPostCheckout = true
      }
    }
    const orcaHooksCurrent = hasPreCommit && hasPostCheckout
    log.info(`git.checkHooks: repoPath=${repoPath} count=${installedHooks.length}`)
    return { jsonrpc: '2.0', id, result: { installedHooks, orcaHooksCurrent } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.ServerError,
        message: `git.checkHooks failed: read hooks dir: ${msg}`
      }
    }
  }
}

// ─── git.readIssueCommand / git.writeIssueCommand ───────────────────────────
// Well-known path orca writes/reads its issue-command config from.

const ISSUE_COMMAND_RELATIVE_PATH = path.join('.orca', 'issue-command.json')

export async function handleGitReadIssueCommand(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const repoPath = repoPathOf(params, config)
  try {
    const content = await fs.readFile(path.join(repoPath, ISSUE_COMMAND_RELATIVE_PATH), 'utf8')
    log.info(`git.readIssueCommand: repoPath=${repoPath} exists=true`)
    return { jsonrpc: '2.0', id, result: { content, exists: true } }
  } catch (err: unknown) {
    if (err instanceof Error && 'code' in err && (err as NodeJS.ErrnoException).code === 'ENOENT') {
      log.info(`git.readIssueCommand: repoPath=${repoPath} exists=false`)
      return { jsonrpc: '2.0', id, result: { content: '', exists: false } }
    }
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.ServerError,
        message: `git.readIssueCommand failed: read issue command file: ${msg}`
      }
    }
  }
}

export async function handleGitWriteIssueCommand(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const repoPath = repoPathOf(params, config)
  const content = typeof params.content === 'string' ? params.content : ''
  try {
    const dir = path.join(repoPath, '.orca')
    await fs.mkdir(dir, { recursive: true })
    await fs.writeFile(path.join(dir, 'issue-command.json'), content, 'utf8')
    log.info(`git.writeIssueCommand: repoPath=${repoPath} bytes=${content.length}`)
    return { jsonrpc: '2.0', id, result: {} }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.ServerError,
        message: `git.writeIssueCommand failed: write issue command file: ${msg}`
      }
    }
  }
}

// ─── git.scanSetupScriptImports ─────────────────────────────────────────────
// Reads .orca/setup.sh (or setup.ts/setup.js, in that preference order) and
// returns any source/import/require lines verbatim — a best-effort static
// scan, not a real shell/JS parser (matches executor.go's own caveat).

const SETUP_SCRIPT_CANDIDATES = ['setup.sh', 'setup.ts', 'setup.js']
const IMPORT_LINE_PREFIXES = ['source ', 'import ', 'require(']

export async function handleGitScanSetupScriptImports(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const repoPath = repoPathOf(params, config)
  try {
    let script: string | null = null
    for (const name of SETUP_SCRIPT_CANDIDATES) {
      try {
        script = await fs.readFile(path.join(repoPath, '.orca', name), 'utf8')
        break
      } catch (err: unknown) {
        if (
          !(
            err instanceof Error &&
            'code' in err &&
            (err as NodeJS.ErrnoException).code === 'ENOENT'
          )
        ) {
          throw err
        }
      }
    }
    if (script === null) {
      log.info(`git.scanSetupScriptImports: repoPath=${repoPath} no setup script found`)
      return { jsonrpc: '2.0', id, result: { importedPaths: [] } }
    }
    const importedPaths = script
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => IMPORT_LINE_PREFIXES.some((prefix) => line.startsWith(prefix)))
    log.info(`git.scanSetupScriptImports: repoPath=${repoPath} count=${importedPaths.length}`)
    return { jsonrpc: '2.0', id, result: { importedPaths } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.ServerError,
        message: `git.scanSetupScriptImports failed: ${msg}`
      }
    }
  }
}
