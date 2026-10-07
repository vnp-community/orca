export type ReindexCommand =
  | { tool: 'gitnexus', mode: 'init' | 'full' | 'incremental', repoRoot: string }
  | { tool: 'codegraph', mode: 'init' | 'full' | 'incremental', projectPath: string }

export function buildReindexArgv(cmd: ReindexCommand): string[] {
  if (cmd.tool === 'gitnexus') {
    const args = ['analyze', '--index-only']
    if (cmd.mode === 'full') {
      args.push('--force')
    }
    args.push('-r', cmd.repoRoot)
    return args
  } else if (cmd.tool === 'codegraph') {
    const args = ['reindex']
    if (cmd.mode === 'full') {
      args.push('--force')
    }
    args.push('-p', cmd.projectPath)
    return args
  }
  throw new Error(`Unknown tool: ${(cmd as any).tool}`)
}
