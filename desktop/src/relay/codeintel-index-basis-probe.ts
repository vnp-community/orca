import { execFile } from 'child_process'
import { promisify } from 'util'
import fs from 'fs'
import path from 'path'

const execFileAsync = promisify(execFile)

export type GitRunner = (args: string[], cwd: string) => Promise<{ stdout: string; stderr: string }>
export type StatFn = (path: string) => Promise<fs.Stats>

export async function probeIndexBasis(
  workspaceRoot: string,
  input: { indexedCommit: string | null; indexedAtMs: number | null; baseRef: string },
  deps?: { git?: GitRunner; stat?: StatFn; now?: () => number }
): Promise<{ 
  headCommit: string | null; 
  headCommitTimeMs: number | null; 
  mergeBase: string | null; 
  mergeBaseCommitTimeMs: number | null; 
  changedFilesNotInIndex: number | null; 
  dirtySinceIndex: boolean 
}> {
  // All commands here are compatible with Git < 2.25.
  // No need for GitCapabilityCache. Always use -c core.quotePath=false when needed.
  const git = deps?.git || (async (args: string[], cwd: string) => {
    return execFileAsync('git', args, { cwd, timeout: 5000, maxBuffer: 8 * 1024 * 1024 })
  })
  const stat = deps?.stat || fs.promises.stat

  const result = {
    headCommit: null as string | null,
    headCommitTimeMs: null as number | null,
    mergeBase: null as string | null,
    mergeBaseCommitTimeMs: null as number | null,
    changedFilesNotInIndex: null as number | null,
    dirtySinceIndex: false
  }

  // 1. headCommit
  try {
    const headOutput = await git(['rev-parse', 'HEAD'], workspaceRoot)
    result.headCommit = headOutput.stdout.trim()
    const timeOutput = await git(['log', '-1', '--format=%ct', 'HEAD'], workspaceRoot)
    result.headCommitTimeMs = parseInt(timeOutput.stdout.trim(), 10) * 1000
  } catch (e) {
    return result // unborn or missing git
  }

  // 2. mergeBase
  try {
    const mbOutput = await git(['merge-base', 'HEAD', input.baseRef], workspaceRoot)
    result.mergeBase = mbOutput.stdout.trim()
    const mbTimeOutput = await git(['log', '-1', '--format=%ct', result.mergeBase], workspaceRoot)
    result.mergeBaseCommitTimeMs = parseInt(mbTimeOutput.stdout.trim(), 10) * 1000
  } catch (e) {
    result.mergeBase = null
    result.mergeBaseCommitTimeMs = null
  }

  // 3. changedFilesNotInIndex
  if (!input.indexedCommit) {
    result.changedFilesNotInIndex = null
    return result
  }

  try {
    await git(['cat-file', '-e', `${input.indexedCommit}^{commit}`], workspaceRoot)
  } catch (e) {
    result.changedFilesNotInIndex = null
    result.dirtySinceIndex = true
    return result
  }

  const filesSet = new Set<string>()
  let capped = false

  try {
    const diffOut = await git(['-c', 'core.quotePath=false', 'diff', '--name-only', '-z', input.indexedCommit, 'HEAD'], workspaceRoot)
    const diffFiles = diffOut.stdout.split('\0').filter(Boolean)
    for (const f of diffFiles) {
      if (filesSet.size >= 5000) {
        capped = true
        break
      }
      filesSet.add(f)
    }

    if (!capped) {
      const statusOut = await git(['-c', 'core.quotePath=false', 'status', '--porcelain=v1', '-z'], workspaceRoot)
      const entries = statusOut.stdout.split('\0')
      for (let i = 0; i < entries.length; i++) {
        if (!entries[i]) continue
        
        const line = entries[i]
        const xy = line.substring(0, 2)
        const filePath = line.substring(3)
        
        if (filesSet.size >= 5000) {
          capped = true
          break
        }
        filesSet.add(filePath)

        if (xy[0] === 'R' || xy[1] === 'R' || xy[0] === 'C' || xy[1] === 'C') {
          i++ // skip original path
        }
      }
    }
  } catch (e) {
    capped = true
  }

  result.changedFilesNotInIndex = capped ? 5000 : filesSet.size
  
  if (result.changedFilesNotInIndex > 0 || capped) {
    result.dirtySinceIndex = true
  } else if (input.indexedAtMs != null) {
    // Technical fallback: check mtimes of the set. But if set is empty, it skips.
    for (const f of filesSet) {
      try {
        const st = await stat(path.join(workspaceRoot, f))
        if (st.mtimeMs > input.indexedAtMs) {
          result.dirtySinceIndex = true
          break
        }
      } catch {
        // ignore
      }
    }
  }

  return result
}
