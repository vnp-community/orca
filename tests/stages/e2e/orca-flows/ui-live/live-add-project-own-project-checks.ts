/**
 * Assertions of the add-project spec (TASK-FE-E2E-027), kept apart from the UI steps. Gateway reads only; the one write is the
 * deliberate duplicate probe `project.create`, which the backend must refuse.
 */
import { expect } from '@playwright/test'
import type { GatewayRpcClient } from './gateway-rpc-client'
import { REPO_FACTS, type SandboxRepo } from './add-project-two-repos-fixture'
import { readInventory, type ProjectWithRepos } from './live-add-project-ledger'
import { callsTo, type SentRpc } from './live-add-project-own-project-steps'

export type RegistrySnapshot = { projects: ProjectWithRepos[] }

export async function takeRegistry(rpc: GatewayRpcClient): Promise<RegistrySnapshot> {
  return { projects: await readInventory(rpc) }
}

const repoCount = (r: RegistrySnapshot): number =>
  r.projects.reduce((n, p) => n + p.repos.length, 0)

/** Nothing was registered between the two snapshots: same projects, same repos in each. */
export function expectRegistryUnchanged(
  before: RegistrySnapshot,
  after: RegistrySnapshot,
  what: string
): void {
  expect.soft(after.projects.length, `${what}: project count`).toBe(before.projects.length)
  expect.soft(repoCount(after), `${what}: repo count`).toBe(repoCount(before))
  expect
    .soft(
      after.projects.map((p) => `${p.id}:${p.repos.map((r) => r.id).join(',')}`).sort(),
      `${what}: same projects holding the same repos`
    )
    .toEqual(before.projects.map((p) => `${p.id}:${p.repos.map((r) => r.id).join(',')}`).sort())
}

/**
 * The folder got a project of its own: exactly one new project, named after the folder, bound to the dev server, holding exactly
 * this repo (also bound to it); no other project gained a repo. Returns that project.
 */
export function expectOwnProject(
  before: RegistrySnapshot,
  after: RegistrySnapshot,
  folder: SandboxRepo,
  devServerId: string
): ProjectWithRepos {
  const known = new Set(before.projects.map((p) => p.id))
  const created = after.projects.filter((p) => !known.has(p.id))
  expect(
    created.map((p) => p.name),
    `Add Project of ${folder.name} creates ONE new project`
  ).toHaveLength(1)
  const [project] = created
  expect.soft(project.name, 'the project is named after the folder').toBe(folder.name)
  expect
    .soft(project.devServerId, 'the project is bound to the chosen dev server')
    .toBe(devServerId)
  expect
    .soft(
      project.repos.map((r) => r.url),
      'the project holds exactly the added repo'
    )
    .toEqual([folder.path])
  expect
    .soft(project.repos[0]?.devServerId, 'the repo is bound to the dev server')
    .toBe(devServerId)
  for (const old of before.projects) {
    const now = after.projects.find((p) => p.id === old.id)
    expect
      .soft(
        now?.repos.map((r) => r.id),
        `the existing project "${old.name}" gained no repo`
      )
      .toEqual(old.repos.map((r) => r.id))
  }
  return project
}

/** What the SPA sent while adding one folder: one project.create with name/devServerId/repoPath, and no separate repo.add. */
export function expectOwnProjectRpc(
  calls: SentRpc[],
  from: number,
  folder: SandboxRepo,
  devServerId: string
): void {
  const creates = callsTo({ calls }, 'project.create', from)
  expect.soft(creates, 'the UI sent exactly one project.create').toHaveLength(1)
  expect
    .soft(creates[0]?.params, 'project.create carries the folder name, host and path')
    .toMatchObject({ name: folder.name, devServerId, repoPath: folder.path })
  expect
    .soft(
      callsTo({ calls }, 'repo.add', from),
      'the repo came with the project: no repo.add to a host-wide project'
    )
    .toEqual([])
}

/** The UI wrote nothing while it refused the folder. */
export function expectNoWriteSent(calls: SentRpc[], from: number, what: string): void {
  const writes = calls
    .slice(from)
    .filter((c) =>
      /^(project\.create|repo\.add|repo\.clone|repo\.create|repo\.addRemote)$/.test(c.method)
    )
  expect
    .soft(
      writes.map((w) => w.method),
      `${what}: no project/repo write was sent`
    )
    .toEqual([])
}

/** The backend refuses a second registration of the same folder on the same dev server, with the documented code. */
export async function expectBackendRefusesDuplicate(
  rpc: GatewayRpcClient,
  args: { name: string; devServerId: string; repoPath: string; holder: ProjectWithRepos }
): Promise<void> {
  const refused = await rpc
    .call('project.create', {
      name: args.name,
      devServerId: args.devServerId,
      repoPath: args.repoPath,
      visibility: 'private'
    })
    .then(
      () => '',
      (e: Error) => e.message
    )
  expect(refused, `project.create of ${args.repoPath} must be refused`).toContain(
    'PROJECT_REPO_ALREADY_REGISTERED'
  )
  expect
    .soft(refused, 'the refusal names the project that holds the folder')
    .toContain(`already registered in project "${args.holder.name}" (${args.holder.id})`)
}

const BRAVO_FILES = /bravo_math|test_bravo_math|README\.md|CLAUDE\.md/
const ALPHA_MARKERS = new RegExp(
  `${REPO_FACTS.alpha.functions.join('|')}|${REPO_FACTS.alpha.module}|-alpha`,
  'i'
)

type AnswerSolution = { kind: string; status: string; options?: Record<string, unknown> }

/** The answer on screen is the backend's, is about the bravo repo, and mentions nothing of the alpha repo. */
export function expectAnswerAboutBravo(args: {
  screen: { view: string; citations: string[]; confidence: string }
  stored: AnswerSolution
  snippet: string
}): void {
  const { screen, stored, snippet } = args
  const markdown = String(stored.options?.answer_markdown ?? '')
  expect(
    markdown.trim().length,
    'solution.list: options.answer_markdown is not empty'
  ).toBeGreaterThan(0)
  expect.soft(screen.view, 'the Answer box is not "No data"').not.toContain('No data')
  expect
    .soft(screen.view, `the screen shows a stretch of the stored answer ("${snippet}")`)
    .toContain(snippet)
  expect.soft(screen.confidence, 'answer-confidence shows a percentage').toMatch(/\d+%/)
  const tokens = [...REPO_FACTS.bravo.functions, REPO_FACTS.bravo.module]
  expect
    .soft(
      tokens.some((t) => screen.view.includes(t)),
      `the answer names a function or module of the bravo repo (${tokens.join(', ')})`
    )
    .toBe(true)
  expect
    .soft(screen.citations.length, 'answer-citations lists at least one citation')
    .toBeGreaterThan(0)
  expect
    .soft(
      screen.citations.some((c) => BRAVO_FILES.test(c)),
      `a citation points at a file of the bravo repo (got ${JSON.stringify(screen.citations.slice(0, 4))})`
    )
    .toBe(true)
  const stray = [markdown, ...screen.citations, screen.view].filter((t) => ALPHA_MARKERS.test(t))
  expect.soft(stray, 'nothing of the alpha repo appears in the answer or its citations').toEqual([])
}

const READ_ONLY_TOOLS = new Set(['Read', 'Glob', 'Grep'])

/** The analysis ran in the bravo repo (its guard log), only read, and neither repo changed. */
export function expectOnlyBravoWasRead(args: {
  alpha: SandboxRepo
  bravo: SandboxRepo
  heads: { alpha: string; bravo: string }
}): string {
  const { alpha, bravo, heads } = args
  const tools = bravo.guardCalls().map((l) => l.split(' ')[1])
  expect.soft(tools.length, 'the bravo guard saw claude work in the bravo repo').toBeGreaterThan(0)
  expect
    .soft(
      tools.filter((t) => !READ_ONLY_TOOLS.has(t)),
      'claude used read-only tools only'
    )
    .toEqual([])
  expect
    .soft(alpha.guardCalls(), 'claude never ran in the alpha repo (its guard saw nothing)')
    .toEqual([])
  for (const [name, repo, head] of [
    ['alpha', alpha, heads.alpha],
    ['bravo', bravo, heads.bravo]
  ] as const) {
    expect.soft(repo.repoStatus(), `${name}: the working tree is clean after the analysis`).toBe('')
    expect.soft(repo.headSha(), `${name}: no commit was made`).toBe(head)
    expect.soft(repo.branches(), `${name}: no branch was created`).toEqual(['main'])
    expect.soft(repo.worktrees().length, `${name}: no worktree was created`).toBe(1)
  }
  return `bravo guard calls ${tools.length}`
}
