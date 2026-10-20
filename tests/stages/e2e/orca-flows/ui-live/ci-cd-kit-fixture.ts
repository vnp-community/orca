/**
 * What the ci-cd-kit spec works on: the user's EXISTING Orca project vnp-qa-platform and its repo. Nothing is created or deleted
 * around them (no Add Project, no throwaway repo): the guard of deepseek-repo-fixture.ts protects the checkout and its refs, the new
 * worktrees and branches are KEPT, and the only Orca record the spec may close is the Request it made itself.
 */
import { execFileSync, spawnSync } from 'node:child_process'
import type { GatewayRpcClient, RpcRow } from './gateway-rpc-client'
import { CICDKIT_BRANCH, CICDKIT_PROJECT_NAME, CICDKIT_REPO } from './ci-cd-kit-backlog'
import { createDeepseekFixture, type DeepseekFixture } from './deepseek-repo-fixture'
import { connectedDevServer, type LiveProject } from './live-project-fixture'
import type { LiveStackConfig } from './live-stack-config'

export function gitIn(repo: string, args: string[]): string {
  return execFileSync('git', ['-C', repo, ...args], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
}

export type CicdKitFixture = DeepseekFixture & {
  /** The commit the starting branch pointed at before the run: every worktree of the Request must descend from it. */
  baseTip: string
}

/** The branch the user asked for must exist; if it does not the run stops before anything is spent. */
export function createCicdKitFixture(prefix: string): CicdKitFixture {
  const exists = spawnSync(
    'git',
    [
      '-C',
      CICDKIT_REPO,
      'rev-parse',
      '--verify',
      '--quiet',
      `refs/heads/${CICDKIT_BRANCH}^{commit}`
    ],
    { encoding: 'utf8' }
  )
  if (exists.status !== 0) {
    throw new Error(
      `the branch ${CICDKIT_BRANCH} no longer exists in ${CICDKIT_REPO}: the run needs it as its starting branch (stop and tell the owner)`
    )
  }
  const fx = createDeepseekFixture(prefix, { repo: CICDKIT_REPO, keepGrowth: true })
  const baseTip = gitIn(CICDKIT_REPO, ['rev-parse', `refs/heads/${CICDKIT_BRANCH}`])
  // Why: worktree creation runs `git fetch origin <branch>` and starts from FETCH_HEAD, so a remote tip that differs from the local
  // branch means the worktrees would not start from the commit the owner sees. A read of the remote; a failure here only gets noted.
  const remote = spawnSync(
    'git',
    ['-C', CICDKIT_REPO, 'ls-remote', '--heads', 'origin', CICDKIT_BRANCH],
    {
      encoding: 'utf8',
      timeout: 25_000,
      env: {
        ...process.env,
        GIT_TERMINAL_PROMPT: '0',
        GIT_SSH_COMMAND: 'ssh -o BatchMode=yes -o ConnectTimeout=15'
      }
    }
  )
  const remoteTip = remote.status === 0 ? remote.stdout.split('\t')[0].trim() : ''
  console.log(
    `[ci-cd-kit] starting branch ${CICDKIT_BRANCH} at ${baseTip.slice(0, 10)}; remote tip ${remoteTip ? remoteTip.slice(0, 10) : 'not readable'}`
  )
  if (remoteTip && remoteTip !== baseTip) {
    fx.dispose()
    throw new Error(
      `origin/${CICDKIT_BRANCH} (${remoteTip.slice(0, 10)}) differs from the local branch (${baseTip.slice(0, 10)}): worktrees would start from the remote tip; the owner must decide`
    )
  }
  return Object.assign(fx, { baseTip })
}

/** Ids of the Requests this run created: the only ones its cleanup may cancel. */
export const ownRequestIds = new Set<string>()

/**
 * The existing project of the owner, found by name and repo path. dispose() deletes nothing: it cancels only a Request of this run
 * that is still open, which the spec normally already cancelled through the UI (this is the net for a page that was closed).
 */
export async function findExistingProject(
  rpc: GatewayRpcClient,
  cfg: LiveStackConfig
): Promise<LiveProject> {
  const server = await connectedDevServer(rpc, cfg)
  const projects = await rpc.call<RpcRow[]>('project.list', {})
  const matches = projects.filter((p) => p.name === CICDKIT_PROJECT_NAME)
  let found: RpcRow | undefined
  for (const p of matches) {
    const repos = await rpc.call<{ repos?: RpcRow[] }>('repo.list', { projectId: p.id })
    if ((repos.repos ?? []).some((r) => r.url === CICDKIT_REPO && r.devServerId === server.id)) {
      found = p
    }
  }
  if (!found) {
    throw new Error(
      `no project named "${CICDKIT_PROJECT_NAME}" holds ${CICDKIT_REPO} on ${cfg.devServerName}: this spec uses the existing project and never creates one`
    )
  }
  const projectId = found.id as string
  return {
    id: projectId,
    name: CICDKIT_PROJECT_NAME,
    devServerId: server.id as string,
    dispose: async () => {
      const problems: string[] = []
      for (const id of ownRequestIds) {
        try {
          const fresh = (await rpc.call<{ request: RpcRow }>('request.get', { id })).request
          if (fresh.status !== 'completed' && fresh.status !== 'cancelled') {
            await rpc.call('request.cancel', {
              id,
              reason: 'ci-cd-kit e2e cleanup: the UI cancel did not happen',
              expectedVersion: fresh.version
            })
            problems.push(`cancelled Request ${id} through the gateway (the UI cancel did not run)`)
          }
        } catch (e) {
          problems.push(`could not close Request ${id}: ${(e as Error).message.slice(0, 120)}`)
        }
      }
      return problems
    }
  }
}
