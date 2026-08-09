/**
 * ProfileResolver — 3-layer profile merge engine with 60s TTL cache (TDD-14)
 *
 * Merge strategy (lowest → highest priority):
 *   Company → Department → User
 *
 * Special sections:
 *   - security: always company-locked, user/dept cannot override
 *   - shell.pathAdditions: concatenated (company + dept + user)
 *   - shell.envVars: merged (company base, dept+user override)
 *   - mcp.servers: deduped by name (user wins on conflict)
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
  McpServerConfig,
  AgentProfileSection,
  EditorProfileSection,
  ShellProfileSection,
} from './OrcaProfile'

const PROFILE_TTL_MS = 60_000

type CacheEntry = {
  resolved: ResolvedProfile
  expiresAt: number
}

export class ProfileResolver {
  private readonly cache = new Map<string, CacheEntry>()

  constructor(private readonly profileService: ProfileService) {}

  /**
   * Resolve the merged 3-layer profile for a user.
   * Returns cached result if within TTL; otherwise fetches all 3 layers in parallel.
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

    // Fetch all 3 layers in parallel — không step riêng cho từng SELECT (CR-TRACE-000 §5)
    const [companyProfile, deptProfile, userProfile] = await Promise.all([
      this.profileService.getCompanyProfileForUser(userId),
      this.profileService.getDeptProfileForUser(userId),
      this.profileService.getUserProfile(userId),
    ])

    // merge() thuần in-memory — KHÔNG có span/step riêng (CR-TRACE-000 §5)
    const resolved = this.merge(
      companyProfile ?? {},
      deptProfile ?? {},
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
    user: OrcaProfile
  ): ResolvedProfile {
    const sources: Record<string, 'company' | 'dept' | 'user'> = {}

    // ── security: company-locked, user/dept cannot override ──────────────────
    const security = company.security
      ? { ...company.security }
      : undefined
    if (company.security) {
      sources['security'] = 'company'
      for (const key of Object.keys(company.security)) {
        sources[`security.${key}`] = 'company'
      }
    }

    // ── agent: scalar merge user > dept > company per-field ──────────────────
    const agent = this.mergeScalar<AgentProfileSection>(
      company.agent,
      dept.agent,
      user.agent,
      'agent',
      sources
    )

    // ── editor: scalar merge user > dept > company per-field ─────────────────
    const editor = this.mergeScalar<EditorProfileSection>(
      company.editor,
      dept.editor,
      user.editor,
      'editor',
      sources
    )

    // ── shell: composite merge ────────────────────────────────────────────────
    const shell = this.mergeShell(company.shell, dept.shell, user.shell, sources)

    // ── mcp.servers: dedup by name (user wins on conflict) ───────────────────
    const mcpServers = this.mergeMcpServers(
      company.mcp?.servers,
      dept.mcp?.servers,
      user.mcp?.servers,
      sources
    )
    const mcp = mcpServers.length > 0 ? { servers: mcpServers } : undefined

    // ── envVars (legacy top-level): user > dept > company ────────────────────
    const envVars = this.mergeEnvVars(
      company.envVars,
      dept.envVars,
      user.envVars,
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
   * Scalar merge: for each key in the section, user > dept > company.
   * Records the winning layer in sources under `prefix.key`.
   */
  private mergeScalar<T extends Record<string, unknown>>(
    company: T | undefined,
    dept: T | undefined,
    user: T | undefined,
    prefix: string,
    sources: Record<string, 'company' | 'dept' | 'user'>
  ): T | undefined {
    if (!company && !dept && !user) {return undefined}

    const merged: Record<string, unknown> = {}

    // Collect all keys across layers
    const allKeys = new Set<string>([
      ...Object.keys(company ?? {}),
      ...Object.keys(dept ?? {}),
      ...Object.keys(user ?? {}),
    ])

    for (const key of allKeys) {
      if (user && key in user && user[key] !== undefined) {
        merged[key] = user[key]
        sources[`${prefix}.${key}`] = 'user'
      } else if (dept && key in dept && dept[key] !== undefined) {
        merged[key] = dept[key]
        sources[`${prefix}.${key}`] = 'dept'
      } else if (company && key in company && company[key] !== undefined) {
        merged[key] = company[key]
        sources[`${prefix}.${key}`] = 'company'
      }
    }

    return Object.keys(merged).length > 0 ? (merged as T) : undefined
  }

  /**
   * Shell section merge:
   * - defaultShell: scalar (user > dept > company)
   * - pathAdditions: concatenate (company + dept + user)
   * - envVars: object merge (user overrides dept overrides company)
   */
  private mergeShell(
    company: ShellProfileSection | undefined,
    dept: ShellProfileSection | undefined,
    user: ShellProfileSection | undefined,
    sources: Record<string, 'company' | 'dept' | 'user'>
  ): ShellProfileSection | undefined {
    if (!company && !dept && !user) {return undefined}

    const shell: ShellProfileSection = {}

    // defaultShell: scalar
    if (user?.defaultShell !== undefined) {
      shell.defaultShell = user.defaultShell
      sources['shell.defaultShell'] = 'user'
    } else if (dept?.defaultShell !== undefined) {
      shell.defaultShell = dept.defaultShell
      sources['shell.defaultShell'] = 'dept'
    } else if (company?.defaultShell !== undefined) {
      shell.defaultShell = company.defaultShell
      sources['shell.defaultShell'] = 'company'
    }

    // pathAdditions: concatenate all (company first, user last)
    const paths: string[] = [
      ...(company?.pathAdditions ?? []),
      ...(dept?.pathAdditions ?? []),
      ...(user?.pathAdditions ?? []),
    ]
    if (paths.length > 0) {
      shell.pathAdditions = paths
      sources['shell.pathAdditions'] = 'user' // user is last / highest priority
    }

    // envVars: object merge
    const envVars = this.mergeEnvVars(
      company?.envVars,
      dept?.envVars,
      user?.envVars,
      'shell.envVars',
      sources
    )
    if (envVars && Object.keys(envVars).length > 0) {
      shell.envVars = envVars
    }

    return Object.keys(shell).length > 0 ? shell : undefined
  }

  /**
   * Merge env var dictionaries: user overrides dept overrides company.
   */
  private mergeEnvVars(
    company: Record<string, string> | undefined,
    dept: Record<string, string> | undefined,
    user: Record<string, string> | undefined,
    prefix: string,
    sources: Record<string, 'company' | 'dept' | 'user'>
  ): Record<string, string> | undefined {
    if (!company && !dept && !user) {return undefined}

    const merged: Record<string, string> = {
      ...company,
      ...dept,
      ...user,
    }

    // Track sources for each key
    for (const key of Object.keys(company ?? {})) {
      if (!sources[`${prefix}.${key}`]) {sources[`${prefix}.${key}`] = 'company'}
    }
    for (const key of Object.keys(dept ?? {})) {
      sources[`${prefix}.${key}`] = 'dept'
    }
    for (const key of Object.keys(user ?? {})) {
      sources[`${prefix}.${key}`] = 'user'
    }

    return Object.keys(merged).length > 0 ? merged : undefined
  }

  /**
   * Merge MCP server lists: dedup by name, user wins on conflict.
   * Order: company servers first, then dept, then user (user overrides).
   */
  private mergeMcpServers(
    company: McpServerConfig[] | undefined,
    dept: McpServerConfig[] | undefined,
    user: McpServerConfig[] | undefined,
    sources: Record<string, 'company' | 'dept' | 'user'>
  ): McpServerConfig[] {
    const serverMap = new Map<string, McpServerConfig>()

    // Company servers first (lowest priority)
    for (const server of company ?? []) {
      serverMap.set(server.name, server)
      sources[`mcp.servers.${server.name}`] = 'company'
    }
    // Dept overrides company
    for (const server of dept ?? []) {
      serverMap.set(server.name, server)
      sources[`mcp.servers.${server.name}`] = 'dept'
    }
    // User overrides dept+company
    for (const server of user ?? []) {
      serverMap.set(server.name, server)
      sources[`mcp.servers.${server.name}`] = 'user'
    }

    return [...serverMap.values()]
  }
}
