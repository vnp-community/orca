/**
 * ProfileResolver — cascade profile merge engine with 60s TTL cache (TDD-14)
 *
 * Merge strategy (lowest → highest priority):
 *   Company → Department → Team(s) (ascending `priority`, higher wins) → User
 *
 * A user may belong to zero, one, or many Teams at once
 * (docs/guides/user-profile-team-department-rbac.md §5.2). When two Teams
 * disagree on a field, the Team with the higher `orca_team_members.priority`
 * wins; `_sources` records the winning layer as `'team:<teamId>'` (not just
 * `'team'`) so audit can tell exactly which Team overrode a field.
 *
 * Special sections:
 *   - security: always company-locked, dept/Team/user cannot override
 *   - shell.pathAdditions: concatenated (company + dept + teams + user)
 *   - shell.envVars: merged (company base, dept/teams/user override)
 *   - mcp.servers: deduped by name (highest-priority layer wins on conflict)
 *
 * Cache: 60s TTL per userId, invalidatable individually or in bulk.
 *
 * @module main/profile/ProfileResolver
 */

import { Tracers } from '../../shared/trace/tracers'
import type { ProfileService } from './ProfileService'
import type {
  OrcaProfile,
  ResolvedProfile,
  ProfileSourceLayer,
  McpServerConfig,
  AgentProfileSection,
  EditorProfileSection,
  ShellProfileSection,
} from './OrcaProfile'

const PROFILE_TTL_MS = 60_000

/** One cascade layer feeding a scalar-section merge, in lowest→highest priority order. */
type ScalarLayer<T> = { label: ProfileSourceLayer; section: T | undefined }

/** A user's team membership, already resolved to its profile JSON. */
type TeamProfileLayer = { teamId: string; profile: OrcaProfile }

type CacheEntry = {
  resolved: ResolvedProfile
  expiresAt: number
}

export class ProfileResolver {
  private readonly cache = new Map<string, CacheEntry>()

  constructor(private readonly profileService: ProfileService) {}

  /**
   * Resolve the merged cascade profile for a user.
   * Returns cached result if within TTL; otherwise fetches all layers in parallel.
   */
  async resolve(userId: string): Promise<ResolvedProfile> {
    const span = Tracers.profileResolveFlow.start({ userId })
    const now = Date.now()
    const cached = this.cache.get(userId)
    if (cached && cached.expiresAt > now) {
      span.step('cacheCheck', { cacheHit: true })
      span.ok({ cacheHit: true })
      return cached.resolved
    }
    span.step('cacheCheck', { cacheHit: false })

    // Fetch all layers in parallel — không step riêng cho từng SELECT (CR-TRACE-000 §5)
    const [companyProfile, deptProfile, userProfile, teamProfiles] = await Promise.all([
      this.profileService.getCompanyProfileForUser(userId),
      this.profileService.getDeptProfileForUser(userId),
      this.profileService.getUserProfile(userId),
      this.profileService.getTeamProfilesForUser(userId),
    ])

    // merge() thuần in-memory — KHÔNG có span/step riêng (CR-TRACE-000 §5)
    const resolved = this.merge(
      companyProfile ?? {},
      deptProfile ?? {},
      teamProfiles,
      userProfile ?? {}
    )

    this.cache.set(userId, { resolved, expiresAt: now + PROFILE_TTL_MS })
    span.ok({ cacheHit: false, hasSecurityLock: resolved.security !== undefined })
    return resolved
  }

  /**
   * Invalidate cache.
   * - With userId: clears that user's entry only
   * - Without args: clears the entire cache
   *
   * Why no tracer here: callers (profile-rpc-handler.ts) already wrap this in
   * their own step('invalidateCache') — a second span here would double-count
   * the same logical action (CR-TRACE-000 §4, "1 tracer = 1 sub-flow").
   */
  invalidate(userId?: string): void {
    if (userId !== undefined) {
      this.cache.delete(userId)
    } else {
      this.cache.clear()
    }
  }

  // ── Merge logic ─────────────────────────────────────────────────────────────

  private merge(
    company: OrcaProfile,
    dept: OrcaProfile,
    teams: TeamProfileLayer[],
    user: OrcaProfile
  ): ResolvedProfile {
    const sources: Record<string, ProfileSourceLayer> = {}

    // ── security: company-locked, dept/Team/user cannot override ─────────────
    const security = company.security
      ? { ...company.security }
      : undefined
    if (company.security) {
      sources['security'] = 'company'
      for (const key of Object.keys(company.security)) {
        sources[`security.${key}`] = 'company'
      }
    }

    // ── agent: scalar merge, highest-priority layer with the key wins ────────
    const agent = this.mergeScalar<AgentProfileSection>(
      [
        { label: 'company', section: company.agent },
        { label: 'dept', section: dept.agent },
        ...teams.map((t) => ({ label: `team:${t.teamId}` as ProfileSourceLayer, section: t.profile.agent })),
        { label: 'user', section: user.agent },
      ],
      'agent',
      sources
    )

    // ── editor: scalar merge, highest-priority layer with the key wins ───────
    const editor = this.mergeScalar<EditorProfileSection>(
      [
        { label: 'company', section: company.editor },
        { label: 'dept', section: dept.editor },
        ...teams.map((t) => ({ label: `team:${t.teamId}` as ProfileSourceLayer, section: t.profile.editor })),
        { label: 'user', section: user.editor },
      ],
      'editor',
      sources
    )

    // ── shell: composite merge ────────────────────────────────────────────────
    const shell = this.mergeShell(company.shell, dept.shell, teams, user.shell, sources)

    // ── mcp.servers: dedup by name (highest-priority layer wins on conflict) ─
    const mcpServers = this.mergeMcpServers(
      [
        { label: 'company', servers: company.mcp?.servers },
        { label: 'dept', servers: dept.mcp?.servers },
        ...teams.map((t) => ({ label: `team:${t.teamId}` as ProfileSourceLayer, servers: t.profile.mcp?.servers })),
        { label: 'user', servers: user.mcp?.servers },
      ],
      sources
    )
    const mcp = mcpServers.length > 0 ? { servers: mcpServers } : undefined

    // ── envVars (legacy top-level): highest-priority layer with the key wins ─
    const envVars = this.mergeEnvVars(
      [
        { label: 'company', vars: company.envVars },
        { label: 'dept', vars: dept.envVars },
        ...teams.map((t) => ({ label: `team:${t.teamId}` as ProfileSourceLayer, vars: t.profile.envVars })),
        { label: 'user', vars: user.envVars },
      ],
      'envVars',
      sources
    )

    const result: ResolvedProfile = {
      _sources: sources,
      _resolvedAt: Date.now(),
    }

    if (agent) {result.agent = agent}
    if (editor) {result.editor = editor}
    if (shell) {result.shell = shell}
    if (mcp) {result.mcp = mcp}
    if (security) {result.security = security}
    if (envVars && Object.keys(envVars).length > 0) {result.envVars = envVars}

    return result
  }

  /**
   * Scalar merge: for each key in the section, the highest-priority layer
   * (last in `layers`) that defines it wins. Records the winning layer's
   * label in sources under `prefix.key`.
   */
  private mergeScalar<T extends Record<string, unknown>>(
    layers: ScalarLayer<T>[],
    prefix: string,
    sources: Record<string, ProfileSourceLayer>
  ): T | undefined {
    const allKeys = new Set<string>()
    for (const { section } of layers) {
      for (const key of Object.keys(section ?? {})) {allKeys.add(key)}
    }
    if (allKeys.size === 0) {return undefined}

    const merged: Record<string, unknown> = {}

    for (const key of allKeys) {
      // Scan from the end — last layer is highest priority.
      for (let i = layers.length - 1; i >= 0; i--) {
        const { label, section } = layers[i]
        if (section && key in section && section[key] !== undefined) {
          merged[key] = section[key]
          sources[`${prefix}.${key}`] = label
          break
        }
      }
    }

    return Object.keys(merged).length > 0 ? (merged as T) : undefined
  }

  /**
   * Shell section merge:
   * - defaultShell: scalar (highest-priority layer with a value wins)
   * - pathAdditions: concatenate (company + dept + teams + user)
   * - envVars: object merge (later layers override earlier ones)
   */
  private mergeShell(
    company: ShellProfileSection | undefined,
    dept: ShellProfileSection | undefined,
    teams: TeamProfileLayer[],
    user: ShellProfileSection | undefined,
    sources: Record<string, ProfileSourceLayer>
  ): ShellProfileSection | undefined {
    const layers: ScalarLayer<ShellProfileSection>[] = [
      { label: 'company', section: company },
      { label: 'dept', section: dept },
      ...teams.map((t) => ({ label: `team:${t.teamId}` as ProfileSourceLayer, section: t.profile.shell })),
      { label: 'user', section: user },
    ]
    if (layers.every((l) => !l.section)) {return undefined}

    const shell: ShellProfileSection = {}

    // defaultShell: scalar — highest-priority layer with a value wins
    for (let i = layers.length - 1; i >= 0; i--) {
      const { label, section } = layers[i]
      if (section?.defaultShell !== undefined) {
        shell.defaultShell = section.defaultShell
        sources['shell.defaultShell'] = label
        break
      }
    }

    // pathAdditions: concatenate all (company first, user last)
    const paths = layers.flatMap(({ section }) => section?.pathAdditions ?? [])
    if (paths.length > 0) {
      shell.pathAdditions = paths
      sources['shell.pathAdditions'] = 'user' // user is last / highest priority
    }

    // envVars: object merge
    const envVars = this.mergeEnvVars(
      layers.map(({ label, section }) => ({ label, vars: section?.envVars })),
      'shell.envVars',
      sources
    )
    if (envVars && Object.keys(envVars).length > 0) {
      shell.envVars = envVars
    }

    return Object.keys(shell).length > 0 ? shell : undefined
  }

  /**
   * Merge env var dictionaries: later layers override earlier ones.
   */
  private mergeEnvVars(
    layers: Array<{ label: ProfileSourceLayer; vars: Record<string, string> | undefined }>,
    prefix: string,
    sources: Record<string, ProfileSourceLayer>
  ): Record<string, string> | undefined {
    if (layers.every((l) => !l.vars)) {return undefined}

    const merged: Record<string, string> = {}
    for (const { vars } of layers) {
      Object.assign(merged, vars)
    }

    // Track sources for each key — later layers overwrite earlier assignments,
    // consistent with the merge order above.
    for (const { label, vars } of layers) {
      for (const key of Object.keys(vars ?? {})) {
        sources[`${prefix}.${key}`] = label
      }
    }

    return Object.keys(merged).length > 0 ? merged : undefined
  }

  /**
   * Merge MCP server lists: dedup by name, highest-priority layer wins on conflict.
   * Order: company servers first, then dept, then teams (ascending priority), then user.
   */
  private mergeMcpServers(
    layers: Array<{ label: ProfileSourceLayer; servers: McpServerConfig[] | undefined }>,
    sources: Record<string, ProfileSourceLayer>
  ): McpServerConfig[] {
    const serverMap = new Map<string, McpServerConfig>()

    for (const { label, servers } of layers) {
      for (const server of servers ?? []) {
        serverMap.set(server.name, server)
        sources[`mcp.servers.${server.name}`] = label
      }
    }

    return [...serverMap.values()]
  }
}
