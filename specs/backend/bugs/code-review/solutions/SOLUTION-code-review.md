# SOLUTION: Code Review Domain — Fix tất cả Bugs

**Domain:** code-review  
**TDD Reference:** TDD-20 (Remote Git UI), TDD-07 (Runtime Service), TDD-05 (SSH Relay)  
**Files cần thay đổi:** `src/main/code-review/AnnotationDiffService.ts` (NEW), `src/main/code-review/DiffService.ts`  
**Tổng số bugs:** 2 (CR-001, BE-CR-001)

---

## BUG-BE-CR-001 — Fix AnnotationDiffService not implemented

**Mức độ:** 🟠 HIGH  
**Root cause:** Service để diff annotations (code review comments) chưa được implement.

### Fix — Implement AnnotationDiffService

```typescript
// src/main/code-review/AnnotationDiffService.ts (NEW)

export interface FileAnnotation {
  id:         string
  filePath:   string
  line:       number
  endLine?:   number
  comment:    string
  authorId:   string
  severity:   'info' | 'warning' | 'error' | 'suggestion'
  resolved:   boolean
  createdAt:  number
}

export interface AnnotationDiff {
  added:    FileAnnotation[]
  removed:  FileAnnotation[]
  modified: Array<{ before: FileAnnotation; after: FileAnnotation }>
}

export class AnnotationDiffService {
  /**
   * Compute diff giữa 2 sets of annotations.
   * Dùng để show "new comments since last review" hoặc "resolved comments".
   */
  diff(before: FileAnnotation[], after: FileAnnotation[]): AnnotationDiff {
    const beforeMap = new Map(before.map(a => [a.id, a]))
    const afterMap  = new Map(after.map(a => [a.id, a]))

    const added: FileAnnotation[] = []
    const removed: FileAnnotation[] = []
    const modified: Array<{ before: FileAnnotation; after: FileAnnotation }> = []

    // Find added + modified
    for (const [id, afterAnnotation] of afterMap) {
      const beforeAnnotation = beforeMap.get(id)
      if (!beforeAnnotation) {
        added.push(afterAnnotation)
      } else if (this.hasChanged(beforeAnnotation, afterAnnotation)) {
        modified.push({ before: beforeAnnotation, after: afterAnnotation })
      }
    }

    // Find removed
    for (const [id, beforeAnnotation] of beforeMap) {
      if (!afterMap.has(id)) {
        removed.push(beforeAnnotation)
      }
    }

    return { added, removed, modified }
  }

  private hasChanged(before: FileAnnotation, after: FileAnnotation): boolean {
    return before.comment !== after.comment
      || before.resolved !== after.resolved
      || before.line !== after.line
      || before.severity !== after.severity
  }

  /**
   * Apply git patch để shift annotation line numbers khi file thay đổi.
   * Khi file thay đổi, dòng số của annotations cần được cập nhật.
   */
  applyPatchToAnnotations(
    annotations: FileAnnotation[],
    hunks: Array<{ startLine: number; addedLines: number; removedLines: number }>,
  ): FileAnnotation[] {
    return annotations.map(annotation => {
      let adjustedLine = annotation.line

      for (const hunk of hunks) {
        if (annotation.line > hunk.startLine) {
          adjustedLine += hunk.addedLines - hunk.removedLines
        }
      }

      return { ...annotation, line: Math.max(1, adjustedLine) }
    })
  }
}
```

---

## BUG-CR-001 — Fix DiffService thiếu remote path

**Mức độ:** 🟠 HIGH  
**Root cause:** `DiffService` chỉ handle local paths, không handle remote file paths trên Dev Server.

### Fix — Thêm remote path resolution

```typescript
// src/main/code-review/DiffService.ts

export class DiffService {
  constructor(
    private readonly devServerManager: DevServerManager,
    private readonly log: Logger,
  ) {}

  /**
   * Get diff cho file — hỗ trợ cả local và remote.
   * Remote path được resolve thông qua relay bridge.
   */
  async getDiff(params: DiffParams): Promise<DiffResult> {
    const { projectId, filePath, baseRef, headRef, devServerId } = params

    if (devServerId) {
      // Remote path: delegate đến Dev Server
      return await this.getRemoteDiff(devServerId, params)
    } else {
      // Local path
      return await this.getLocalDiff(params)
    }
  }

  /**
   * FIX CR-001: Remote path resolution via relay bridge.
   */
  private async getRemoteDiff(devServerId: string, params: DiffParams): Promise<DiffResult> {
    const bridge = this.devServerManager.getBridge(devServerId)
    if (!bridge) {
      throw new Error(`Dev server not connected: ${devServerId}`)
    }

    // Sử dụng git.exec trên remote host
    const result = await bridge.call('git.exec', {
      cwd:  params.repoPath,  // remote repo path
      args: ['diff', params.baseRef, params.headRef, '--', params.filePath],
    })

    return this.parseDiffOutput(result.stdout)
  }

  private async getLocalDiff(params: DiffParams): Promise<DiffResult> {
    // Existing local diff logic
    const { stdout } = await runCommandCapture('git', [
      'diff', params.baseRef, params.headRef, '--', params.filePath
    ], { cwd: params.repoPath })
    
    return this.parseDiffOutput(stdout)
  }

  private parseDiffOutput(diffText: string): DiffResult {
    const hunks: DiffHunk[] = []
    const lines = diffText.split('\n')
    
    let currentHunk: DiffHunk | null = null
    
    for (const line of lines) {
      if (line.startsWith('@@')) {
        // Parse hunk header: @@ -L,S +L,S @@
        const match = line.match(/@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@/)
        if (match) {
          currentHunk = {
            oldStart:  parseInt(match[1]),
            oldLines:  parseInt(match[2] || '1'),
            newStart:  parseInt(match[3]),
            newLines:  parseInt(match[4] || '1'),
            lines:     [],
          }
          hunks.push(currentHunk)
        }
      } else if (currentHunk) {
        currentHunk.lines.push({
          type:    line[0] === '+' ? 'added' : line[0] === '-' ? 'removed' : 'context',
          content: line.slice(1),
        })
      }
    }

    return { hunks, raw: diffText }
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/code-review/AnnotationDiffService.ts` | NEW — implement annotation diff | BE-CR-001 |
| `src/main/code-review/DiffService.ts` | Add remote path resolution via relay | CR-001 |
| `src/main/ipc/code-review-ipc.ts` | Wire AnnotationDiffService | BE-CR-001 |

---

## Verification Plan

```bash
pnpm vitest run src/main/code-review/__tests__/

# Manual test:
# 1. Add annotation → modify file → verify line numbers shifted correctly
# 2. Request diff for remote file → verify relay bridge called with correct cwd
# 3. Diff 2 annotation sets → verify added/removed/modified correctly
```
