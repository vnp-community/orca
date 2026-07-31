/**
 * git-remote-handler-v6.ts — v6 high-level git methods (TDD-20 / Conflict C1)
 *
 * Strategy: Re-use git-remote-handler.ts exports, không fork/copy logic.
 * Được chọn khi __ORCA_GIT_V6__ = true qua git-remote-handler-index.ts.
 *
 * KHÔNG chỉnh git-remote-handler.ts (v5 baseline, 93 lines).
 *
 * @module relay/git-remote-handler-v6
 */
import { gitRemoteHandlers } from './git-remote-handler'

export type { GitExecResult } from './git-remote-handler'
export { validateGitArgs, ALLOWED_GIT_SUBCOMMANDS } from './git-remote-handler'

export const gitRemoteHandlersV6 = {
  // ── Kế thừa v5 (git.exec + git.execStream) ───────────────────────────────
  ...gitRemoteHandlers,

  // ── v6: Status & Diff ─────────────────────────────────────────────────────
  'git.status': async (params: { cwd: string; worktreePath?: string }) => {
    const raw = await gitRemoteHandlers['git.exec']({
      cwd: params.worktreePath ?? params.cwd,
      args: ['status', '--porcelain=v2', '--branch'],
    })
    return { raw: raw.stdout }
  },

  'git.diff': async (params: { cwd: string; staged?: boolean; file?: string }) => {
    const args = ['diff']
    if (params.staged) args.push('--staged')
    if (params.file) args.push('--', params.file)
    return gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
  },

  // ── v6: Stage & Commit ────────────────────────────────────────────────────
  'git.add': async (params: { cwd: string; files: string[] }) => {
    await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args: ['add', '--', ...params.files] })
    return { ok: true }
  },

  'git.restore': async (params: { cwd: string; files: string[]; staged?: boolean }) => {
    const args = ['restore']
    if (params.staged) args.push('--staged')
    args.push('--', ...params.files)
    await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
    return { ok: true }
  },

  'git.commit': async (params: { cwd: string; message: string }) => {
    const result = await gitRemoteHandlers['git.exec']({
      cwd: params.cwd,
      args: ['commit', '-m', params.message],
    })
    return { ok: true, output: result.stdout }
  },

  // ── v6: Push & Pull ───────────────────────────────────────────────────────
  'git.push': async (params: { cwd: string; remote?: string; branch?: string; force?: boolean }) => {
    const args = ['push', params.remote ?? 'origin', params.branch ?? 'HEAD']
    if (params.force) args.push('--force-with-lease')
    return gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
  },

  'git.pull': async (params: { cwd: string; remote?: string; branch?: string; rebase?: boolean }) => {
    const args = ['pull', params.remote ?? 'origin']
    if (params.branch) args.push(params.branch)
    if (params.rebase) args.push('--rebase')
    return gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
  },

  // ── v6: Branch & Checkout ─────────────────────────────────────────────────
  'git.branch.list': async (params: { cwd: string; remote?: boolean }) => {
    const args = ['branch', '--format=%(refname:short)']
    if (params.remote) args.push('-r')
    const result = await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
    return { branches: result.stdout.trim().split('\n').filter(Boolean) }
  },

  'git.checkout': async (params: { cwd: string; branch: string; create?: boolean }) => {
    const args = ['checkout']
    if (params.create) args.push('-b')
    args.push(params.branch)
    await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
    return { ok: true }
  },

  // ── v6: PR Create — PROXY xuống agent (agent owns impl) ──────────────────
  // git.pr.create KHÔNG được implement ở đây.
  // Backend RPC layer (git-remote-rpc.ts) route xuống agent-git-handler.ts (line 244).
}
