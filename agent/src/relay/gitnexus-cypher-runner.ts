import { CodeIntelError } from './codeintel-errors'
import { CypherTemplate, CYPHER_TEMPLATES } from './gitnexus-cypher-templates'
import { cypherInt, cypherString, cypherStringList, cypherKindList } from './gitnexus-cypher-literal'
import { assertReadOnlyCypher } from './gitnexus-cypher-guard'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { parseCypherOutput, CypherRows } from './gitnexus-cypher-markdown-parser'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import { CodeIntelRequestContext } from './codeintel-method-table'

export function renderCypherTemplate(template: CypherTemplate, slots: Record<string, any>): string {
  let rendered = template.text

  for (const [key, type] of Object.entries(template.slots)) {
    if (!(key in slots)) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Missing slot value: ${key}`)
    }

    const value = slots[key]
    let encoded = ''

    if (type === 'int') {
      encoded = cypherInt(value, 0, 1_000_000_000)
    } else if (type === 'string') {
      encoded = cypherString(value)
    } else if (type === 'stringList') {
      encoded = cypherStringList(value)
    } else if (type === 'kindList') {
      encoded = cypherKindList(value)
    }

    rendered = rendered.replace(new RegExp(`\\{\\{${key}\\}\\}`, 'g'), encoded)
  }

  if (/\{\{.*?\}\}/.test(rendered)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Template rendered with unreplaced slots')
  }

  return assertReadOnlyCypher(rendered)
}

export async function runCypherTemplate(
  binding: CodeIntelRepoBinding,
  templateId: string,
  slots: Record<string, any>,
  ctx: CodeIntelRequestContext
): Promise<CypherRows> {
  const template = CYPHER_TEMPLATES[templateId]
  if (!template) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', `Unknown template: ${templateId}`)
  }

  const { assertReadable } = await import('./codeintel-reindex-read-guard')
  assertReadable(binding, 'gitnexus')

  const query = renderCypherTemplate(template, slots)

  const timeoutMs = Math.max(100, ctx.deadline - Date.now())

  const { getCodeIntelConcurrencyGate } = await import('./codeintel-concurrency-gate')
  const { buildGitNexusArgv } = await import('./codeintel-command-whitelist')
  const gate = getCodeIntelConcurrencyGate()
  const argv = buildGitNexusArgv({ verb: 'cypher', query }, binding.gitNexusRegistryPath || binding.toplevel)

  const tmpPerf: any[] = []
  const toolResult = await runCodeIntelTool(
    argv,
    binding.mainCheckoutRoot,
    ctx.config,
    gate,
    { tool: 'gitnexus', deadline: ctx.deadline, signal: ctx.signal },
    { perf: tmpPerf }
  )

  if (tmpPerf.length > 0) {
    ctx.perf.recordCli(tmpPerf[0])
  }

  return parseCypherOutput(toolResult.stdout, template)
}
