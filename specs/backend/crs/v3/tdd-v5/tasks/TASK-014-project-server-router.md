# TASK-014: ProjectServerRouter

**Phase:** 3 — Project Binding  
**Solution ref:** [SOL-V5-002](../solutions/SOL-V5-002-project-binding.md) §5  
**Prerequisite:** TASK-002 (RelayConnectionPool), TASK-013 (ProjectService)  
**Status:** ✅ DONE — 2026-07-28

---

## File cần tạo: `src/main/project/ProjectServerRouter.ts`

```typescript
import type { ProjectService } from './ProjectService'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import type { ProfileResolver } from '../profile/ProfileResolver'
import type { ProjectContext, OrcaProject } from '../../shared/project-types'

export class ProjectServerRouter {
  constructor(
    private readonly projectService: ProjectService,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool
  ) {}

  async getRelayForProject(projectId: string, userId: string): Promise<import('../dev-server/dev-server-relay-bridge').DevServerRelayBridge> {
    await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)
    if (!project) throw new Error('PROJECT_NOT_FOUND')
    const server = this.devServerManager.getServer(project.devServerId)
    if (!server) throw new Error('DEV_SERVER_NOT_FOUND')
    return this.relayPool.getOrConnect(project.devServerId, server)
  }

  async getProjectContext(projectId: string, userId: string, profileResolver: ProfileResolver): Promise<ProjectContext> {
    const member = await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)
    if (!project) throw new Error('PROJECT_NOT_FOUND')
    const devServer = this.devServerManager.getServer(project.devServerId)
    if (!devServer) throw new Error('DEV_SERVER_NOT_FOUND')
    const resolvedProfile = await profileResolver.resolve(userId)
    return { project, member, devServer, resolvedProfile }
  }

  async getProject(projectId: string): Promise<OrcaProject | null> {
    return this.projectService.get(projectId)
  }
}
```

## Acceptance Criteria

- [x] `ProjectServerRouter` class export
- [x] `getRelayForProject`: assert access → get project → get server → relay pool
- [x] `getProjectContext`: returns full context
- [x] Không TypeScript errors
