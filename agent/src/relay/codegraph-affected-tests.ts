import { CodeIntelRequestContext } from './codeintel-method-table'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { parseCodeGraphAffected } from './codegraph-cli-output'
import { CodeIntelConcurrencyGate } from './codeintel-concurrency-gate'

const affectedGate = new CodeIntelConcurrencyGate()

export async function getAffectedTests(
  binding: { toplevel: string },
  files: string[],
  ctx: CodeIntelRequestContext
): Promise<{ changedFiles: string[], affectedTests: any[], truncated: boolean }> {
  // Validate files: no `-` at start
  const validFiles = files.filter(f => !f.startsWith('-'))
  if (validFiles.length === 0) {
    return { changedFiles: [], affectedTests: [], truncated: false }
  }

  // Chunk by 500
  const CHUNK_SIZE = 500
  const chunks = []
  for (let i = 0; i < validFiles.length; i += CHUNK_SIZE) {
    chunks.push(validFiles.slice(i, i + CHUNK_SIZE))
  }

  const allChangedFiles = new Set<string>()
  const allTestsMap = new Map<string, any>()

  for (const chunk of chunks) {
    const stdinText = chunk.join('\n') + '\n'
    
    // codegraph affected --stdin -p <root> -j -d 5
    const res = await runCodeIntelTool(
      ['affected', '--stdin', '-p', binding.toplevel, '-j', '-d', '5'],
      binding.toplevel,
      ctx.config,
      affectedGate,
      { deadline: ctx.deadline, tool: 'codegraph', signal: ctx.signal, stdinText },
      ctx
    )

    const parsed = parseCodeGraphAffected(res.stdout)
    for (const cf of parsed.changedFiles) {
      allChangedFiles.add(cf)
    }
    for (const test of parsed.affectedTests) {
      const key = `${test.file}::${test.name}`
      if (!allTestsMap.has(key)) {
        allTestsMap.set(key, test)
      }
    }

    if (allTestsMap.size >= 200) {
      break
    }
  }

  let affectedTests = Array.from(allTestsMap.values())
  let truncated = false
  if (affectedTests.length > 200) {
    affectedTests = affectedTests.slice(0, 200)
    truncated = true
  }

  return {
    changedFiles: Array.from(allChangedFiles),
    affectedTests,
    truncated
  }
}
