import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { execFileSync } from 'node:child_process'
import * as fs from 'node:fs'
import * as os from 'node:os'
import * as path from 'node:path'
import type { AgentLogger } from './agent-logger'
import {
  checkProjectDir,
  handleMcpCallTool,
  loadBridgeConfig,
  type McpBridgeConfig
} from './agent-rpc-dispatch-mcp'
import { AGENT_FEATURES } from './agent-protocol-features'
import { McpSessionPool, parseToolResult } from './mcp-stdio-bridge'

// Loose view of the result for assertions.
type McpCallResult = { ok: boolean; result: { pid: number; calls: number } }

const log: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

// A tiny stdio MCP server: answers initialize, echoes tools/call, fails "boom", hangs on "slow".
const FAKE_SERVER = `
const rl = require('node:readline').createInterface({ input: process.stdin })
let calls = 0
rl.on('line', (line) => {
  const msg = JSON.parse(line)
  if (msg.id === undefined) return
  const reply = (result) => process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: msg.id, result }) + '\\n')
  if (msg.method === 'initialize') return reply({ protocolVersion: msg.params.protocolVersion, capabilities: {}, serverInfo: { name: 'fake' } })
  if (msg.method !== 'tools/call') return
  calls += 1
  const { name, arguments: args } = msg.params
  if (name === 'slow') return
  if (name === 'boom') return reply({ isError: true, content: [{ type: 'text', text: JSON.stringify({ error: { code: 'RUN_NOT_FOUND', message: 'no run' } }) }] })
  reply({ content: [{ type: 'text', text: JSON.stringify({ tool: name, args, cwd: process.cwd(), pid: process.pid, calls }) }] })
})
`

let root: string
let repo: string
let serverJs: string

beforeAll(() => {
  root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'mcp-bridge-')))
  repo = path.join(root, 'projects', 'repo')
  fs.mkdirSync(repo, { recursive: true })
  execFileSync('git', ['init', '-q'], { cwd: repo })
  serverJs = path.join(root, 'fake-mcp.js')
  fs.writeFileSync(serverJs, FAKE_SERVER)
})

function bridge(tools = ['run_get', 'boom', 'slow']): McpBridgeConfig {
  return {
    roots: [path.join(root, 'projects')],
    servers: {
      fake: { command: process.execPath, args: [serverJs, '--project', '${projectDir}'], tools }
    }
  }
}

const pools: McpSessionPool[] = []
function pool(idleMs?: number): McpSessionPool {
  const p = new McpSessionPool(process.env, idleMs)
  pools.push(p)
  return p
}
afterEach(() => {
  pools.splice(0).forEach((p) => p.closeAll())
})

describe('mcp.callTool', () => {
  it('advertises the feature', () => {
    expect(AGENT_FEATURES).toContain('mcp.callTool')
  })

  it('calls an allowlisted tool in the project directory and reuses the server process', async () => {
    const p = pool()
    const first = (await handleMcpCallTool(
      { server: 'fake', projectDir: repo, tool: 'run_get', arguments: { run_id: 'r1' } },
      bridge(),
      p,
      log
    )) as McpCallResult
    expect(first).toMatchObject({
      ok: true,
      isError: false,
      result: { tool: 'run_get', args: { run_id: 'r1' }, cwd: repo, calls: 1 }
    })
    const second = (await handleMcpCallTool(
      { server: 'fake', projectDir: repo, tool: 'run_get' },
      bridge(),
      p,
      log
    )) as McpCallResult
    expect(second.result.pid).toBe(first.result.pid)
    expect(second.result.calls).toBe(2)
    expect(p.size()).toBe(1)
  })

  it('passes a tool error through with its code', async () => {
    const out = await handleMcpCallTool(
      { server: 'fake', projectDir: repo, tool: 'boom' },
      bridge(),
      pool(),
      log
    )
    expect(out).toMatchObject({
      ok: true,
      isError: true,
      result: null,
      error: { code: 'RUN_NOT_FOUND' }
    })
  })

  it('refuses servers, tools and directories outside the allowlist', async () => {
    const p = pool()
    expect(
      await handleMcpCallTool(
        { server: 'other', projectDir: repo, tool: 'run_get' },
        bridge(),
        p,
        log
      )
    ).toMatchObject({ ok: false, error: { code: 'MCP_SERVER_NOT_ALLOWED' } })
    expect(
      await handleMcpCallTool(
        { server: 'fake', projectDir: repo, tool: 'kg_query' },
        bridge(),
        p,
        log
      )
    ).toMatchObject({ ok: false, error: { code: 'MCP_TOOL_NOT_ALLOWED' } })
    expect(
      await handleMcpCallTool(
        { server: 'fake', projectDir: root, tool: 'run_get' },
        bridge(),
        p,
        log
      )
    ).toMatchObject({ ok: false, error: { code: 'MCP_PROJECT_DIR_INVALID' } })
    const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'mcp-outside-'))
    execFileSync('git', ['init', '-q'], { cwd: outside })
    expect(
      await handleMcpCallTool(
        { server: 'fake', projectDir: outside, tool: 'run_get' },
        bridge(),
        p,
        log
      )
    ).toMatchObject({ ok: false, error: { code: 'MCP_PROJECT_DIR_OUTSIDE_WORKSPACE' } })
    expect(
      await handleMcpCallTool(
        { server: 'fake', projectDir: 'relative/dir', tool: 'run_get' },
        bridge(),
        p,
        log
      )
    ).toMatchObject({ ok: false, error: { code: 'MCP_PROJECT_DIR_INVALID' } })
    expect(
      await handleMcpCallTool({ server: 'fake', tool: 'run_get' }, bridge(), p, log)
    ).toMatchObject({ ok: false, error: { code: 'MCP_INVALID_PARAMS' } })
    expect(p.size()).toBe(0)
  })

  it('reports a missing server binary as MCP_SERVER_NOT_INSTALLED', async () => {
    const cfg: McpBridgeConfig = {
      roots: [path.join(root, 'projects')],
      servers: { ghost: { command: 'definitely-not-installed-mcp', args: [], tools: ['run_get'] } }
    }
    expect(
      await handleMcpCallTool(
        { server: 'ghost', projectDir: repo, tool: 'run_get' },
        cfg,
        pool(),
        log
      )
    ).toMatchObject({ ok: false, error: { code: 'MCP_SERVER_NOT_INSTALLED' } })
  })

  it('times out a hanging tool and starts a fresh process next time', async () => {
    const p = pool()
    const slow = await handleMcpCallTool(
      { server: 'fake', projectDir: repo, tool: 'slow', timeoutMs: 1000 },
      bridge(),
      p,
      log
    )
    expect(slow).toMatchObject({ ok: false, error: { code: 'MCP_TIMEOUT' } })
    const after = (await handleMcpCallTool(
      { server: 'fake', projectDir: repo, tool: 'run_get' },
      bridge(),
      p,
      log
    )) as McpCallResult
    expect(after).toMatchObject({ ok: true, result: { calls: 1 } })
  })

  it('closes idle server processes', async () => {
    const p = pool(50)
    await handleMcpCallTool({ server: 'fake', projectDir: repo, tool: 'run_get' }, bridge(), p, log)
    expect(p.size()).toBe(1)
    await new Promise((r) => setTimeout(r, 150))
    expect(p.size()).toBe(0)
  })
})

describe('bridge config and helpers', () => {
  it('defaults to tdd-agent under the agent work dir, with an env override for the roots', () => {
    const cfg = loadBridgeConfig({}, '/srv/projects')
    expect(cfg.roots).toEqual(['/srv/projects'])
    expect(cfg.servers['tdd-agent'].args).toEqual([
      '-C',
      '${projectDir}',
      'mcp',
      'serve',
      '--transport',
      'stdio'
    ])
    expect(cfg.servers['tdd-agent'].tools).toContain('run_dispatch')
    expect(cfg.servers['tdd-agent'].tools).toEqual(
      expect.arrayContaining(['testspec_start', 'testspec_get'])
    )
    expect(cfg.servers['tdd-agent'].tools).not.toContain('changeset_propose')
    expect(
      loadBridgeConfig({ ORCA_MCP_BRIDGE_ROOTS: ['/a', '/b'].join(path.delimiter) }, '/w').roots
    ).toEqual(['/a', '/b'])
  })

  it('reads a JSON config file', () => {
    const file = path.join(root, 'bridge.json')
    fs.writeFileSync(
      file,
      JSON.stringify({ servers: { x: { command: 'x', args: [], tools: ['t'] } } })
    )
    const cfg = loadBridgeConfig({ ORCA_MCP_BRIDGE_CONFIG: file }, '/w')
    expect(Object.keys(cfg.servers)).toEqual(['x'])
    expect(cfg.roots).toEqual(['/w'])
  })

  it('accepts the root itself and rejects siblings sharing a prefix', async () => {
    const projects = path.join(root, 'projects')
    expect(await checkProjectDir(repo, [projects])).toMatchObject({ ok: true, realPath: repo })
    const sibling = path.join(root, 'projects-evil')
    fs.mkdirSync(sibling, { recursive: true })
    execFileSync('git', ['init', '-q'], { cwd: sibling })
    expect(await checkProjectDir(sibling, [projects])).toMatchObject({
      ok: false,
      code: 'MCP_PROJECT_DIR_OUTSIDE_WORKSPACE'
    })
  })

  it('parses structured and text tool results', () => {
    expect(parseToolResult({ structuredContent: { a: 1 } })).toEqual({
      isError: false,
      payload: { a: 1 }
    })
    expect(
      parseToolResult({
        isError: true,
        content: [{ type: 'text', text: '{"error":{"code":"X"}}' }]
      })
    ).toEqual({ isError: true, payload: { error: { code: 'X' } } })
    expect(parseToolResult({ content: [{ type: 'text', text: 'plain' }] })).toEqual({
      isError: false,
      payload: 'plain'
    })
  })
})
