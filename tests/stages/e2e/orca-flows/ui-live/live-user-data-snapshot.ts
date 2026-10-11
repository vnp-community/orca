/**
 * Fingerprint of the user's own Orca data (every project, repo and finished Request that exists before the run) and the list of
 * their repos on this machine, so the spec can prove it left them alone. Read-only. Requests still in flight are not compared:
 * they may move by themselves while the run lasts (a plan being written); finished ones must not change at all.
 */
import { existsSync } from 'node:fs'
import { join } from 'node:path'
import type { GatewayRpcClient, RpcRow } from './gateway-rpc-client'
import { readInventory } from './live-add-project-ledger'
import { fingerprintRepo } from './live-safety-invariants'

export type UserDataSnapshot = {
  /** project id -> what the project and its repos looked like */
  projects: Record<string, string>
  /** finished Request id -> status/version/updatedAt */
  finishedRequests: Record<string, string>
  /** Repos of the user on this dev server that exist on disk with a .git: handed to the git fingerprint of the safety net. */
  repoPaths: string[]
}

export async function snapshotUserData(
  rpc: GatewayRpcClient,
  devServerId: string
): Promise<UserDataSnapshot> {
  const projectRows = await rpc.call<RpcRow[]>('project.list', {})
  const rowById = new Map(projectRows.map((p) => [p.id, p]))
  const snapshot: UserDataSnapshot = { projects: {}, finishedRequests: {}, repoPaths: [] }
  for (const p of await readInventory(rpc)) {
    const row = rowById.get(p.id)
    snapshot.projects[p.id] = JSON.stringify({
      name: p.name,
      devServerId: p.devServerId,
      updatedAt: row?.updatedAt,
      repos: p.repos.map((r) => [r.id, r.url, r.displayName, r.devServerId])
    })
    for (const r of p.repos) {
      if (r.devServerId === devServerId && existsSync(join(r.url, '.git'))) {
        snapshot.repoPaths.push(r.url)
      }
    }
    const listed = await rpc.call<{ requests?: RpcRow[] }>('request.list', { projectId: p.id })
    for (const r of listed.requests ?? []) {
      if (r.status === 'completed' || r.status === 'cancelled') {
        snapshot.finishedRequests[r.id] =
          `#${String(r.number)} ${r.status} v${r.version} ${String(r.updatedAt)}`
      }
    }
  }
  snapshot.repoPaths = [...new Set(snapshot.repoPaths)]
  return snapshot
}

/** Differences that mean the run touched data it does not own; empty when every pre-existing record is as it was. */
export function diffUserData(before: UserDataSnapshot, after: UserDataSnapshot): string[] {
  const problems: string[] = []
  for (const [id, was] of Object.entries(before.projects)) {
    const now = after.projects[id]
    if (now === undefined) {
      problems.push(`project ${was.slice(0, 80)} is gone`)
    } else if (now !== was) {
      problems.push(`project ${id} changed: ${was.slice(0, 120)} -> ${now.slice(0, 120)}`)
    }
  }
  for (const id of Object.keys(after.projects)) {
    if (!(id in before.projects)) {
      problems.push(
        `a project the run did not clean up is left: ${after.projects[id].slice(0, 120)}`
      )
    }
  }
  for (const [id, was] of Object.entries(before.finishedRequests)) {
    if (after.finishedRequests[id] !== was) {
      problems.push(
        `finished Request ${id} changed: ${was} -> ${after.finishedRequests[id] ?? 'missing'}`
      )
    }
  }
  return problems
}

/** One line per fact worth showing in the run log: what was guarded, so "unchanged" is visible and not only asserted. */
export function describeUserData(snapshot: UserDataSnapshot): string[] {
  const names = Object.values(snapshot.projects).map(
    (p) => (JSON.parse(p) as { name: string }).name
  )
  const lines = [
    `${names.length} project(s): ${names.map((n) => `"${n}"`).join(', ')}`,
    `${Object.keys(snapshot.finishedRequests).length} finished Request(s): ${Object.values(snapshot.finishedRequests).join(' | ')}`
  ]
  for (const path of snapshot.repoPaths) {
    const fp = fingerprintRepo(path, true)
    lines.push(
      `repo ${path}: HEAD ${fp.head.slice(0, 10)}, ${fp.worktreeList.length} worktree(s), ${Object.keys(fp.files).length} changed/untracked file(s)`
    )
  }
  return lines
}
