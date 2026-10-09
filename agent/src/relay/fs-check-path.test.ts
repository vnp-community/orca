import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { handleFsCheckPath } from './fs-agent-extensions'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { AgentConfig } from './agent-config'

// project-service calls fs.checkPath to validate a repo path before binding a project to this dev server.
describe('handleFsCheckPath', () => {
  const root = mkdtempSync(join(tmpdir(), 'orca-checkpath-'))
  mkdirSync(join(root, 'repo'))
  writeFileSync(join(root, 'a-file.txt'), 'x')
  const config = { workDir: root } as unknown as AgentConfig
  const call = (params: Record<string, unknown>) => handleFsCheckPath(7, params, config) as Promise<{ id: number; result?: { exists: boolean; isDir: boolean }; error?: { code: number } }>

  it('reports an existing directory', async () => {
    expect((await call({ path: join(root, 'repo') })).result).toEqual({ exists: true, isDir: true })
  })

  it('reports an existing file as not a directory', async () => {
    expect((await call({ path: join(root, 'a-file.txt') })).result).toEqual({ exists: true, isDir: false })
  })

  it('reports a missing path as absent, not as an error', async () => {
    expect((await call({ path: join(root, 'nope') })).result).toEqual({ exists: false, isDir: false })
  })

  it('resolves a relative path against the agent work directory', async () => {
    expect((await call({ path: 'repo' })).result).toEqual({ exists: true, isDir: true })
  })

  it('rejects a call without a path', async () => {
    const reply = await call({})
    expect(reply.error?.code).toBe(AgentErrorCode.InvalidParams)
    expect(reply.result).toBeUndefined()
  })

  it('answers with the request id so the caller can match the reply', async () => {
    expect((await call({ path: root })).id).toBe(7)
  })
})
