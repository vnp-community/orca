/**
 * What the run left on disk, read with git and never changed: the new worktrees next to the repo (names, branches, ancestry from
 * the starting branch's old tip, commits, diff stat) and a syntax-only look at the files the agents wrote. Nothing is executed:
 * a file written by an agent is parsed, not run.
 */
import { spawnSync } from 'node:child_process'
import { existsSync, mkdtempSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { basename, extname, join } from 'node:path'
import { CICDKIT_REPO } from './ci-cd-kit-backlog.ts'
import type { WorktreeFact } from './ci-cd-kit-report'

function git(cwd: string, args: string[]): { status: number; out: string } {
  const r = spawnSync('git', ['-C', cwd, ...args], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024
  })
  return { status: r.status ?? 1, out: (r.stdout ?? '').trim() }
}

/** `git worktree list --porcelain` of the repo as path -> branch ('' when detached). */
export function worktreeBranches(repo = CICDKIT_REPO): Map<string, string> {
  const map = new Map<string, string>()
  let path = ''
  for (const line of git(repo, ['worktree', 'list', '--porcelain']).out.split('\n')) {
    if (line.startsWith('worktree ')) {
      path = line.slice('worktree '.length)
      map.set(path, '')
    } else if (line.startsWith('branch ') && path) {
      map.set(path, line.slice('branch refs/heads/'.length))
    }
  }
  return map
}

export function describeWorktree(path: string, baseTip: string, branch: string): WorktreeFact {
  const head = git(path, ['rev-parse', 'HEAD']).out
  return {
    path,
    branch,
    head,
    descendsFromBase: git(path, ['merge-base', '--is-ancestor', baseTip, 'HEAD']).status === 0,
    mergeBase: git(path, ['merge-base', baseTip, 'HEAD']).out,
    commitsAhead: Number(git(path, ['rev-list', '--count', `${baseTip}..HEAD`]).out) || 0,
    diffStat: git(path, ['diff', '--stat', `${baseTip}..HEAD`])
      .out.split('\n')
      .slice(-25)
      .join('\n'),
    uncommitted: git(path, ['status', '--porcelain']).out.split('\n').filter(Boolean).length
  }
}

/** The worktrees the run added, each described against the old tip of the starting branch. */
export function describeNewWorktrees(newPaths: string[], baseTip: string): WorktreeFact[] {
  const branches = worktreeBranches()
  return newPaths
    .filter((p) => existsSync(p))
    .map((p) => describeWorktree(p, baseTip, branches.get(p) ?? ''))
}

const PY_PARSE = 'import ast,sys; ast.parse(open(sys.argv[1], encoding="utf-8").read())'
const YAML_PARSE = 'import sys,yaml; list(yaml.safe_load_all(open(sys.argv[1], encoding="utf-8")))'

/** An empty directory to run the parsers in, so nothing a worktree holds (a stray json.py) is picked up by an interpreter. */
const neutralDir = mkdtempSync(join(tmpdir(), 'cicd-static-'))

function run(cmd: string, args: string[]): { ok: boolean; err: string } {
  const r = spawnSync(cmd, args, { encoding: 'utf8', timeout: 20_000, cwd: neutralDir })
  return { ok: r.status === 0, err: `${r.stderr ?? ''}`.trim().split('\n').at(-1) ?? '' }
}

/** One written file looked at for syntax only; returns a line for the report, or '' for a kind with nothing to check. */
export function staticCheckFile(path: string, yamlAvailable: boolean): string {
  const name = basename(path)
  const ext = extname(path)
  if (ext === '.py') {
    const r = run('python3', ['-I', '-c', PY_PARSE, path])
    return `${r.ok ? 'ok' : 'LỖI CÚ PHÁP'} python ${name}${r.ok ? '' : `: ${r.err}`}`
  }
  if (ext === '.yml' || ext === '.yaml') {
    if (!yamlAvailable) {
      return `bỏ qua yaml ${name} (python3 không có PyYAML)`
    }
    // Why: -I would hide the user site where PyYAML lives; -E plus an empty working directory keeps the same protection.
    const r = run('python3', ['-E', '-c', YAML_PARSE, path])
    return `${r.ok ? 'ok' : 'LỖI YAML'} ${name}${r.ok ? '' : `: ${r.err}`}`
  }
  if (ext === '.json') {
    try {
      JSON.parse(readFileSync(path, 'utf8'))
      return `ok json ${name}`
    } catch (e) {
      return `LỖI JSON ${name}: ${(e as Error).message.slice(0, 100)}`
    }
  }
  const first =
    ext === '.sh' || name === 'Jenkinsfile' ? readFileSync(path, 'utf8').split('\n')[0] : ''
  if (ext === '.sh' || /^#!.*\b(ba)?sh\b/.test(first)) {
    const r = run('bash', ['-n', path])
    return `${r.ok ? 'ok' : 'LỖI CÚ PHÁP'} bash -n ${name}${r.ok ? '' : `: ${r.err}`}`
  }
  return ''
}

/** Syntax-only checks of the files a worktree changed (committed since the base tip, modified or new). */
export function staticChecksOfWorktree(wt: WorktreeFact, baseTip: string): string[] {
  const changed = new Set<string>([
    ...git(wt.path, ['diff', '--name-only', baseTip]).out.split('\n'),
    ...git(wt.path, ['ls-files', '--others', '--exclude-standard']).out.split('\n')
  ])
  const yamlAvailable = run('python3', ['-E', '-c', 'import yaml']).ok
  const lines: string[] = []
  for (const rel of [...changed].filter(Boolean).slice(0, 60)) {
    const abs = join(wt.path, rel)
    if (!existsSync(abs)) {
      continue
    }
    const line = staticCheckFile(abs, yamlAvailable)
    if (line) {
      lines.push(`${basename(wt.path)}: ${rel}: ${line}`)
    }
  }
  if (lines.length === 0) {
    lines.push(`${basename(wt.path)}: không có file python/yaml/json/sh nào đổi để kiểm`)
  }
  return lines
}
