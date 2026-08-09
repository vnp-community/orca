/**
 * Tests for ProfileAwareAgentSpawner (TDD-15) — TASK-018
 *
 * Uses mocks for ProjectServerRouter, ProfileResolver, AIProviderResolver.
 * ≥ 8 tests.
 *
 * @module main/project/__tests__/ProfileAwareAgentSpawner.test
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { ProfileAwareAgentSpawner } from '../ProfileAwareAgentSpawner'
import type { ProjectServerRouter } from '../ProjectServerRouter'
import type { ProfileResolver } from '../../profile/ProfileResolver'
import type { AIProviderResolver } from '../ProfileAwareAgentSpawner'
import type { OrcaProject, ProjectMember } from '../../../shared/project-types'
import type { ResolvedProfile } from '../../profile/OrcaProfile'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

// ── helpers ────────────────────────────────────────────────────────────────

const FAKE_PROJECT: OrcaProject = {
  id: 'proj-1',
  name: 'Test Project',
  devServerId: 'srv-1',
  repoPath: '/home/user/repo',
  defaultBranch: 'main',
  visibility: 'team',
  createdBy: 'u-1',
  createdAt: new Date(),
  updatedAt: new Date(),
}

const FAKE_MEMBER: ProjectMember = {
  projectId: 'proj-1',
  userId: 'u-1',
  role: 'owner',
  addedAt: new Date(),
}

const FAKE_DEV_SERVER = { id: 'srv-1', name: 'Dev Server', connectionType: 'direct-websocket' as const }

function makeResolvedProfile(overrides: Partial<ResolvedProfile> = {}): ResolvedProfile {
  return {
    _sources: {},
    _resolvedAt: Date.now(),
    ...overrides,
  }
}

function makeSpawner(profileOverrides: Partial<ResolvedProfile> = {}, providerResult: {
  providerId: string, modelId: string, credentials: Record<string, string>
} | null = null): {
  spawner: ProfileAwareAgentSpawner
  relayCallMock: ReturnType<typeof vi.fn>
} {
  const relayCallMock = vi.fn().mockResolvedValue({ sessionId: 'agent-sess-1' })
  const fakeRelay = { call: relayCallMock }

  const router = {
    getProjectContext: vi.fn().mockResolvedValue({
      project: FAKE_PROJECT,
      member: FAKE_MEMBER,
      devServer: FAKE_DEV_SERVER,
      resolvedProfile: makeResolvedProfile(profileOverrides),
    }),
    getRelayForProject: vi.fn().mockResolvedValue(fakeRelay),
    getProject: vi.fn().mockResolvedValue(FAKE_PROJECT),
  } as unknown as ProjectServerRouter

  const profileResolver = {
    resolve: vi.fn().mockResolvedValue(makeResolvedProfile(profileOverrides)),
    invalidate: vi.fn(),
  } as unknown as ProfileResolver

  const providerService = {
    resolveForProject: vi.fn().mockResolvedValue(providerResult),
  } as unknown as AIProviderResolver

  return { spawner: new ProfileAwareAgentSpawner(router, profileResolver, providerService), relayCallMock }
}

// ── tests ──────────────────────────────────────────────────────────────────

describe('ProfileAwareAgentSpawner', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── 1. spawn: ORCA_PROJECT_ID in env ──────────────────────────────────────

  it('spawn: sets ORCA_PROJECT_ID in relay env', async () => {
    const { spawner, relayCallMock } = makeSpawner()
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'echo hi' })
    const callArgs = relayCallMock.mock.calls[0]
    expect(callArgs[0]).toBe('agent.exec')
    expect(callArgs[1].env['ORCA_PROJECT_ID']).toBe('proj-1')
  })

  // ── 2. spawn: ORCA_USER_ID in env ─────────────────────────────────────────

  it('spawn: sets ORCA_USER_ID in relay env', async () => {
    const { spawner, relayCallMock } = makeSpawner()
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'echo hi' })
    expect(relayCallMock.mock.calls[0][1].env['ORCA_USER_ID']).toBe('u-1')
  })

  // ── 3. spawn: pathAdditions prepended to PATH ─────────────────────────────

  it('spawn: prepends pathAdditions to PATH', async () => {
    const { spawner, relayCallMock } = makeSpawner({
      shell: { pathAdditions: ['/custom/bin', '/other/bin'] }
    })
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    const path: string = relayCallMock.mock.calls[0][1].env['PATH'] ?? ''
    expect(path.startsWith('/custom/bin:/other/bin:')).toBe(true)
  })

  // ── 4. spawn: shell.envVars injected ─────────────────────────────────────

  it('spawn: injects resolvedProfile.shell.envVars into env', async () => {
    const { spawner, relayCallMock } = makeSpawner({
      shell: { envVars: { MY_KEY: 'my-value', ANOTHER: 'yes' } }
    })
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    const env = relayCallMock.mock.calls[0][1].env
    expect(env['MY_KEY']).toBe('my-value')
    expect(env['ANOTHER']).toBe('yes')
  })

  // ── 5. spawn: relay.call('agent.exec') invoked ──────────────────────────

  it('spawn: calls relay.call with agent.exec', async () => {
    const { spawner, relayCallMock } = makeSpawner()
    const result = await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'ls' })
    expect(relayCallMock).toHaveBeenCalledTimes(1)
    expect(relayCallMock.mock.calls[0][0]).toBe('agent.exec')
    expect(result.sessionId).toBe('agent-sess-1')
  })

  // ── 6. spawn: trustPreset from profile.agent passed (via ORCA_TRUST_PRESET) ──

  it('spawn: sets ORCA_REPO_PATH from project', async () => {
    const { spawner, relayCallMock } = makeSpawner({
      agent: { trustPreset: 'full' }
    })
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    const env = relayCallMock.mock.calls[0][1].env
    expect(env['ORCA_REPO_PATH']).toBe('/home/user/repo')
  })

  // ── 7. spawn: ORCA_AI_PROVIDER_ID set from provider account ──────────────

  it('spawn: sets ORCA_AI_PROVIDER_ID when provider resolved', async () => {
    const { spawner, relayCallMock } = makeSpawner({}, {
      providerId: 'anthropic',
      modelId: 'claude-3-5-sonnet',
      credentials: { ANTHROPIC_API_KEY: 'sk-test' }
    })
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    const env = relayCallMock.mock.calls[0][1].env
    expect(env['ORCA_AI_PROVIDER_ID']).toBe('anthropic')
    expect(env['ORCA_AI_MODEL_ID']).toBe('claude-3-5-sonnet')
    expect(env['ANTHROPIC_API_KEY']).toBe('sk-test')
  })

  // ── 8. spawn: workdir override used ──────────────────────────────────────

  it('spawn: uses workdir override when provided', async () => {
    const { spawner, relayCallMock } = makeSpawner()
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run', workdir: '/custom/wd' })
    expect(relayCallMock.mock.calls[0][1].workdir).toBe('/custom/wd')
  })

  // ── 9. spawn: uses project.repoPath as default workdir ───────────────────

  it('spawn: uses project.repoPath as default workdir when not specified', async () => {
    const { spawner, relayCallMock } = makeSpawner()
    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    expect(relayCallMock.mock.calls[0][1].workdir).toBe('/home/user/repo')
  })

  // ── CR-TRACE-002: agentOrch:spawn tracing (TASK-BE-002.4) ─────────────────

  function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    return { events, stop: unregister }
  }

  it('spawn() emits an agentOrch:spawn span wrapping resolve-context → resolve-provider → relay-agent-exec steps in order', async () => {
    const { spawner } = makeSpawner({}, {
      providerId: 'anthropic',
      modelId: 'claude-3-5-sonnet',
      credentials: { ANTHROPIC_API_KEY: 'sk-test' }
    })
    const { events, stop } = captureTraceEvents()

    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'echo hi' })
    stop()

    const spanEvents = events.filter((e) => e.flow === 'agentOrch:spawn')
    expect(spanEvents.map((e) => e.level === 'step' ? e.label : e.level)).toEqual([
      'start',
      'resolve-context',
      'resolve-provider',
      'relay-agent-exec',
      'ok'
    ])
  })

  it('spawn() ok() field set contains sessionId only, never env/profileEnv/credentials', async () => {
    const { spawner } = makeSpawner({}, {
      providerId: 'anthropic',
      modelId: 'claude-3-5-sonnet',
      credentials: { ANTHROPIC_API_KEY: 'sk-test' }
    })
    const { events, stop } = captureTraceEvents()

    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'echo hi' })
    stop()

    const okEvent = events.find((e) => e.flow === 'agentOrch:spawn' && e.level === 'ok')
    expect(Object.keys(okEvent?.fields ?? {})).toEqual(['sessionId'])
    // Guard: no event field across the whole span may leak env/profileEnv/credential keys.
    const allSpanFields = events
      .filter((e) => e.flow === 'agentOrch:spawn')
      .flatMap((e) => Object.entries(e.fields))
    for (const [key, value] of allSpanFields) {
      expect(key.toLowerCase()).not.toContain('env')
      expect(key.toLowerCase()).not.toContain('credential')
      expect(String(value)).not.toContain('sk-test')
    }
  })

  it('spawn() resumes span id from options.traceId when provided', async () => {
    const { spawner } = makeSpawner()
    const { events, stop } = captureTraceEvents()

    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run', traceId: 'resume-spawn-1' })
    stop()

    const spanEvents = events.filter((e) => e.flow === 'agentOrch:spawn')
    expect(spanEvents.every((e) => e.id === 'resume-spawn-1')).toBe(true)
  })

  it('spawn() sends both traceId (flat) and _trace.id (nested) to relay.call agent.exec', async () => {
    const { spawner, relayCallMock } = makeSpawner()
    const { stop } = captureTraceEvents()

    await spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    stop()

    const callParams = relayCallMock.mock.calls[0][1] as { traceId?: string; _trace?: { id: string } }
    expect(callParams.traceId).toBeTruthy()
    expect(callParams._trace?.id).toBe(callParams.traceId)
  })

  it('spawn() fail() propagates original error and projectId field on getProjectContext rejection', async () => {
    const router = {
      getProjectContext: vi.fn().mockRejectedValue(new Error('context boom')),
      getRelayForProject: vi.fn(),
    } as unknown as ProjectServerRouter
    const profileResolver = {} as unknown as ProfileResolver
    const providerService = { resolveForProject: vi.fn() } as unknown as AIProviderResolver
    const spawner = new ProfileAwareAgentSpawner(router, profileResolver, providerService)
    const { events, stop } = captureTraceEvents()

    await expect(
      spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    ).rejects.toThrow('context boom')
    stop()

    const failEvent = events.find((e) => e.flow === 'agentOrch:spawn' && e.level === 'fail')
    expect(failEvent?.fields.err).toContain('context boom')
    expect(failEvent?.fields.projectId).toBe('proj-1')
  })

  it('spawn() fail() propagates on relay.call agent.exec rejection', async () => {
    const { spawner, relayCallMock } = makeSpawner()
    relayCallMock.mockRejectedValueOnce(new Error('relay unreachable'))
    const { events, stop } = captureTraceEvents()

    await expect(
      spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    ).rejects.toThrow('relay unreachable')
    stop()

    const failEvent = events.find((e) => e.flow === 'agentOrch:spawn' && e.level === 'fail')
    expect(failEvent?.fields.err).toContain('relay unreachable')
  })

  // ── CR-TRACE-015 (TASK-BE-015.5): getProjectContext failure short-circuits ──

  it('spawn() failing at getProjectContext (resolve-context step) never reaches the relay-agent-exec step', async () => {
    const router = {
      getProjectContext: vi.fn().mockRejectedValue(new Error('context boom')),
      getRelayForProject: vi.fn(),
    } as unknown as ProjectServerRouter
    const profileResolver = {} as unknown as ProfileResolver
    const providerService = { resolveForProject: vi.fn() } as unknown as AIProviderResolver
    const spawner = new ProfileAwareAgentSpawner(router, profileResolver, providerService)
    const { events, stop } = captureTraceEvents()

    await expect(
      spawner.spawn({ projectId: 'proj-1', userId: 'u-1', command: 'run' })
    ).rejects.toThrow('context boom')
    stop()

    const spanEvents = events.filter((e) => e.flow === 'agentOrch:spawn')
    const labels = spanEvents.map((e) => (e.level === 'step' ? e.label : e.level))
    expect(labels).toEqual(['start', 'resolve-context', 'fail'])
    expect(labels).not.toContain('relay-agent-exec')
    expect(router.getRelayForProject).not.toHaveBeenCalled()
  })
})
