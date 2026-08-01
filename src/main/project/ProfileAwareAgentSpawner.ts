/**
 * ProfileAwareAgentSpawner — Spawns agents with profile-injected environment (TDD-15)
 *
 * Combines ProjectServerRouter, ProfileResolver, and AIProviderService to:
 * 1. Resolve project context (project + dev server + merged profile)
 * 2. Inject profile shell env vars and PATH additions into agent spawn env
 * 3. Resolve the AI provider from the user's preferred model
 * 4. Send the `agent.exec` call via the project relay
 *
 * Note: AIProviderService is referenced via a minimal interface to avoid
 * circular imports with Phase 4. The interface will be fulfilled by the
 * concrete AIProviderService created in TASK-021.
 *
 * @module main/project/ProfileAwareAgentSpawner
 */

import type { ProjectServerRouter } from './ProjectServerRouter'
import type { ProfileResolver } from '../profile/ProfileResolver'

/** Minimal interface for AI provider resolution (fulfilled by Phase 4 AIProviderService) */
export interface AIProviderResolver {
  resolveForProject(
    projectId: string,
    preferredModel: string | undefined
  ): Promise<{ providerId: string; modelId: string; credentials: Record<string, string> } | null>
}

/** Options for spawning an agent in a project */
export interface AgentSpawnOptions {
  /** Target project */
  projectId: string
  /** Authenticated user requesting the spawn */
  userId: string
  /** Command/task for the agent */
  command: string
  /** Additional env vars (merged on top of profile envVars, user wins) */
  extraEnv?: Record<string, string>
  /** Working directory override (defaults to project.repoPath) */
  workdir?: string
}

/** Result returned by spawn() */
export interface AgentSpawnResult {
  /** Opaque agent session ID assigned by the relay */
  sessionId: string
  /** Resolved AI provider used for this session */
  provider?: { providerId: string; modelId: string }
}

export class ProfileAwareAgentSpawner {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly profileResolver: ProfileResolver,
    private readonly providerService: AIProviderResolver
  ) {}

  /**
   * Spawn an agent for a project with profile-injected environment.
   *
   * Steps:
   * 1. Build ProjectContext (asserts access, resolves profile)
   * 2. Compose env: profile.shell.envVars merged with extraEnv
   * 3. Prepend profile.shell.pathAdditions to PATH
   * 4. Resolve AI provider for the project
   * 5. Call agent.exec on the relay
   */
  async spawn(options: AgentSpawnOptions): Promise<AgentSpawnResult> {
    const { projectId, userId, command, extraEnv, workdir } = options

    // 1. Get project context (includes access check + merged profile)
    const ctx = await this.router.getProjectContext(projectId, userId, this.profileResolver)
    const { project, resolvedProfile } = ctx

    // 2. Compose env: profile envVars + shell.envVars + extraEnv (last wins)
    const profileEnv: Record<string, string> = {
      ...(resolvedProfile.envVars ?? {}),
      ...(resolvedProfile.shell?.envVars ?? {}),
      ...(extraEnv ?? {}),
    }

    // 3. Prepend pathAdditions to PATH
    const pathAdditions = resolvedProfile.shell?.pathAdditions ?? []
    if (pathAdditions.length > 0) {
      const currentPath = process.env['PATH'] ?? ''
      profileEnv['PATH'] = [...pathAdditions, currentPath].join(':')
    }

    // 4. Add ORCA_* context vars
    profileEnv['ORCA_PROJECT_ID'] = project.id
    profileEnv['ORCA_USER_ID'] = userId
    profileEnv['ORCA_REPO_PATH'] = project.repoPath
    profileEnv['ORCA_DEV_SERVER_ID'] = project.devServerId

    // 5. Resolve AI provider
    const preferredModel = resolvedProfile.agent?.preferredModel
    const provider = await this.providerService.resolveForProject(projectId, preferredModel)
    if (provider) {
      profileEnv['ORCA_AI_PROVIDER_ID'] = provider.providerId
      profileEnv['ORCA_AI_MODEL_ID']    = provider.modelId
      // FIX TASK-WT-002 (SECURITY): Do NOT inject raw credentials into agent env.
      // Raw API keys in process.env are visible via /proc/<pid>/environ on Linux.
      // Agent reads credentials via ORCA_ACCOUNT_ID from the credential store on Dev Server.
      profileEnv['ORCA_ACCOUNT_ID']     = provider.providerId
      // Removed: Object.assign(profileEnv, provider.credentials)
    }

    // 6. Get relay and send agent.exec
    // FIX TASK-TG-001: relay agent.exec expects { binary, args, cwd } not { command, workdir }.
    // agent-rpc-dispatch.ts:506-508 reads p.binary, p.args, p.cwd respectively.
    const relay = await this.router.getRelayForProject(projectId, userId)
    const commandParts = command.trim().split(/\s+/).filter(Boolean)
    const binary = commandParts[0] ?? ''
    const args   = commandParts.slice(1)

    const result = await relay.call('agent.exec', {
      binary,                            // was: command (wrong field name)
      args,                              // was: missing
      cwd: workdir ?? project.repoPath,  // was: workdir (wrong field name)
      env: profileEnv,
      timeoutMs: 5 * 60 * 1000,         // 5 minutes
    })

    const sessionId = (result as { sessionId?: string }).sessionId ?? randomId()

    return {
      sessionId,
      provider: provider ? { providerId: provider.providerId, modelId: provider.modelId } : undefined,
    }
  }
}

/** Fallback ID generator if relay doesn't return one */
function randomId(): string {
  return `agent-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}
