# TASK-CR-001: Implement AnnotationDiffService

**Priority:** 🟠 HIGH  
**Effort:** ~60 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-CR-001, BUG-CR-001  
**Solution ref:** [SOLUTION-code-review.md](../solutions/SOLUTION-code-review.md)

## Mục tiêu

Tạo `AnnotationDiffService` tính toán diff giữa code review annotations và current file content, resolve remote file paths.

## File cần tạo

`src/main/code-review/AnnotationDiffService.ts` (NEW)

## Pattern

```typescript
export class AnnotationDiffService {
  constructor(private readonly relay: DevServerRelayBridge) {}

  /**
   * Compute diff annotations for a file at a given commit vs HEAD.
   * Resolves remote paths before fetching.
   */
  async getDiffAnnotations(params: {
    repoPath:    string
    filePath:    string   // relative to repo root
    fromCommit:  string
    toCommit:    string
  }): Promise<DiffAnnotation[]> {
    // Resolve absolute remote path:
    const absolutePath = `${params.repoPath}/${params.filePath}`.replace('//', '/')

    // Get diff via relay:
    const diff = await this.relay.call('git.exec', {
      cwd: params.repoPath,
      args: ['diff', params.fromCommit, params.toCommit, '--', params.filePath],
    }) as { stdout: string }

    return this.parseDiff(diff.stdout, params.filePath)
  }

  private parseDiff(rawDiff: string, filePath: string): DiffAnnotation[] {
    const annotations: DiffAnnotation[] = []
    const lines = rawDiff.split('\n')
    let lineNum = 0

    for (const line of lines) {
      if (line.startsWith('@@')) {
        const match = line.match(/@@ -\d+(?:,\d+)? \+(\d+)/)
        if (match) lineNum = Number(match[1]) - 1
      } else if (line.startsWith('+') && !line.startsWith('+++')) {
        lineNum++
        annotations.push({ file: filePath, line: lineNum, type: 'addition', content: line.slice(1) })
      } else if (line.startsWith('-') && !line.startsWith('---')) {
        annotations.push({ file: filePath, line: lineNum, type: 'deletion', content: line.slice(1) })
      } else if (!line.startsWith('\\')) {
        lineNum++
      }
    }
    return annotations
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
```
