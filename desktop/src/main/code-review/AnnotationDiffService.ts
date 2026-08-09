/**
 * AnnotationDiffService — Code review annotation diff computation (TASK-CR-001)
 *
 * Computes line-level diff annotations between two commits for a given file.
 * Resolves remote paths via relay before fetching git diff output.
 *
 * Architecture:
 *   - Calls 'git.exec' via relay (works with remote dev servers)
 *   - Returns DiffAnnotation[] with file, line, type (addition/deletion), content
 *   - Non-throwing on relay failure — returns empty array with warning
 *
 * @module main/code-review/AnnotationDiffService
 */

import type { DevServerRelayBridge } from '../dev-server/dev-server-relay-bridge'

// ── Types ─────────────────────────────────────────────────────────────────────

export type DiffAnnotation = {
  /** Relative file path from repo root */
  file:    string
  /** 1-indexed line number in the new version (additions) or old (deletions) */
  line:    number
  /** 'addition' = new line, 'deletion' = removed line */
  type:    'addition' | 'deletion'
  /** Line content (without the leading +/- character) */
  content: string
}

export type GetDiffAnnotationsParams = {
  repoPath:   string
  filePath:   string  // relative to repo root
  fromCommit: string
  toCommit:   string
}

// ── AnnotationDiffService ─────────────────────────────────────────────────────

export class AnnotationDiffService {
  constructor(private readonly relay: DevServerRelayBridge) {}

  /**
   * Compute diff annotations for a file between two commits.
   * Calls git diff via relay — works with remote dev servers.
   * Returns empty array on any error (offline-tolerant).
   */
  async getDiffAnnotations(params: GetDiffAnnotationsParams): Promise<DiffAnnotation[]> {
    try {
      // FIX TASK-CR-001: Resolve path safely (no double-slash)
      const safeRepoPath = params.repoPath.replace(/\/+$/, '')

      const result = await this.relay.call('git.exec', {
        cwd:  safeRepoPath,
        args: ['diff', params.fromCommit, params.toCommit, '--', params.filePath],
      }) as { stdout?: string } | null

      const stdout = result?.stdout ?? ''
      if (!stdout.trim()) {return []}

      return this.parseDiff(stdout, params.filePath)
    } catch (err) {
      console.warn(`[AnnotationDiffService] getDiffAnnotations failed for ${params.filePath}:`, err)
      return []
    }
  }

  /**
   * Parse unified diff output into structured annotations.
   * Handles hunk headers (@@ lines) to track line numbers correctly.
   */
  private parseDiff(rawDiff: string, filePath: string): DiffAnnotation[] {
    const annotations: DiffAnnotation[] = []
    const lines = rawDiff.split('\n')
    let lineNum = 0

    for (const line of lines) {
      if (line.startsWith('@@')) {
        // Parse hunk header: @@ -old_start[,old_count] +new_start[,new_count] @@
        const match = line.match(/@@ -\d+(?:,\d+)? \+(\d+)/)
        if (match) {lineNum = Number(match[1]) - 1}
      } else if (line.startsWith('+') && !line.startsWith('+++')) {
        lineNum++
        annotations.push({
          file:    filePath,
          line:    lineNum,
          type:    'addition',
          content: line.slice(1),
        })
      } else if (line.startsWith('-') && !line.startsWith('---')) {
        // Deletions: keep current lineNum (they don't advance the new file position)
        annotations.push({
          file:    filePath,
          line:    lineNum + 1,
          type:    'deletion',
          content: line.slice(1),
        })
      } else if (!line.startsWith('\\')) {
        // Context line — advance line number
        lineNum++
      }
    }

    return annotations
  }
}
