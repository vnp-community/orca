import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { parseCodeGraphFiles } from './codegraph-cli-output'
import { CodeIntelConcurrencyGate } from './codeintel-concurrency-gate'

const filesGate = new CodeIntelConcurrencyGate()

export async function handleFiles(params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)

  const args = ['files', binding.toplevel, '-j']
  if (params.filter) {
    args.push(params.filter)
  }

  const res = await runCodeIntelTool(
    args,
    binding.toplevel,
    ctx.config,
    filesGate,
    { deadline: ctx.deadline, tool: 'codegraph', signal: ctx.signal },
    ctx
  )

  const { files, truncated } = parseCodeGraphFiles(res.stdout, params.limit)

  return {
    files,
    truncated,
    sources: [{ tool: 'codegraph', commit: null, lineBase: 1 }]
  }
}
