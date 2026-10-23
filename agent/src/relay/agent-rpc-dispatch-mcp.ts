// src/relay/agent-rpc-dispatch-mcp.ts
// mcp.callTool — lets a workflow step call a tool of an MCP server installed on
// this dev server (TDD-WF-AGENT-001 §7.1). Only servers and tools from the
// bridge config run; the request never names a command. Bridge problems are
// returned as a result ({ok:false}) rather than a JSON-RPC error, because the
// relay turns JSON-RPC errors into opaque gRPC errors and the workflow needs
// the code to decide between retrying and failing the step.

import * as fs from 'node:fs'
import * as fsp from 'node:fs/promises'
import * as path from 'node:path'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { McpSessionPool, type McpServerSpec } from './mcp-stdio-bridge'
import { validateWorkspace } from './agent-workspace-validation'

export const MCP_CALL_DEFAULT_TIMEOUT_MS = 60_000
export const MCP_CALL_MAX_TIMEOUT_MS = 120_000

export type McpBridgeServer = McpServerSpec & { tools: string[] }
export type McpBridgeConfig = { roots: string[]; servers: Record<string, McpBridgeServer> }

// Tools a workflow needs from tdd-agent; everything else (kg_*, changeset_propose, ...) stays out of reach of workflows.
const TDD_AGENT_TOOLS = [
  'run_start',
  'run_get',
  'run_list',
  'run_dispatch',
  'run_cancel',
  'generate_start',
  'change_start',
  'approval_submit',
  'trace_get',
  'kg_snapshots',
  'testspec_start',
  'testspec_get',
  'testspec_tcir_get'
]

// Test Script Agent (docs/integrate/TDD-REQ-TESTSCRIPT-001): its whole tool surface; it writes only to its own git worktree.
const TSA_TOOLS = ['tsa_start', 'tsa_get', 'tsa_report', 'tsa_file', 'tsa_cancel']

export function defaultBridgeConfig(workDir: string): McpBridgeConfig {
  return {
    roots: [workDir],
    servers: {
      'tdd-agent': {
        command: 'tdd-agent',
        args: ['-C', '${projectDir}', 'mcp', 'serve', '--transport', 'stdio'],
        tools: TDD_AGENT_TOOLS
      },
      'test-script-agent': {
        command: 'tsa',
        args: ['-C', '${projectDir}', 'mcp', 'serve', '--transport', 'stdio'],
        tools: TSA_TOOLS
      }
    }
  }
}

/** ORCA_MCP_BRIDGE_CONFIG (JSON file) replaces the default servers; ORCA_MCP_BRIDGE_ROOTS (path-delimited) replaces the roots. */
export function loadBridgeConfig(env: NodeJS.ProcessEnv, workDir: string): McpBridgeConfig {
  const cfg = defaultBridgeConfig(workDir)
  const file = env.ORCA_MCP_BRIDGE_CONFIG
  if (file) {
    const parsed = JSON.parse(fs.readFileSync(file, 'utf8')) as Partial<McpBridgeConfig>
    if (parsed.servers) {
      cfg.servers = parsed.servers
    }
    if (parsed.roots) {
      cfg.roots = parsed.roots
    }
  }
  if (env.ORCA_MCP_BRIDGE_ROOTS) {
    cfg.roots = env.ORCA_MCP_BRIDGE_ROOTS.split(path.delimiter).filter(Boolean)
  }
  return cfg
}

type CallParams = {
  server: string
  projectDir: string
  tool: string
  args: Record<string, unknown>
  timeoutMs: number
}
type BridgeFailure = { ok: false; error: { code: string; message: string }; durationMs: number }

function failure(code: string, message: string, started: number): BridgeFailure {
  return { ok: false, error: { code, message }, durationMs: Date.now() - started }
}

function parseParams(p: Record<string, unknown>): CallParams | string {
  const server = typeof p.server === 'string' ? p.server : ''
  const projectDir = typeof p.projectDir === 'string' ? p.projectDir : ''
  const tool = typeof p.tool === 'string' ? p.tool : ''
  if (!server || !projectDir || !tool) {
    return 'server, projectDir and tool are required'
  }
  const rawArgs = p.arguments ?? {}
  if (typeof rawArgs !== 'object' || rawArgs === null || Array.isArray(rawArgs)) {
    return 'arguments must be an object'
  }
  const timeoutMs =
    typeof p.timeoutMs === 'number'
      ? Math.min(Math.max(p.timeoutMs, 1000), MCP_CALL_MAX_TIMEOUT_MS)
      : MCP_CALL_DEFAULT_TIMEOUT_MS
  return { server, projectDir, tool, args: rawArgs as Record<string, unknown>, timeoutMs }
}

/** projectDir must be an existing git repo whose real path lies under one of the bridge roots. */
export async function checkProjectDir(
  projectDir: string,
  roots: string[]
): Promise<{ ok: true; realPath: string } | { ok: false; code: string; message: string }> {
  if (!path.isAbsolute(projectDir)) {
    return { ok: false, code: 'MCP_PROJECT_DIR_INVALID', message: 'projectDir must be absolute' }
  }
  const ws = await validateWorkspace({
    kind: 'repo_root',
    path: projectDir,
    accessMode: 'readonly',
    scratchRoots: []
  })
  if (!ws.ok) {
    return { ok: false, code: 'MCP_PROJECT_DIR_INVALID', message: `${ws.message} (${ws.code})` }
  }
  for (const root of roots) {
    const realRoot = await fsp.realpath(root).catch(() => path.resolve(root))
    const rel = path.relative(realRoot, ws.realPath)
    if (rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel))) {
      return { ok: true, realPath: ws.realPath }
    }
  }
  return {
    ok: false,
    code: 'MCP_PROJECT_DIR_OUTSIDE_WORKSPACE',
    message: `projectDir is outside the allowed roots (${roots.join(', ')})`
  }
}

function specFor(server: McpBridgeServer, projectDir: string): McpServerSpec {
  return {
    command: server.command,
    args: server.args.map((a) => a.split('${projectDir}').join(projectDir)),
    env: server.env
  }
}

export async function handleMcpCallTool(
  params: Record<string, unknown>,
  bridge: McpBridgeConfig,
  pool: McpSessionPool,
  log: AgentLogger
): Promise<object> {
  const started = Date.now()
  const parsed = parseParams(params)
  if (typeof parsed === 'string') {
    return failure('MCP_INVALID_PARAMS', parsed, started)
  }
  const server = bridge.servers[parsed.server]
  if (!server) {
    return failure(
      'MCP_SERVER_NOT_ALLOWED',
      `MCP server ${parsed.server} is not configured on this dev server`,
      started
    )
  }
  if (!server.tools.includes(parsed.tool)) {
    return failure(
      'MCP_TOOL_NOT_ALLOWED',
      `tool ${parsed.tool} of ${parsed.server} is not allowed for workflows`,
      started
    )
  }
  const dir = await checkProjectDir(parsed.projectDir, bridge.roots)
  if (!dir.ok) {
    return failure(dir.code, dir.message, started)
  }
  const outcome = await pool.call(
    parsed.server,
    specFor(server, dir.realPath),
    dir.realPath,
    parsed.tool,
    parsed.args,
    parsed.timeoutMs
  )
  const durationMs = Date.now() - started
  if (outcome.kind === 'failure') {
    log.warn?.(`mcp.callTool ${parsed.server}/${parsed.tool} failed: ${outcome.code}`)
    return { ok: false, error: { code: outcome.code, message: outcome.message }, durationMs }
  }
  log.info?.(
    `mcp.callTool ${parsed.server}/${parsed.tool} ${outcome.isError ? 'tool error' : 'ok'} in ${durationMs} ms`
  )
  if (outcome.isError) {
    const payload = outcome.payload as
      | { error?: { code?: string; message?: string; details?: unknown } }
      | string
    const err =
      typeof payload === 'object' && payload?.error
        ? payload.error
        : { code: 'MCP_TOOL_ERROR', message: String(payload).slice(0, 500) }
    return { ok: true, isError: true, result: null, error: err, durationMs }
  }
  return { ok: true, isError: false, result: outcome.payload, durationMs }
}

let sharedPool: McpSessionPool | null = null
let sharedBridge: McpBridgeConfig | null = null

export async function dispatchMcpRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  log: AgentLogger
): Promise<JsonRpcResponse | null> {
  if (rpc.method !== 'mcp.callTool') {
    return null
  }
  sharedBridge ??= loadBridgeConfig(process.env, config.workDir)
  sharedPool ??= new McpSessionPool(config.toolEnv)
  const result = await handleMcpCallTool(rpc.params ?? {}, sharedBridge, sharedPool, log)
  return { jsonrpc: '2.0', id: rpc.id, result } as JsonRpcResponse
}

/** Stop every MCP server process (agent shutdown). */
export function closeMcpBridge(): void {
  sharedPool?.closeAll()
}
