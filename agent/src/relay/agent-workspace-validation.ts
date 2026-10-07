// src/relay/agent-workspace-validation.ts
// Validates the worktreePath against the declared workspaceKind:
//   worktree  — no filesystem checks (preserves legacy ENOENT behaviour)
//   repo_root — must exist, be a directory, be inside a git repo, AND be readonly
//   scratch   — must exist, be a directory, be under an allowed scratch root
// Returns a discriminated union; never throws.

import * as fs from 'node:fs/promises'
import * as os from 'node:os'
import * as path from 'node:path'
import { execFile as _execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(_execFile)

// ── Types ──────────────────────────────────────────────────────────────────────

export type WorkspaceErrorCode =
  | 'REPO_ROOT_REQUIRES_READONLY'
  | 'WORKSPACE_PATH_NOT_FOUND'
  | 'WORKSPACE_NOT_A_DIRECTORY'
  | 'WORKSPACE_NOT_A_GIT_REPO'
  | 'SCRATCH_OUTSIDE_ALLOWED_ROOTS'

export type WorkspaceCheckInput = {
  kind: 'worktree' | 'repo_root' | 'scratch'
  path: string
  accessMode: 'write' | 'readonly'
  scratchRoots: string[]
  /** Injectable for tests; defaults to execFile git rev-parse --show-toplevel */
  gitToplevel?: (cwd: string) => Promise<string | null>
}

type ValidateOk = { ok: true; realPath: string }
type ValidateErr = { ok: false; code: WorkspaceErrorCode; message: string }

export type WorkspaceValidationResult = ValidateOk | ValidateErr

// ── Helpers ────────────────────────────────────────────────────────────────────

async function defaultGitToplevel(cwd: string): Promise<string | null> {
  try {
    const { stdout } = await execFileAsync(
      'git',
      ['rev-parse', '--show-toplevel'],
      { cwd, timeout: 5_000, windowsHide: true }
    )
    return stdout.trim() || null
  } catch {
    return null
  }
}

/** Compute allowed scratch roots; non-existent roots are included (realpath
 *  resolution happens lazily inside validateWorkspace for scratch paths). */
export function defaultScratchRoots(workDir: string): string[] {
  return [os.tmpdir(), path.join(workDir, '.orca-scratch')]
}

// ── Main validator ─────────────────────────────────────────────────────────────

export async function validateWorkspace(
  input: WorkspaceCheckInput
): Promise<WorkspaceValidationResult> {
  const { kind, path: inputPath, accessMode, scratchRoots } = input
  const gitToplevel = input.gitToplevel ?? defaultGitToplevel

  switch (kind) {
    case 'worktree': {
      // No filesystem checks — preserves legacy behaviour where ENOENT is
      // exposed at spawn time, which keeps the existing error path intact.
      return { ok: true, realPath: inputPath }
    }

    case 'repo_root': {
      // (a) enforce readonly BEFORE any filesystem access — avoids leaking
      // whether the path exists when the request itself is invalid
      if (accessMode !== 'readonly') {
        return {
          ok: false,
          code: 'REPO_ROOT_REQUIRES_READONLY',
          message: `repo_root workspaceKind requires accessMode "readonly", got "${accessMode}"`
        }
      }

      // (b) resolve symlinks
      let realPath: string
      try {
        realPath = await fs.realpath(inputPath)
      } catch {
        return {
          ok: false,
          code: 'WORKSPACE_PATH_NOT_FOUND',
          message: `workspace path not found: "${inputPath}". ` +
            'If the path exists but is owned by a different user, check git safe.directory.'
        }
      }

      // (c) must be a directory
      try {
        const stat = await fs.stat(realPath)
        if (!stat.isDirectory()) {
          return {
            ok: false,
            code: 'WORKSPACE_NOT_A_DIRECTORY',
            message: `workspace path is not a directory: "${inputPath}"`
          }
        }
      } catch {
        return {
          ok: false,
          code: 'WORKSPACE_PATH_NOT_FOUND',
          message: `workspace path not found: "${inputPath}"`
        }
      }

      // (d) must be inside a git repo
      const toplevel = await gitToplevel(realPath)
      if (!toplevel) {
        return {
          ok: false,
          code: 'WORKSPACE_NOT_A_GIT_REPO',
          message: `workspace path is not inside a git repository: "${inputPath}". ` +
            'If the directory is owned by a different user, check git safe.directory.'
        }
      }

      return { ok: true, realPath }
    }

    case 'scratch': {
      // Resolve symlinks for the input path
      let realPath: string
      try {
        realPath = await fs.realpath(inputPath)
      } catch {
        return {
          ok: false,
          code: 'WORKSPACE_PATH_NOT_FOUND',
          message: `workspace path not found: "${inputPath}"`
        }
      }

      // Must be a directory
      try {
        const stat = await fs.stat(realPath)
        if (!stat.isDirectory()) {
          return {
            ok: false,
            code: 'WORKSPACE_NOT_A_DIRECTORY',
            message: `workspace path is not a directory: "${inputPath}"`
          }
        }
      } catch {
        return {
          ok: false,
          code: 'WORKSPACE_PATH_NOT_FOUND',
          message: `workspace path not found: "${inputPath}"`
        }
      }

      // Check against each allowed root (resolve root symlinks too)
      for (const root of scratchRoots) {
        let rootReal: string
        try {
          rootReal = await fs.realpath(root)
        } catch {
          // Root doesn't exist — skip it, not an error
          continue
        }

        const rel = path.relative(rootReal, realPath)
        // Valid if: non-empty (not equal to root), doesn't start with '..', not absolute
        if (rel && !rel.startsWith('..') && !path.isAbsolute(rel)) {
          return { ok: true, realPath }
        }
      }

      return {
        ok: false,
        code: 'SCRATCH_OUTSIDE_ALLOWED_ROOTS',
        message: `scratch workspace "${inputPath}" is outside all allowed scratch roots`
      }
    }
  }
}
