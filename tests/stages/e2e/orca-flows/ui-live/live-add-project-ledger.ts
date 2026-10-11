/**
 * What the add-project spec created in Orca, found again by what identifies it (a project named like the run's folders, or a repo at
 * one of the run's two paths) and removed in `finally`. Nothing else is touched: the user's own projects never match, because the
 * names and paths carry the run stamp. Also the read-only inventory the spec uses to count projects and repos.
 */
import type { GatewayRpcClient, RpcRow } from './gateway-rpc-client'
import {
  cancelOpenRequests,
  connectedDevServer,
  deleteProjectWhenIdle,
  type LiveProject
} from './live-project-fixture'
import type { LiveStackConfig } from './live-stack-config'

export type RegisteredRepo = {
  id: string
  projectId: string
  url: string
  displayName: string
  devServerId: string
}

export type ProjectWithRepos = {
  id: string
  name: string
  devServerId: string
  repos: RegisteredRepo[]
}

/** Same notion of "same folder" the product uses: trailing slashes do not matter. */
export const sameFolder = (a: string, b: string): boolean =>
  a.replace(/\/+$/, '') === b.replace(/\/+$/, '')

/** Every project with its repos (read-only: project.list + repo.list). */
export async function readInventory(rpc: GatewayRpcClient): Promise<ProjectWithRepos[]> {
  const projects = await rpc.call<RpcRow[]>('project.list', {})
  const out: ProjectWithRepos[] = []
  for (const p of projects) {
    const { repos } = await rpc.call<{ repos?: RpcRow[] }>('repo.list', { projectId: p.id })
    out.push({
      id: p.id,
      name: String(p.name),
      devServerId: String(p.devServerId ?? ''),
      repos: (repos ?? []).map((r) => ({
        id: r.id,
        projectId: String(r.projectId ?? p.id),
        url: String(r.url ?? ''),
        displayName: String(r.displayName ?? ''),
        devServerId: String(r.devServerId ?? '')
      }))
    })
  }
  return out
}

export type LedgerScope = { namePrefix: string; paths: string[] }

export function ownedByRun(scope: LedgerScope, inventory: ProjectWithRepos[]): ProjectWithRepos[] {
  return inventory.filter(
    (p) =>
      p.name.startsWith(scope.namePrefix) ||
      p.repos.some((r) => scope.paths.some((path) => sameFolder(r.url, path)))
  )
}

export type AddProjectLedger = {
  /** Gives the harness a project placeholder; the spec fills id/name once the UI has created the real one. */
  provision: (rpc: GatewayRpcClient, cfg: LiveStackConfig) => Promise<LiveProject>
  /** Problems met while removing; included in the run's safety verdict. */
  problems: () => string[]
  /** What of this run is still registered in Orca (projects, repos): must be empty after cleanup. */
  leftovers: (rpc: GatewayRpcClient) => Promise<string[]>
}

export function createAddProjectLedger(scope: LedgerScope): AddProjectLedger {
  const problems: string[] = []
  return {
    problems: () => [...problems],
    provision: async (rpc, cfg) => {
      const server = await connectedDevServer(rpc, cfg)
      let disposed = false
      return {
        id: '',
        name: '',
        devServerId: server.id,
        dispose: async () => {
          if (disposed) {
            return []
          }
          disposed = true
          const mine: string[] = []
          const owned = ownedByRun(scope, await readInventory(rpc))
          // Why: open Requests first, so none is orphaned when its project goes (Requests live in another database).
          for (const p of owned.filter((x) => x.name.startsWith(scope.namePrefix))) {
            await cancelOpenRequests(rpc, p.id, mine)
          }
          for (const p of owned) {
            const ours = p.repos.filter((r) => scope.paths.some((path) => sameFolder(r.url, path)))
            for (const r of p.name.startsWith(scope.namePrefix) ? p.repos : ours) {
              await rpc
                .call('repo.rm', { repoId: r.id })
                .catch((e: Error) =>
                  mine.push(`repo.rm ${r.displayName}: ${e.message.slice(0, 120)}`)
                )
            }
            // Why: a project that only HOLDS one of our repos (not named by this run) is somebody else's: only the repo goes.
            if (p.name.startsWith(scope.namePrefix)) {
              await deleteProjectWhenIdle(rpc, p.id, mine)
            }
          }
          problems.push(...mine)
          return mine
        }
      }
    },
    leftovers: async (rpc) => {
      const left: string[] = []
      for (const p of ownedByRun(scope, await readInventory(rpc))) {
        left.push(
          `project still in Orca: "${p.name}" (${p.id}) with ${p.repos.length} repo(s): ${p.repos.map((r) => r.url).join(', ')}`
        )
      }
      return left
    }
  }
}
