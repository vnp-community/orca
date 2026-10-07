export type ReindexCommand =
  | { tool: 'gitnexus', mode: 'init' | 'full' | 'incremental', repoRoot: string }
  | { tool: 'codegraph', mode: 'init' | 'full' | 'incremental', projectPath: string }

export function buildReindexArgv(cmd: ReindexCommand): string[] {
  if (cmd.tool === 'gitnexus') {
    const args = ['analyze', '--index-only', '-r', cmd.repoRoot]
    if (cmd.mode === 'full') {
      args.push('--force')
    }
    return args
  } else if (cmd.tool === 'codegraph') {
    const args = ['reindex', '-p', cmd.projectPath]
    if (cmd.mode === 'full') {
      args.push('--force')
    }
    return args
  }
  throw new Error(`Unknown tool: ${(cmd as any).tool}`)
}
