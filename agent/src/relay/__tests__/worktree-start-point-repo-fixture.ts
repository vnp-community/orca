import { execFileSync } from 'node:child_process'
import { mkdtempSync, realpathSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// Real-git fixture for start-point resolution: nothing about git is mocked.
export type StartPointFixture = {
  root: string
  clone: string
  /** Env a git child process needs to be isolated from the developer's config. */
  env: Record<string, string>
  /** SHA of every interesting ref by label. */
  sha: Record<string, string>
  git: (cwd: string, ...args: string[]) => string
  cleanup: () => void
}

function isolatedEnv(root: string): Record<string, string> {
  const base: Record<string, string> = {}
  for (const [key, value] of Object.entries(process.env)) {
    if (value !== undefined) {
      base[key] = value
    }
  }
  return {
    ...base,
    HOME: root,
    USERPROFILE: root,
    GIT_CONFIG_NOSYSTEM: '1',
    GIT_CONFIG_GLOBAL: join(root, 'gitconfig'),
    GIT_AUTHOR_NAME: 'Fixture',
    GIT_AUTHOR_EMAIL: 'fixture@example.com',
    GIT_COMMITTER_NAME: 'Fixture',
    GIT_COMMITTER_EMAIL: 'fixture@example.com'
  }
}

export function createStartPointFixture(): StartPointFixture {
  const root = realpathSync(mkdtempSync(join(tmpdir(), 'orca-start-point-')))
  const env = isolatedEnv(root)
  const git = (cwd: string, ...args: string[]): string =>
    execFileSync('git', args, { cwd, env, encoding: 'utf8' }).trim()
  // Why: `init -b` needs git 2.28; the suite also runs on the 2.25 baseline.
  const initRepo = (repo: string, bare: boolean): void => {
    git(root, 'init', '-q', ...(bare ? ['--bare'] : []), repo)
    git(repo, 'symbolic-ref', 'HEAD', 'refs/heads/main')
  }
  const commitOn = (repo: string, label: string): string => {
    git(repo, 'commit', '-q', '--allow-empty', '-m', label)
    return git(repo, 'rev-parse', 'HEAD')
  }

  const seed = join(root, 'seed')
  initRepo(seed, false)
  const sha: Record<string, string> = { main: commitOn(seed, 'main') }

  // Remote-only branches: release/y, plus release/dup which also exists locally later.
  for (const name of ['release/y', 'release/dup']) {
    git(seed, 'checkout', '-q', '-b', name, 'main')
    sha[`origin:${name}`] = commitOn(seed, `origin ${name}`)
  }
  git(seed, 'checkout', '-q', 'main')

  const origin = join(root, 'origin.git')
  initRepo(origin, true)
  git(seed, 'push', '-q', origin, 'main', 'release/y', 'release/dup')

  // feature/z on two other remotes, at different commits.
  for (const remote of ['upstream', 'fork']) {
    git(seed, 'checkout', '-q', '-b', 'feature/z', 'main')
    sha[`${remote}:feature/z`] = commitOn(seed, `${remote} feature/z`)
    const bare = join(root, `${remote}.git`)
    initRepo(bare, true)
    git(seed, 'push', '-q', bare, 'feature/z')
    git(seed, 'checkout', '-q', 'main')
    git(seed, 'branch', '-q', '-D', 'feature/z')
  }

  const clone = join(root, 'clone')
  git(root, 'clone', '-q', origin, clone)
  for (const remote of ['upstream', 'fork']) {
    git(clone, 'remote', 'add', remote, join(root, `${remote}.git`))
  }
  git(clone, 'fetch', '-q', '--all')

  git(clone, 'checkout', '-q', '-b', 'release/x', 'main')
  sha['local:release/x'] = commitOn(clone, 'local release/x')
  git(clone, 'checkout', '-q', '-b', 'release/dup', 'main')
  sha['local:release/dup'] = commitOn(clone, 'local release/dup')
  git(clone, 'checkout', '-q', '-b', 'tagged', 'main')
  sha['tag:v1'] = commitOn(clone, 'tagged v1')
  git(clone, 'tag', 'v1')
  // The checkout sits on an unrelated branch: it must not influence the start point.
  git(clone, 'checkout', '-q', '-b', 'other', 'main')
  sha['local:other'] = commitOn(clone, 'local other')

  return {
    root,
    clone,
    env,
    sha,
    git,
    cleanup: () => rmSync(root, { recursive: true, force: true })
  }
}
