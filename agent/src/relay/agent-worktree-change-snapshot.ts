// src/relay/agent-worktree-change-snapshot.ts
// Captures and diffs filesystem state before/after an agent run to detect
// what the model changed. Only lstat — never reads file contents.
//
// Git baseline: uses git 2.15+ options (all under the 2.25 baseline).
// Runs on the same host as the agent (correct for native, WSL, SSH).

import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { execFile as _execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(_execFile)

const SNAPSHOT_TIMEOUT_MS = 30_000
const MAX_CHANGED_FILES = 2000
const MAX_DIRECTORY_ENTRIES = 5000

// ── Types ──────────────────────────────────────────────────────────────────────

export type SnapshotEntry = {
  status: string   // XY code from git status, or '' for directory mode
  size: number | null
  mtimeMs: number | null
}

export type WorkspaceSnapshot =
  | { kind: 'git'; head: string | null; entries: Map<string, SnapshotEntry> }
  | { kind: 'directory'; entries: Map<string, SnapshotEntry>; truncated: boolean }
  | { kind: 'unavailable'; reason: string }

export type ChangedFile = {
  path: string
  change: 'added' | 'modified' | 'deleted' | 'untracked' | 'renamed'
}

export type ChangeReport =
  | {
      available: true
      headBefore: string | null
      headAfter: string | null
      headMoved: boolean
      changedFiles: ChangedFile[]
      truncated: boolean
    }
  | { available: false; reason: string }

export type SnapshotDeps = {
  runGit?: (args: string[], cwd: string) => Promise<{ stdout: string; code: number }>
  statFile?: (p: string) => Promise<{ size: number; mtimeMs: number } | null>
}

// ── Default deps ───────────────────────────────────────────────────────────────

async function defaultRunGit(
  args: string[],
  cwd: string
): Promise<{ stdout: string; code: number }> {
  try {
    const { stdout } = await execFileAsync('git', args, {
      cwd,
      timeout: SNAPSHOT_TIMEOUT_MS,
      windowsHide: true,
      maxBuffer: 50 * 1024 * 1024
    })
    return { stdout, code: 0 }
  } catch (err: unknown) {
    const e = err as { code?: number; stdout?: string }
    return { stdout: e.stdout ?? '', code: e.code ?? 1 }
  }
}

async function defaultStatFile(p: string): Promise<{ size: number; mtimeMs: number } | null> {
  try {
    const s = await fs.lstat(p)
    return { size: s.size, mtimeMs: s.mtimeMs }
  } catch {
    return null
  }
}

// ── Snapshot capture ───────────────────────────────────────────────────────────

export async function captureSnapshot(
  cwd: string,
  mode: 'git' | 'directory',
  deps: SnapshotDeps = {}
): Promise<WorkspaceSnapshot> {
  const runGit = deps.runGit ?? defaultRunGit
  const statFile = deps.statFile ?? defaultStatFile

  try {
    if (mode === 'directory') {
      return await captureDirectorySnapshot(cwd, statFile)
    }
    return await captureGitSnapshot(cwd, runGit, statFile)
  } catch (err: unknown) {
    return { kind: 'unavailable', reason: 'SNAPSHOT_FAILED' }
  }
}

async function captureGitSnapshot(
  cwd: string,
  runGit: NonNullable<SnapshotDeps['runGit']>,
  statFile: NonNullable<SnapshotDeps['statFile']>
): Promise<WorkspaceSnapshot> {
  // Check inside work tree
  const insideRes = await runGit(['rev-parse', '--is-inside-work-tree'], cwd)
  if (insideRes.code !== 0 || insideRes.stdout.trim() !== 'true') {
    return { kind: 'unavailable', reason: 'NOT_A_GIT_REPO' }
  }

  // Get HEAD (may be null for fresh repos)
  let head: string | null = null
  const headRes = await runGit(['rev-parse', 'HEAD'], cwd)
  if (headRes.code === 0 && headRes.stdout.trim()) {
    head = headRes.stdout.trim()
  }

  // Get dirty files: --no-optional-locks MUST precede 'status'
  const statusRes = await runGit(
    ['--no-optional-locks', 'status', '--porcelain=v1', '-z', '--untracked-files=all'],
    cwd
  )

  const entries = new Map<string, SnapshotEntry>()
  if (statusRes.stdout) {
    const parts = statusRes.stdout.split('\0').filter(Boolean)
    let i = 0
    while (i < parts.length) {
      const entry = parts[i]!
      i++
      if (entry.length < 4) { continue }
      const xy = entry.slice(0, 2)
      const filePath = entry.slice(3)

      // Rename/copy: next part is the source path; skip it
      if (xy[0] === 'R' || xy[0] === 'C' || xy[1] === 'R' || xy[1] === 'C') {
        i++ // skip source path
      }

      const fullPath = path.join(cwd, filePath)
      const stat = await statFile(fullPath)
      entries.set(filePath, {
        status: xy,
        size: stat?.size ?? null,
        mtimeMs: stat?.mtimeMs ?? null
      })
    }
  }

  return { kind: 'git', head, entries }
}

async function captureDirectorySnapshot(
  cwd: string,
  statFile: NonNullable<SnapshotDeps['statFile']>
): Promise<WorkspaceSnapshot> {
  const entries = new Map<string, SnapshotEntry>()
  let count = 0
  let truncated = false

  async function walk(dir: string, prefix: string): Promise<void> {
    if (count >= MAX_DIRECTORY_ENTRIES) {
      truncated = true
      return
    }
    let children: fs.Dirent[]
    try {
      children = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }
    for (const child of children) {
      if (count >= MAX_DIRECTORY_ENTRIES) {
        truncated = true
        break
      }
      if (child.name === '.git') { continue }
      const rel = prefix ? `${prefix}/${child.name}` : child.name
      const fullPath = path.join(dir, child.name)
      count++

      // Do NOT follow symlinks
      if (child.isSymbolicLink()) {
        entries.set(rel, { status: '', size: null, mtimeMs: null })
        continue
      }
      if (child.isDirectory()) {
        entries.set(rel, { status: '', size: null, mtimeMs: null })
        await walk(fullPath, rel)
      } else {
        const stat = await statFile(fullPath)
        entries.set(rel, { status: '', size: stat?.size ?? null, mtimeMs: stat?.mtimeMs ?? null })
      }
    }
  }

  await walk(cwd, '')
  return { kind: 'directory', entries, truncated }
}

// ── Diff ───────────────────────────────────────────────────────────────────────

function mapChange(xy: string): ChangedFile['change'] {
  if (xy === '??' || xy === '!!') { return 'untracked' }
  if (xy.includes('R') || xy.includes('C')) { return 'renamed' }
  if (xy.includes('D')) { return 'deleted' }
  if (xy.includes('A')) { return 'added' }
  return 'modified'
}

export function diffSnapshots(
  before: WorkspaceSnapshot,
  after: WorkspaceSnapshot
): ChangeReport {
  if (before.kind === 'unavailable') {
    return { available: false, reason: before.reason }
  }
  if (after.kind === 'unavailable') {
    return { available: false, reason: after.reason }
  }
  if (before.kind !== after.kind) {
    return { available: false, reason: 'SNAPSHOT_KIND_MISMATCH' }
  }

  if (before.kind === 'git' && after.kind === 'git') {
    const headBefore = before.head
    const headAfter = after.head
    const headMoved = headBefore !== headAfter

    const allPaths = new Set([...before.entries.keys(), ...after.entries.keys()])
    const changedFiles: ChangedFile[] = []

    for (const p of allPaths) {
      const b = before.entries.get(p)
      const a = after.entries.get(p)

      const bSig = b ? `${b.status}|${b.size}|${b.mtimeMs}` : null
      const aSig = a ? `${a.status}|${a.size}|${a.mtimeMs}` : null

      if (bSig !== aSig) {
        // Use after entry's XY for change classification; fall back to before if only in before
        const xy = a ? a.status : (b ? b.status : '')
        changedFiles.push({ path: p, change: mapChange(xy) })
      }
    }

    changedFiles.sort((a, b) => a.path.localeCompare(b.path))
    const truncated = changedFiles.length > MAX_CHANGED_FILES
    return {
      available: true,
      headBefore,
      headAfter,
      headMoved,
      changedFiles: changedFiles.slice(0, MAX_CHANGED_FILES),
      truncated
    }
  }

  // directory mode
  const b = (before as Extract<WorkspaceSnapshot, { kind: 'directory' }>).entries
  const a = (after as Extract<WorkspaceSnapshot, { kind: 'directory' }>).entries
  const allPaths = new Set([...b.keys(), ...a.keys()])
  const changedFiles: ChangedFile[] = []

  for (const p of allPaths) {
    const bEntry = b.get(p)
    const aEntry = a.get(p)
    if (!bEntry && aEntry) {
      changedFiles.push({ path: p, change: 'added' })
    } else if (bEntry && !aEntry) {
      changedFiles.push({ path: p, change: 'deleted' })
    } else if (bEntry && aEntry) {
      if (bEntry.size !== aEntry.size || bEntry.mtimeMs !== aEntry.mtimeMs) {
        changedFiles.push({ path: p, change: 'modified' })
      }
    }
  }

  changedFiles.sort((a, b) => a.path.localeCompare(b.path))
  const truncated = changedFiles.length > MAX_CHANGED_FILES
  return {
    available: true,
    headBefore: null,
    headAfter: null,
    headMoved: false,
    changedFiles: changedFiles.slice(0, MAX_CHANGED_FILES),
    truncated
  }
}
