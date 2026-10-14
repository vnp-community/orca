// src/relay/agent-exec-prompt-knowledge-hub.ts
// Lets a claude run reach a project's tdd-agent knowledge graph over MCP.
// Orca sends only `knowledgeRoot`; the MCP command and tool lists below are fixed here so no
// request-controlled string becomes a process argument, a tool name or a config file.

import { lstat, mkdir, mkdtemp, realpath, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { ClaudeFlagSupport } from './agent-readonly-tool-policy'
import type { ExecPromptOptions } from './agent-exec-prompt-options'

const SERVER_NAME = 'tdd-agent'
const TOOL_PREFIX = `mcp__${SERVER_NAME}__`

const READ_TOOLS = [
  'kg_get_element',
  'kg_neighbors',
  'kg_resolve_scope',
  'kg_query',
  'kg_search',
  'kg_snapshots',
  'source_get'
] as const
const PROPOSE_TOOL = 'changeset_propose'
// Why: `--allowedTools` does not restrict MCP tools in plan mode, so every tool that can start
// a run, resume one, approve a gate or list stored proposals is also denied by name.
const BLOCKED_TOOLS = ['run_start', 'run_resume', 'approval_submit', 'proposals_list'] as const

export type KnowledgeHubErrorCode =
  | 'KNOWLEDGE_ROOT_NOT_ABSOLUTE'
  | 'KNOWLEDGE_ROOT_MISSING'
  | 'KNOWLEDGE_ROOT_NOT_DIRECTORY'
  | 'KNOWLEDGE_CONFIG_MISSING'
  | 'KNOWLEDGE_HUB_UNSUPPORTED'

export type KnowledgeHubPrepared =
  | { ok: true; args: string[]; configPath: string; cleanup: () => Promise<void> }
  | { ok: false; code: KnowledgeHubErrorCode; message: string }

export function knowledgeHubAllowedTools(proposeChangesets: boolean): string[] {
  const names: string[] = [...READ_TOOLS]
  if (proposeChangesets) {
    names.push(PROPOSE_TOOL)
  }
  return names.map((n) => TOOL_PREFIX + n)
}

export function knowledgeHubBlockedTools(proposeChangesets: boolean): string[] {
  const names: string[] = [...BLOCKED_TOOLS]
  if (!proposeChangesets) {
    names.push(PROPOSE_TOOL)
  }
  return names.map((n) => TOOL_PREFIX + n)
}

export function knowledgeHubUnsupportedReason(flags: ClaudeFlagSupport | null): string | null {
  if (flags === null) {
    return 'CLAUDE_HELP_UNAVAILABLE'
  }
  if (!flags.mcpConfig) {
    return 'FLAG_MCP_CONFIG_MISSING'
  }
  if (!flags.allowedTools) {
    return 'FLAG_ALLOWED_TOOLS_MISSING'
  }
  if (!flags.disallowedTools) {
    return 'FLAG_DISALLOWED_TOOLS_MISSING'
  }
  return null
}

async function checkRoot(
  root: string
): Promise<
  { ok: true; real: string } | { ok: false; code: KnowledgeHubErrorCode; message: string }
> {
  if (!path.isAbsolute(root)) {
    return {
      ok: false,
      code: 'KNOWLEDGE_ROOT_NOT_ABSOLUTE',
      message: 'knowledgeRoot must be an absolute path'
    }
  }
  let real: string
  try {
    real = await realpath(root)
  } catch {
    return { ok: false, code: 'KNOWLEDGE_ROOT_MISSING', message: 'knowledgeRoot does not exist' }
  }
  const dir = await lstat(real).catch(() => null)
  if (!dir?.isDirectory()) {
    return {
      ok: false,
      code: 'KNOWLEDGE_ROOT_NOT_DIRECTORY',
      message: 'knowledgeRoot is not a directory'
    }
  }
  // Why: lstat, not stat — a `.tdd-agent` or config.toml that is itself a symlink could point the
  // server at a graph outside the project.
  const store = await lstat(path.join(real, '.tdd-agent')).catch(() => null)
  const config = await lstat(path.join(real, '.tdd-agent', 'config.toml')).catch(() => null)
  if (!store?.isDirectory() || !config?.isFile()) {
    return {
      ok: false,
      code: 'KNOWLEDGE_CONFIG_MISSING',
      message: 'knowledgeRoot has no .tdd-agent/config.toml'
    }
  }
  return { ok: true, real }
}

/** Validates the root, writes a private MCP config and returns the claude flags that use it. */
export async function prepareKnowledgeHub(opts: {
  root: string
  proposeChangesets: boolean
  workDir: string
}): Promise<KnowledgeHubPrepared> {
  const checked = await checkRoot(opts.root)
  if (!checked.ok) {
    return checked
  }
  const base = path.join(opts.workDir, 'tmp')
  await mkdir(base, { recursive: true, mode: 0o700 })
  const dir = await mkdtemp(path.join(base, 'mcp-'))
  const configPath = path.join(dir, 'mcp.json')
  const config = {
    mcpServers: {
      [SERVER_NAME]: { command: 'tdd-agent', args: ['-C', checked.real, 'mcp', 'serve'] }
    }
  }
  try {
    await writeFile(configPath, JSON.stringify(config), { mode: 0o600 })
  } catch (err) {
    await rm(dir, { recursive: true, force: true })
    throw err
  }
  return {
    ok: true,
    configPath,
    args: [
      '--mcp-config',
      configPath,
      '--strict-mcp-config',
      '--allowedTools',
      knowledgeHubAllowedTools(opts.proposeChangesets).join(','),
      '--disallowedTools',
      knowledgeHubBlockedTools(opts.proposeChangesets).join(',')
    ],
    cleanup: () => rm(dir, { recursive: true, force: true })
  }
}

export function knowledgeHubError(
  id: string | number | null,
  method: 'agent.execPrompt' | 'agent.execPromptStream',
  code: KnowledgeHubErrorCode,
  message: string,
  detail?: string
): object {
  return {
    jsonrpc: '2.0',
    id,
    error: {
      code: AgentErrorCode.InvalidParams,
      message: `${method}: ${message} (${code})`,
      data: { reason: code, ...(detail ? { detail } : {}) }
    }
  }
}

/** Either the claude flags for the hub or the JSON-RPC error to return (nothing is left on disk on error). */
export async function resolveKnowledgeHub(
  method: 'agent.execPrompt' | 'agent.execPromptStream',
  id: string | number | null,
  options: Pick<ExecPromptOptions, 'knowledgeRoot' | 'proposeChangesets'>,
  flags: ClaudeFlagSupport | null,
  workDir: string
): Promise<{ args: string[]; cleanup: () => Promise<void> } | { error: object }> {
  const unsupported = knowledgeHubUnsupportedReason(flags)
  if (unsupported) {
    return {
      error: knowledgeHubError(
        id,
        method,
        'KNOWLEDGE_HUB_UNSUPPORTED',
        'claude cannot load the knowledge hub',
        unsupported
      )
    }
  }
  const prepared = await prepareKnowledgeHub({
    root: options.knowledgeRoot ?? '',
    proposeChangesets: options.proposeChangesets,
    workDir
  })
  if (!prepared.ok) {
    return { error: knowledgeHubError(id, method, prepared.code, prepared.message) }
  }
  return { args: prepared.args, cleanup: prepared.cleanup }
}
