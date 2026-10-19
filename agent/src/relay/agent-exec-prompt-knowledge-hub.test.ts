import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdir, mkdtemp, readFile, rm, stat, symlink, writeFile } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import { realpathSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import {
  knowledgeHubAllowedTools,
  knowledgeHubBlockedTools,
  knowledgeHubUnsupportedReason,
  prepareKnowledgeHub,
  resolveKnowledgeHub
} from './agent-exec-prompt-knowledge-hub'
import type { ClaudeFlagSupport } from './agent-readonly-tool-policy'

const FLAGS: ClaudeFlagSupport = {
  tools: true,
  permissionMode: true,
  disallowedTools: true,
  mcpConfig: true,
  allowedTools: true,
  streamJson: true
}

let tmp: string
let workDir: string
let root: string

beforeEach(async () => {
  tmp = await mkdtemp(path.join(tmpdir(), 'orca-kh-'))
  workDir = path.join(tmp, 'work')
  root = path.join(tmp, 'repo')
  await mkdir(workDir, { recursive: true })
  await mkdir(path.join(root, '.tdd-agent'), { recursive: true })
  await writeFile(path.join(root, '.tdd-agent', 'config.toml'), '[project]\n')
})
afterEach(async () => {
  await rm(tmp, { recursive: true, force: true })
})

describe('prepareKnowledgeHub', () => {
  it('writes one fixed tdd-agent server and returns the claude flags', async () => {
    const res = await prepareKnowledgeHub({ root, proposeChangesets: false, workDir })
    expect(res.ok).toBe(true)
    if (!res.ok) {
      return
    }
    const config = JSON.parse(await readFile(res.configPath, 'utf8'))
    expect(config).toEqual({
      mcpServers: {
        'tdd-agent': { command: 'tdd-agent', args: ['-C', realpathSync(root), 'mcp', 'serve'] }
      }
    })
    expect(res.args.slice(0, 3)).toEqual(['--mcp-config', res.configPath, '--strict-mcp-config'])
    expect(res.args[3]).toBe('--allowedTools')
    expect(res.args[5]).toBe('--disallowedTools')
    expect(res.configPath.startsWith(path.join(workDir, 'tmp'))).toBe(true)
  })

  it('keeps the config private to the agent user', async () => {
    const res = await prepareKnowledgeHub({ root, proposeChangesets: false, workDir })
    if (!res.ok) {
      throw new Error('expected ok')
    }
    if (process.platform !== 'win32') {
      expect((await stat(res.configPath)).mode & 0o777).toBe(0o600)
    }
  })

  it('removes the config directory on cleanup, and cleanup can run twice', async () => {
    const res = await prepareKnowledgeHub({ root, proposeChangesets: false, workDir })
    if (!res.ok) {
      throw new Error('expected ok')
    }
    expect(existsSync(res.configPath)).toBe(true)
    await res.cleanup()
    expect(existsSync(path.dirname(res.configPath))).toBe(false)
    await expect(res.cleanup()).resolves.toBeUndefined()
  })

  it('rejects a relative root', async () => {
    const res = await prepareKnowledgeHub({ root: 'repo', proposeChangesets: false, workDir })
    expect(res).toMatchObject({ ok: false, code: 'KNOWLEDGE_ROOT_NOT_ABSOLUTE' })
  })

  it('rejects a root that does not exist', async () => {
    const res = await prepareKnowledgeHub({
      root: path.join(tmp, 'nope'),
      proposeChangesets: false,
      workDir
    })
    expect(res).toMatchObject({ ok: false, code: 'KNOWLEDGE_ROOT_MISSING' })
  })

  it('rejects a root that is a file', async () => {
    const file = path.join(tmp, 'file.txt')
    await writeFile(file, 'x')
    const res = await prepareKnowledgeHub({ root: file, proposeChangesets: false, workDir })
    expect(res).toMatchObject({ ok: false, code: 'KNOWLEDGE_ROOT_NOT_DIRECTORY' })
  })

  it('rejects a root without .tdd-agent/config.toml', async () => {
    const bare = path.join(tmp, 'bare')
    await mkdir(bare)
    const res = await prepareKnowledgeHub({ root: bare, proposeChangesets: false, workDir })
    expect(res).toMatchObject({ ok: false, code: 'KNOWLEDGE_CONFIG_MISSING' })
  })

  it.skipIf(process.platform === 'win32')(
    'rejects a .tdd-agent that is a symlink out of the project',
    async () => {
      const outside = path.join(tmp, 'outside')
      await mkdir(outside)
      await writeFile(path.join(outside, 'config.toml'), '')
      const linked = path.join(tmp, 'linked')
      await mkdir(linked)
      await symlink(outside, path.join(linked, '.tdd-agent'))
      const res = await prepareKnowledgeHub({ root: linked, proposeChangesets: false, workDir })
      expect(res).toMatchObject({ ok: false, code: 'KNOWLEDGE_CONFIG_MISSING' })
    }
  )

  it.skipIf(process.platform === 'win32')('rejects a config.toml that is a symlink', async () => {
    const other = path.join(tmp, 'other.toml')
    await writeFile(other, '')
    const linked = path.join(tmp, 'linked2')
    await mkdir(path.join(linked, '.tdd-agent'), { recursive: true })
    await symlink(other, path.join(linked, '.tdd-agent', 'config.toml'))
    const res = await prepareKnowledgeHub({ root: linked, proposeChangesets: false, workDir })
    expect(res).toMatchObject({ ok: false, code: 'KNOWLEDGE_CONFIG_MISSING' })
  })

  it.skipIf(process.platform === 'win32')(
    'uses the real path of a root reached through a symlink',
    async () => {
      const alias = path.join(tmp, 'alias')
      await symlink(root, alias)
      const res = await prepareKnowledgeHub({ root: alias, proposeChangesets: false, workDir })
      if (!res.ok) {
        throw new Error('expected ok')
      }
      const config = JSON.parse(await readFile(res.configPath, 'utf8'))
      expect(config.mcpServers['tdd-agent'].args[1]).toBe(realpathSync(root))
    }
  )

  it('does not leave a config behind when the root is refused', async () => {
    await prepareKnowledgeHub({ root: 'relative', proposeChangesets: false, workDir })
    expect(existsSync(path.join(workDir, 'tmp'))).toBe(false)
  })
})

describe('tool lists', () => {
  it('allows only read tools by default', () => {
    expect(knowledgeHubAllowedTools(false)).toEqual([
      'mcp__tdd-agent__kg_get_element',
      'mcp__tdd-agent__kg_neighbors',
      'mcp__tdd-agent__kg_resolve_scope',
      'mcp__tdd-agent__kg_query',
      'mcp__tdd-agent__kg_search',
      'mcp__tdd-agent__kg_snapshots',
      'mcp__tdd-agent__source_get'
    ])
  })

  it('adds changeset_propose only when asked, and blocks it otherwise', () => {
    expect(knowledgeHubAllowedTools(true)).toContain('mcp__tdd-agent__changeset_propose')
    expect(knowledgeHubAllowedTools(false)).not.toContain('mcp__tdd-agent__changeset_propose')
    expect(knowledgeHubBlockedTools(false)).toContain('mcp__tdd-agent__changeset_propose')
    expect(knowledgeHubBlockedTools(true)).not.toContain('mcp__tdd-agent__changeset_propose')
  })

  it('always blocks the tools that start runs or approve gates', () => {
    for (const proposals of [false, true]) {
      const blocked = knowledgeHubBlockedTools(proposals)
      for (const name of ['run_start', 'run_resume', 'approval_submit', 'proposals_list']) {
        expect(blocked).toContain(`mcp__tdd-agent__${name}`)
        expect(knowledgeHubAllowedTools(proposals)).not.toContain(`mcp__tdd-agent__${name}`)
      }
    }
  })
})

describe('knowledgeHubUnsupportedReason', () => {
  it('reports a failed probe and each missing flag', () => {
    expect(knowledgeHubUnsupportedReason(null)).toBe('CLAUDE_HELP_UNAVAILABLE')
    expect(knowledgeHubUnsupportedReason({ ...FLAGS, mcpConfig: false })).toBe(
      'FLAG_MCP_CONFIG_MISSING'
    )
    expect(knowledgeHubUnsupportedReason({ ...FLAGS, allowedTools: false })).toBe(
      'FLAG_ALLOWED_TOOLS_MISSING'
    )
    expect(knowledgeHubUnsupportedReason({ ...FLAGS, disallowedTools: false })).toBe(
      'FLAG_DISALLOWED_TOOLS_MISSING'
    )
    expect(knowledgeHubUnsupportedReason(FLAGS)).toBeNull()
  })
})

describe('resolveKnowledgeHub', () => {
  it('fails closed with KNOWLEDGE_HUB_UNSUPPORTED instead of running without MCP', async () => {
    const res = await resolveKnowledgeHub(
      'agent.execPrompt',
      1,
      { knowledgeRoot: root, proposeChangesets: false },
      { ...FLAGS, mcpConfig: false },
      workDir
    )
    expect(res).toHaveProperty('error')
    const err = (res as { error: { error: { data: { reason: string; detail: string } } } }).error
    expect(err.error.data).toEqual({
      reason: 'KNOWLEDGE_HUB_UNSUPPORTED',
      detail: 'FLAG_MCP_CONFIG_MISSING'
    })
    expect(existsSync(path.join(workDir, 'tmp'))).toBe(false)
  })

  it('returns the root error as a JSON-RPC invalid-params error', async () => {
    const res = await resolveKnowledgeHub(
      'agent.execPrompt',
      7,
      { knowledgeRoot: 'rel', proposeChangesets: false },
      FLAGS,
      workDir
    )
    expect(res).toMatchObject({
      error: { id: 7, error: { data: { reason: 'KNOWLEDGE_ROOT_NOT_ABSOLUTE' } } }
    })
  })

  it('returns claude args and a cleanup for a valid root', async () => {
    const res = await resolveKnowledgeHub(
      'agent.execPrompt',
      1,
      { knowledgeRoot: root, proposeChangesets: true },
      FLAGS,
      workDir
    )
    if ('error' in res) {
      throw new Error('expected ok')
    }
    expect(res.args.join(' ')).toContain('mcp__tdd-agent__changeset_propose')
    await res.cleanup()
  })
})
