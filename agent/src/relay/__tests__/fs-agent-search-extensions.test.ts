// src/relay/__tests__/fs-agent-search-extensions.test.ts
import { describe, it, expect } from 'vitest'
import { handlePreflightCheck } from '../fs-agent-search-extensions'
import type { AgentConfig } from '../agent-config'
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'

type RpcResponse<T = Record<string, unknown>> = {
  jsonrpc: string
  id: number | string | null
  result?: T
  error?: { code: number; message: string }
}

type PreflightResult = Record<string, boolean>

function makeConfig(): AgentConfig {
  return { workDir: '/tmp', toolEnv: { PATH: '/usr/bin' } } as unknown as AgentConfig
}

// ─── handlePreflightCheck ─────────────────────────────────────────────────────
describe('handlePreflightCheck', () => {
  it('returns a result object with boolean for each requested service', async () => {
    const resp = (await handlePreflightCheck(
      1,
      { services: ['github-cli', 'docker'] },
      makeConfig()
    )) as RpcResponse<PreflightResult>
    expect(resp.result && 'github-cli' in resp.result).toBe(true)
    expect(resp.result && 'docker' in resp.result).toBe(true)
    expect(typeof resp.result?.['github-cli']).toBe('boolean')
  })

  it('returns false for unknown service names', async () => {
    const resp = (await handlePreflightCheck(
      1,
      { services: ['unknown-tool-xyz-999'] },
      makeConfig()
    )) as RpcResponse<PreflightResult>
    expect(resp.result?.['unknown-tool-xyz-999']).toBe(false)
  })

  it('returns empty object for empty services array', async () => {
    const resp = (await handlePreflightCheck(
      1,
      { services: [] },
      makeConfig()
    )) as RpcResponse<PreflightResult>
    expect(Object.keys(resp.result ?? {})).toHaveLength(0)
  })

  it('handles multiple services in parallel without crash', async () => {
    const resp = (await handlePreflightCheck(
      1,
      { services: ['github-cli', 'ripgrep', 'docker', 'claude'] },
      makeConfig()
    )) as RpcResponse<PreflightResult>
    expect(Object.keys(resp.result ?? {})).toHaveLength(4)
  })
})

// ─── handlePreflightCheck — agent:preflight tracing (TASK-AG-014.2) ─────────
describe('handlePreflightCheck — agent:preflight tracing', () => {
  it('span.ok({checkedCount}) khi tất cả services khả dụng (empty list = vacuously all-ok)', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    await handlePreflightCheck(1, { services: [] }, makeConfig())
    unregister()

    const ok = events.find((e) => e.flow === 'agent:preflight' && e.level === 'ok')
    expect(ok).toBeDefined()
    expect(ok?.fields.checkedCount).toBe(0)
  })

  it('span.fail("unavailable: ...") khi có service không cài đặt', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    await handlePreflightCheck(1, { services: ['not-a-real-binary-xyz'] }, makeConfig())
    unregister()

    const fail = events.find((e) => e.flow === 'agent:preflight' && e.level === 'fail')
    expect(fail).toBeDefined()
    expect(fail?.fields.failedCount).toBe(1)
  })

  it('phân biệt agent:preflight với agent:fs (khác flow name)', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    await handlePreflightCheck(1, { services: ['ripgrep'] }, makeConfig())
    unregister()

    expect(events.every((e) => e.flow !== 'agent:fs')).toBe(true)
    expect(events.some((e) => e.flow === 'agent:preflight')).toBe(true)
  })
})
