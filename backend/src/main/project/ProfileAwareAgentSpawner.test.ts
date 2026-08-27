/**
 * Tests for ProfileAwareAgentSpawner's agent.execPrompt fix.
 *
 * spawn() used to naively split `command` (free-text, e.g. a markdown task
 * prompt from buildTaskAgentPrompt()) on whitespace to fake a {binary, args}
 * pair for relay.call('agent.exec', ...) — producing nonsense like
 * binary="#" for a markdown heading. agent.execPrompt takes the prompt
 * as-is and resolves binary/args agent-side. See
 * specs/agent/api/gaps-and-findings.md.
 *
 * @module main/project/ProfileAwareAgentSpawner.test
 */

import { describe, it, expect, vi } from 'vitest'
import { ProfileAwareAgentSpawner, type AIProviderResolver } from './ProfileAwareAgentSpawner'
import type { ProjectServerRouter } from './ProjectServerRouter'
import type { ProfileResolver } from '../profile/ProfileResolver'

function makeRouter(relayCall: ReturnType<typeof vi.fn>): ProjectServerRouter {
  return {
    getProjectContext: vi.fn().mockResolvedValue({
      project: { id: 'proj-1', repoPath: '/repo', devServerId: 'dev-1' },
      member: { userId: 'user-1' },
      devServer: { id: 'dev-1' },
      resolvedProfile: { envVars: {}, shell: undefined, agent: undefined }
    }),
    getRelayForProject: vi.fn().mockResolvedValue({ call: relayCall })
  } as unknown as ProjectServerRouter
}

const PROFILE_RESOLVER = {} as ProfileResolver

describe('ProfileAwareAgentSpawner — agent.execPrompt', () => {
  it('spawn() calls relay.call("agent.execPrompt", { prompt: command, ... }), not "agent.exec"', async () => {
    const relayCall = vi.fn().mockResolvedValue({})
    const providerService: AIProviderResolver = { resolveForProject: vi.fn().mockResolvedValue(null) }
    const spawner = new ProfileAwareAgentSpawner(makeRouter(relayCall), PROFILE_RESOLVER, providerService)

    await spawner.spawn({
      projectId: 'proj-1',
      userId: 'user-1',
      command: '# Task: Fix login bug\n\n## Description\nDo the thing.'
    })

    expect(relayCall).toHaveBeenCalledWith(
      'agent.execPrompt',
      expect.objectContaining({
        prompt: '# Task: Fix login bug\n\n## Description\nDo the thing.',
        worktreePath: '/repo'
      })
    )
    expect(relayCall).not.toHaveBeenCalledWith('agent.exec', expect.anything())
    // The old bug: command.split(/\s+/)[0] === '#' became "binary". Confirm
    // no such field is sent anymore.
    const params = relayCall.mock.calls[0]?.[1] as Record<string, unknown>
    expect('binary' in params).toBe(false)
    expect('args' in params).toBe(false)
  })

  it('forwards the resolved provider as model/accountId', async () => {
    const relayCall = vi.fn().mockResolvedValue({})
    const providerService: AIProviderResolver = {
      resolveForProject: vi.fn().mockResolvedValue({
        providerId: 'acct-1',
        modelId: 'claude-opus-4-5',
        credentials: {}
      })
    }
    const spawner = new ProfileAwareAgentSpawner(makeRouter(relayCall), PROFILE_RESOLVER, providerService)

    await spawner.spawn({ projectId: 'proj-1', userId: 'user-1', command: 'do it' })

    expect(relayCall).toHaveBeenCalledWith(
      'agent.execPrompt',
      expect.objectContaining({ model: 'claude-opus-4-5', accountId: 'acct-1' })
    )
  })

  it('uses the workdir override instead of project.repoPath when provided', async () => {
    const relayCall = vi.fn().mockResolvedValue({})
    const providerService: AIProviderResolver = { resolveForProject: vi.fn().mockResolvedValue(null) }
    const spawner = new ProfileAwareAgentSpawner(makeRouter(relayCall), PROFILE_RESOLVER, providerService)

    await spawner.spawn({
      projectId: 'proj-1',
      userId: 'user-1',
      command: 'do it',
      workdir: '/custom/worktree'
    })

    expect(relayCall).toHaveBeenCalledWith(
      'agent.execPrompt',
      expect.objectContaining({ worktreePath: '/custom/worktree' })
    )
  })
})
