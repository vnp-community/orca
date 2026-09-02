// src/relay/agent-preflight-handler.ts
// Part A (direct-websocket/relay-websocket) implementations of
// preflight.detectAgents / detectWindowsTerminalCapabilities /
// detectGhosttyConfig / setGitIdentity.
//
// Why this file exists: these four methods previously only existed on Part B
// (relay.ts's PreflightHandler, used by relay-ssh). Backend callers
// (onboarding-ipc.ts, dev-server-relay-bridge.ts's detectAgents()) reach the
// agent via a raw relay.call() that's dispatched against whichever
// connection type a given Dev Server actually uses — Part A being the
// *default* mode meant onboarding's git-identity setup, Ghostty detection,
// Windows terminal capability probing, and installed-CLI detection were all
// broken for most users. See specs/agent/api/gaps-and-findings.md #5 (the
// compliance-audit-2026-08-15.md follow-up).
//
// Reuses preflight-handler.ts's (Part B) already-transport-agnostic free
// functions (isCommandOnPathForRelay) and agent/src/main's shared
// pwsh/wsl/git-bash probes rather than duplicating that logic — only the
// per-connection git-identity storage differs from Part B (see
// git-identity-registry.ts's "Part A variant" section for why).

import type WebSocket from 'ws'
import { homedir } from 'node:os'
import path from 'node:path'
import { existsSync } from 'node:fs'
import { stat } from 'node:fs/promises'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { isCommandOnPathForRelay } from './preflight-handler'
import { setConnectionGitIdentity } from './git-identity-registry'
import { isPwshAvailable } from '../main/pwsh'
import { isWslAvailable, listWslDistros } from '../main/wsl'
import { isGitBashAvailable } from '../main/git-bash'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const execFileAsync = promisify(execFile)
const preflightTracer = createTracer('agent:preflight-a')

type AgentDetectionRuntime = NodeJS.Platform | 'wsl'

type AgentDetectionCommand = {
  id: string
  cmd: string
  requiredCommands?: readonly string[]
  unsupportedRuntimes?: readonly AgentDetectionRuntime[]
}

function isDetectionUnsupportedInRuntime(
  command: AgentDetectionCommand,
  runtime: AgentDetectionRuntime
): boolean {
  return command.unsupportedRuntimes?.includes(runtime) === true
}

// ─── preflight.detectAgents ─────────────────────────────────────────────────

export async function handleDetectAgents(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const span = preflightTracer.start({ method: 'preflight.detectAgents' })
  const commands = params.commands as AgentDetectionCommand[]
  if (!Array.isArray(commands)) {
    span.ok({ agents: 0 })
    return { jsonrpc: '2.0', id, result: { agents: [], platform: process.platform } }
  }

  const probeCommands = [
    ...new Set(
      commands
        .filter((command) => !isDetectionUnsupportedInRuntime(command, process.platform))
        .flatMap((command) => [command.cmd, ...(command.requiredCommands ?? [])])
    )
  ]

  const results = await Promise.all(
    probeCommands.map(async (cmd) => ({ cmd, installed: await isCommandOnPathForRelay(cmd) }))
  )
  const foundCommands = new Set(results.filter((r) => r.installed).map(({ cmd }) => cmd))

  const agents = [
    ...new Set(
      commands
        .filter(
          (command) =>
            !isDetectionUnsupportedInRuntime(command, process.platform) &&
            foundCommands.has(command.cmd) &&
            (command.requiredCommands ?? []).every((required) => foundCommands.has(required))
        )
        .map(({ id: agentId }) => agentId)
    )
  ]

  span.ok({ agents: agents.length })
  return { jsonrpc: '2.0', id, result: { agents, platform: process.platform } }
}

// ─── preflight.detectWindowsTerminalCapabilities ────────────────────────────

async function checkPwsh(): Promise<{ pwshAvailable: boolean; pwshVersion?: string }> {
  if (!isPwshAvailable()) {return { pwshAvailable: false }}
  try {
    const { stdout } = await execFileAsync('pwsh', ['--version'])
    return { pwshAvailable: true, pwshVersion: stdout.trim() }
  } catch {
    return { pwshAvailable: true }
  }
}

async function checkGitBash(): Promise<{ gitBashAvailable: boolean; gitBashPath?: string }> {
  if (!isGitBashAvailable()) {return { gitBashAvailable: false }}
  const candidates = [
    'C:\\Program Files\\Git\\bin\\bash.exe',
    'C:\\Program Files (x86)\\Git\\bin\\bash.exe'
  ]
  for (const candidate of candidates) {
    try {
      await stat(candidate)
      return { gitBashAvailable: true, gitBashPath: candidate }
    } catch {
      /* not found at this path, try next */
    }
  }
  return { gitBashAvailable: true }
}

export async function handleDetectWindowsTerminalCapabilities(
  id: string | number | null
): Promise<object> {
  const span = preflightTracer.start({ method: 'preflight.detectWindowsTerminalCapabilities' })
  const [wslAvailable, pwshResult, gitBashResult] = await Promise.all([
    Promise.resolve(isWslAvailable()).catch(() => false),
    checkPwsh(),
    checkGitBash()
  ])
  const wslDistros =
    wslAvailable === true ? await Promise.resolve(listWslDistros()).catch(() => []) : []

  span.ok({ wslAvailable, pwshAvailable: pwshResult.pwshAvailable })
  return {
    jsonrpc: '2.0', id,
    result: {
      wslAvailable: wslAvailable === true,
      wslDistros,
      ...pwshResult,
      ...gitBashResult,
      hostPlatform: process.platform
    }
  }
}

// ─── host.capabilities ───────────────────────────────────────────────────────
// TASK-070: relayed by infra-fleet-service's GetHostCapabilities usecase
// (backend-go/services/infra-fleet-service/internal/usecase/get_host_capabilities.go)
// via DevServerAgentClient.Exec(ctx, devServer, "host.capabilities", nil). Reuses
// the same WSL/pwsh/git-bash probes as preflight.detectWindowsTerminalCapabilities
// above, but returns only the 4 fields decodeHostCapabilities reads — no
// pwshVersion/gitBashPath enrichment, since this method's only consumer ignores them.

export async function handleHostCapabilities(id: string | number | null): Promise<object> {
  const span = preflightTracer.start({ method: 'host.capabilities' })
  const [wslAvailable, pwshResult, gitBashResult] = await Promise.all([
    Promise.resolve(isWslAvailable()).catch(() => false),
    checkPwsh(),
    checkGitBash()
  ])
  const wslDistros =
    wslAvailable === true ? await Promise.resolve(listWslDistros()).catch(() => []) : []

  span.ok({ wslAvailable, pwshAvailable: pwshResult.pwshAvailable })
  return {
    jsonrpc: '2.0', id,
    result: {
      wslAvailable: wslAvailable === true,
      wslDistros,
      pwshAvailable: pwshResult.pwshAvailable,
      gitBashAvailable: gitBashResult.gitBashAvailable
    }
  }
}

// ─── preflight.detectGhosttyConfig ──────────────────────────────────────────

export async function handleDetectGhosttyConfig(id: string | number | null): Promise<object> {
  const home = homedir()
  const configPath = path.join(home, '.config', 'ghostty', 'config')
  const themeDir = path.join(home, '.config', 'ghostty', 'themes')
  return {
    jsonrpc: '2.0', id,
    result: {
      configPath: existsSync(configPath) ? configPath : null,
      themeDir: existsSync(themeDir) ? themeDir : null
    }
  }
}

// ─── preflight.setGitIdentity ────────────────────────────────────────────────

export async function handleSetGitIdentity(
  id: string | number | null,
  params: Record<string, unknown>,
  ws: WebSocket
): Promise<object> {
  const name = typeof params.name === 'string' ? params.name : ''
  const email = typeof params.email === 'string' ? params.email : ''
  if (!name || !email) {
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'name and email are required' }
    }
  }
  setConnectionGitIdentity(ws, { name, email })
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
