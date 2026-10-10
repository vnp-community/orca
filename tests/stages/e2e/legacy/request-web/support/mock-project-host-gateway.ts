import { expect, type Page } from '@playwright/test'
import {
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  type FakeRequestBackend
} from './mock-request-ws'

/**
 * In-memory project/repo/browse gateway for the Add Project and host re-binding specs
 * (SOL-FE-E2E-004): just enough channels for the dialog, the sidebar and the host picker.
 */
export const MAC = 'Mac Studio'
export const BOX = 'Build Box'

export function devServer(
  id: string,
  name: string,
  status: 'connected' | 'disconnected' = 'connected'
) {
  return {
    id,
    name,
    connectionType: 'direct-websocket',
    status,
    platform: 'darwin',
    arch: 'arm64',
    nodeVersion: 'v22.1.0',
    lastConnectedAt: 1,
    lastError: null,
    workspaceDir: null,
    addedAt: 1,
    capabilities: [],
    approvalStatus: 'approved',
    healthStatus: 'healthy',
    activeExecutions: 0,
    busy: false
  }
}

export type World = {
  servers: ReturnType<typeof devServer>[]
  projects: Record<string, unknown>[]
  repos: Record<string, unknown>[]
}

/** In-memory gateway: just enough project/repo/browse channels for the Add Project flow. */
export function backendFor(world: World): FakeRequestBackend {
  const backend = createFakeRequestBackend({
    'devServer.list': () => world.servers,
    'project.list': () => world.projects,
    'repo.list': (params) => ({
      repos: world.repos.filter((repo) => repo.projectId === params.projectId)
    }),
    'project.create': (params) => {
      const project = {
        id: `proj-${world.projects.length + 1}`,
        name: params.name,
        devServerId: params.devServerId ?? '',
        createdAt: world.projects.length + 1
      }
      world.projects.push(project)
      return project
    },
    'repo.add': (params) => {
      const repo = {
        id: `repo-${world.repos.length + 1}`,
        projectId: params.projectId,
        url: params.url,
        displayName: params.displayName,
        devServerId: params.devServerId ?? '',
        position: world.repos.length
      }
      world.repos.push(repo)
      return repo
    },
    'repo.rebindDevServer': (params) => {
      const repo = world.repos.find((entry) => entry.id === params.repoId)
      if (!repo) {
        throw new Error('PROJECT_REPO_NOT_FOUND')
      }
      repo.devServerId = params.newDevServerId
      return repo
    },
    'projectGroup.scanNested': () => [],
    'repo.clone': (params) => ({ worktreePath: params.destPath, defaultBranch: 'main' }),
    'repo.create': (params) => ({ path: params.destPath, defaultBranch: 'main' }),
    'devServer.browseDir': (params) => {
      const path = String(params.path)
      return {
        resolvedPath: path === '~' ? '/' : path,
        entries: [
          { name: 'srv', isDirectory: true, isSymlink: false },
          { name: '.git', isDirectory: true, isSymlink: false }
        ]
      }
    }
  })
  return backend
}

export async function openAddProject(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Add Project', exact: true }).first().click()
  await expect(
    page.getByRole('dialog').getByRole('heading', { name: 'Add a project' })
  ).toBeVisible()
}

export async function start(page: Page, world: World): Promise<FakeRequestBackend> {
  const backend = backendFor(world)
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  return backend
}
