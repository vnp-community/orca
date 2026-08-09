/**
 * ProjectServerRouter — Routes project operations to the correct dev server relay (TDD-15)
 *
 * Bridges ProjectService, DevServerManager, and RelayConnectionPool to provide
 * project-scoped relay access and full project context for agent spawners.
 *
 * @module main/project/ProjectServerRouter
 */

import type { ProjectService } from './ProjectService'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import type { ProfileResolver } from '../profile/ProfileResolver'
import type { ProjectContext, OrcaProject } from '../../shared/project-types'
import type { DevServerRelayBridge } from '../dev-server/dev-server-relay-bridge'
import { Tracers } from '../../shared/trace/tracers'

export class ProjectServerRouter {
  constructor(
    private readonly projectService: ProjectService,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool
  ) {}

  /**
   * Assert access, resolve the project's dev server, and return an active relay.
   * The relay is pooled — calling twice for the same devServerId returns the same bridge.
   *
   * @throws PROJECT_NOT_FOUND   — project doesn't exist
   * @throws DEV_SERVER_NOT_FOUND — dev server unknown or disconnected
   * @throws PROJECT_ACCESS_DENIED — userId is not a member (from assertAccess)
   */
  async getRelayForProject(projectId: string, userId: string): Promise<DevServerRelayBridge> {
    const span = Tracers.profileProjectRouteFlow.start({ op: 'getRelay', projectId })
    try {
      await this.projectService.assertAccess(projectId, userId)
      const project = await this.projectService.get(projectId)
      if (!project) {
        span.fail('PROJECT_NOT_FOUND', { op: 'getRelay', projectId })
        throw new Error('PROJECT_NOT_FOUND')
      }
      const server = this.devServerManager.get(project.devServerId)
      if (!server) {
        span.fail('DEV_SERVER_NOT_FOUND', { op: 'getRelay', projectId })
        throw new Error('DEV_SERVER_NOT_FOUND')
      }
      const relay = this.relayPool.getOrConnect(project.devServerId, server)
      span.ok({ op: 'getRelay', projectId, devServerId: project.devServerId })
      return relay
    } catch (err) {
      span.fail(err, { op: 'getRelay', projectId })
      throw err
    }
  }

  /**
   * Build a complete ProjectContext for agent spawn / relay invocation.
   * Fetches project, asserts membership, resolves the merged profile in parallel.
   *
   * @throws PROJECT_NOT_FOUND   — project doesn't exist
   * @throws DEV_SERVER_NOT_FOUND — dev server unknown
   * @throws PROJECT_ACCESS_DENIED — userId is not a member
   */
  async getProjectContext(
    projectId: string,
    userId: string,
    profileResolver: ProfileResolver
  ): Promise<ProjectContext> {
    const member = await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)
    if (!project) {throw new Error('PROJECT_NOT_FOUND')}
    const devServer = this.devServerManager.get(project.devServerId)
    if (!devServer) {throw new Error('DEV_SERVER_NOT_FOUND')}
    const resolvedProfile = await profileResolver.resolve(userId)
    return { project, member, devServer, resolvedProfile }
  }

  /**
   * Simple project lookup (no access check) — for admin/internal callers.
   */
  async getProject(projectId: string): Promise<OrcaProject | null> {
    return this.projectService.get(projectId)
  }
}
