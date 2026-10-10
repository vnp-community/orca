/**
 * Steps of the "add a project, then create a worktree" repro (TASK-FE-E2E-015). Everything the user does goes through the real
 * UI; the gateway is used only to read evidence (repo.list) and to remove the throwaway repo afterwards.
 */
import { execFileSync, spawnSync } from 'node:child_process'
import { existsSync, mkdtempSync, readdirSync, realpathSync, rmSync, writeFileSync } from 'node:fs'
import { homedir, tmpdir } from 'node:os'
import { basename, dirname, join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import type { GatewayRpcClient, RpcRow } from './gateway-rpc-client'

export const REPRO_PREFIX = 'e2e-ui-repro-'
export const WORKTREE_NAME = 'test-02'
/** The error the user reported, as the UI prints it. */
export const BUG_PATTERN = /WORKTREE_CREATE_FAILED[\s\S]*chdir[\s\S]*no such file or directory/i

// Why: the dev-server agent's websocket drops any frame over 32 KiB (infra-fleet: "message too big: read limited at 32769 bytes"),
// and the folder browser lists EVERY ancestor of a typed path. A directory with ~1000 entries ($TMPDIR on macOS) therefore
// disconnects the agent; stay far below that.
const MAX_LISTING_BYTES = 12_000

function listingWeight(dir: string): number {
  try {
    return readdirSync(dir).reduce((sum, name) => sum + name.length + 70, 0)
  } catch {
    return Number.MAX_SAFE_INTEGER
  }
}

function heavyAncestors(path: string): string[] {
  const heavy: string[] = []
  for (let dir = dirname(path); ; dir = dirname(dir)) {
    if (listingWeight(dir) > MAX_LISTING_BYTES) {
      heavy.push(dir)
    }
    if (dirname(dir) === dir) {
      return heavy
    }
  }
}

export type ReproRepo = {
  path: string
  name: string
  /** Where git-gateway puts the worktree: `<repo>-<name>` next to the repo. */
  worktreePath: string
  dispose: () => string[]
}

/** A throwaway git repo that the folder browser can reach without listing a huge directory. Never a repo of the user. */
export function createReproRepo(): ReproRepo {
  const candidates = [process.env.ORCA_E2E_REPRO_BASE, tmpdir(), '/tmp', homedir()]
    .filter((c): c is string => Boolean(c) && existsSync(c as string))
    .map((c) => realpathSync(c))
  const base = candidates.find((c) => heavyAncestors(join(c, 'x')).length === 0)
  if (!base) {
    throw new Error(
      `no directory whose ancestors are small enough for the folder browser (tried ${candidates.join(', ')}); set ORCA_E2E_REPRO_BASE`
    )
  }
  // Why: the repo directory itself carries the unique prefix, so the project row in the sidebar is unambiguous.
  const path = realpathSync(mkdtempSync(join(base, REPRO_PREFIX)))
  const git = (...args: string[]): void => {
    execFileSync('git', ['-c', 'user.name=e2e', '-c', 'user.email=e2e@example.com', ...args], {
      cwd: path,
      stdio: 'ignore'
    })
  }
  git('init', '-q', '-b', 'main')
  writeFileSync(join(path, 'README.md'), 'throwaway repo of the add-project repro\n')
  git('add', '-A')
  git('commit', '-q', '-m', 'init')
  const name = basename(path)
  let disposed = false
  return {
    path,
    name,
    worktreePath: `${path}-${WORKTREE_NAME}`,
    dispose: () => {
      if (disposed) {
        return []
      }
      disposed = true
      // Why: also sweeps `<repo>-<branch>` siblings, which exist only if a fixed product created the worktree.
      for (const sibling of readdirSync(base).filter((n) => n.startsWith(`${name}-`))) {
        rmSync(join(base, sibling), { recursive: true, force: true })
      }
      rmSync(path, { recursive: true, force: true })
      return readdirSync(base).filter((n) => n.startsWith(name))
    }
  }
}

export async function openAddProjectDialog(page: Page): Promise<void> {
  // Why: the Projects header button and the empty-state button share the name; the header one comes first.
  await page.getByRole('button', { name: 'Add Project', exact: true }).first().click()
  await expect(page.getByRole('dialog').getByText('Add a project')).toBeVisible()
}

export async function chooseHostInDialog(page: Page, hostName: string): Promise<string> {
  const dialog = page.getByRole('dialog')
  const combo = dialog.getByRole('combobox')
  await combo.click()
  const escaped = hostName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  await page.getByRole('option', { name: new RegExp(escaped) }).click()
  await expect(combo).toContainText(hostName)
  return (await combo.innerText()).replace(/\s+/g, ' ').trim()
}

/** "Browse folder" -> type the repo path -> Enter -> "Select folder"; ends on the "Open host project" step with the path filled. */
export async function browseAndSelectFolder(page: Page, repoPath: string): Promise<void> {
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: /Browse folder/ }).click()
  await expect(page.getByText('Browse host filesystem')).toBeVisible()
  const filter = page.getByPlaceholder(/Type to filter or enter a path/)
  // Why: one fill = one input event = one resolve, instead of a listing per typed prefix.
  await filter.fill(`${repoPath}/`)
  await expect(page.getByText('.git', { exact: true })).toBeVisible({ timeout: 20_000 })
  await filter.press('Enter')
  await expect(page.getByText(`Opens as a project on this host · ${repoPath}`)).toBeVisible()
  await page.getByRole('button', { name: 'Select folder' }).click()
  await expect(page.getByPlaceholder('/home/user/project')).toHaveValue(repoPath)
}

export async function clickAddGitProject(page: Page, repoName: string): Promise<void> {
  await page.getByRole('button', { name: 'Add Git Project' }).click()
  await expect(
    page.locator('[data-repo-header-id]').filter({ hasText: repoName }).first()
  ).toBeVisible({ timeout: 60_000 })
  await expect(page.getByRole('dialog')).toHaveCount(0, { timeout: 30_000 })
}

/** The small "+" of the project row only shows while the row is hovered. */
export async function createWorktreeFromProjectRow(
  page: Page,
  repoName: string,
  worktreeName: string
): Promise<void> {
  await page.locator('[data-repo-header-id]').filter({ hasText: repoName }).first().hover()
  await page
    .getByRole('button', { name: `Create new worktree for ${repoName}`, exact: true })
    .click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('Create worktree').first()).toBeVisible()
  await dialog.getByPlaceholder(/Type a name/).fill(worktreeName)
  await dialog.getByRole('button', { name: /^Create worktree/ }).click()
}

export type CreateOutcome =
  | { kind: 'error'; text: string }
  | { kind: 'created' }
  | { kind: 'timeout'; text: string }

/** Waits for the error panel ("Couldn't create worktree ...") or for the worktree directory to appear on disk. */
export async function waitForCreateOutcome(
  page: Page,
  worktreePath: string,
  timeoutMs = 45_000
): Promise<CreateOutcome> {
  const deadline = Date.now() + timeoutMs
  let last = ''
  while (Date.now() < deadline) {
    last = await page
      .locator('body')
      .innerText()
      .catch(() => '')
    const m = /Couldn.t create worktree[\s\S]{0,700}?(?=\nRetry|\nDismiss|$)/.exec(last)
    if (m) {
      return { kind: 'error', text: m[0].replace(/\s+/g, ' ').trim() }
    }
    if (existsSync(worktreePath)) {
      return { kind: 'created' }
    }
    await page.waitForTimeout(500)
  }
  return { kind: 'timeout', text: last.slice(0, 300) }
}

export type RepoEvidence = {
  repoId: string
  devServerId: string
  projectId: string
  projectName: string
  projectDevServerId: string
  worktrees: string[] | null
}

/** Read-only: which project holds the repo this run added, and which dev server (if any) the repo is bound to. */
export async function readRepoEvidence(
  rpc: GatewayRpcClient,
  repoPath: string
): Promise<RepoEvidence | null> {
  const projects = await rpc.call<RpcRow[]>('project.list', {})
  for (const p of projects) {
    const { repos } = await rpc.call<{ repos?: RpcRow[] }>('repo.list', { projectId: p.id })
    const hit = (repos ?? []).find((r) => r.url === repoPath)
    if (hit) {
      const worktrees = await rpc
        .call<{ worktrees?: RpcRow[] }>('worktree.list', { repo: hit.id, projectId: p.id })
        .then((r) => (r.worktrees ?? []).map((w) => String(w.branch ?? w.path ?? w.id)))
        .catch(() => null)
      return {
        repoId: hit.id,
        devServerId: String(hit.devServerId ?? ''),
        projectId: p.id,
        projectName: String(p.name),
        projectDevServerId: String(p.devServerId ?? ''),
        worktrees
      }
    }
  }
  return null
}

/** Best-effort, read-only: the git-gateway container's own record of the failure, filtered to this run's repo. */
export function gitGatewayLogLines(repoName: string, since: Date): string[] {
  const container = process.env.ORCA_E2E_GIT_GATEWAY_CONTAINER || 'orca-go-git-gateway'
  const out = spawnSync('docker', ['logs', container, '--since', since.toISOString()], {
    encoding: 'utf8',
    timeout: 20_000,
    maxBuffer: 64 * 1024 * 1024
  })
  if (out.error || out.status !== 0) {
    return []
  }
  const lines = `${out.stdout}\n${out.stderr}`
    .split('\n')
    .filter((l) => l.includes(repoName) && /WORKTREE_CREATE_FAILED|CHECK_HOOKS/.test(l))
    .map((l) => {
      const cause = /"cause":"([^"]*)"/.exec(l)?.[1]
      const code = /"code":"(WORKTREE_CREATE_FAILED|GITGATEWAY_CHECK_HOOKS_FAILED)"/.exec(l)?.[1]
      return `${code ?? 'rpc failed'}: ${(cause ?? l).slice(0, 260)}`
    })
  // Why: the same hook-check failure repeats on every sidebar refresh; one line of each kind is the evidence.
  return [...new Set(lines)].sort(
    (a, b) => Number(b.startsWith('WORKTREE')) - Number(a.startsWith('WORKTREE'))
  )
}

/** Read-only: does the git-gateway container see the repo directory? (kept separate so a missing docker is not an error). */
export function containerSeesPath(path: string): boolean | null {
  const container = process.env.ORCA_E2E_GIT_GATEWAY_CONTAINER || 'orca-go-git-gateway'
  const out = spawnSync('docker', ['exec', container, 'test', '-d', path], { timeout: 15_000 })
  if (out.error) {
    return null
  }
  return out.status === 0
}

/** Removes only the repo of THIS run (url equals its path) from Orca, then reports anything still named like the repro. */
export async function removeReproRepoFromOrca(
  rpc: GatewayRpcClient,
  repoPath: string
): Promise<string[]> {
  const problems: string[] = []
  const projects = await rpc.call<RpcRow[]>('project.list', {})
  for (const p of projects) {
    const { repos } = await rpc.call<{ repos?: RpcRow[] }>('repo.list', { projectId: p.id })
    for (const r of (repos ?? []).filter((x) => x.url === repoPath)) {
      await rpc
        .call('repo.rm', { repoId: r.id })
        .catch((e: Error) => problems.push(`repo.rm ${r.id}: ${e.message.slice(0, 120)}`))
    }
  }
  const after = await rpc.call<RpcRow[]>('project.list', {})
  for (const p of after) {
    const { repos } = await rpc.call<{ repos?: RpcRow[] }>('repo.list', { projectId: p.id })
    for (const r of (repos ?? []).filter((x) => String(x.url).includes(REPRO_PREFIX))) {
      problems.push(`repo still registered: ${String(r.displayName)} (${String(r.url)})`)
    }
    if (String(p.name).startsWith(REPRO_PREFIX)) {
      problems.push(`project still exists: ${String(p.name)}`)
    }
  }
  return problems
}
