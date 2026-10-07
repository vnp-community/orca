import { QualityParserInput, QualityParserOutput, QualityParser } from './quality-parser-types'
import { oxlintParser } from './quality-parser-oxlint'
import { tscParser } from './quality-parser-tsc'
import { vitestParser } from './quality-parser-vitest'
import { goVetParser } from './quality-parser-go-vet'
import { goTestParser } from './quality-parser-go-test'
import { golangciParser } from './quality-parser-golangci'
import { bufLintParser, bufBreakingParser } from './quality-parser-buf'
import { opaParser } from './quality-parser-opa'
import { maxLinesRatchetParser, styledScrollbarsParser, reliabilityGatesParser } from './quality-parser-orca-check'

import { runFindingPipeline, inferStepStatus, PipelineContext, QualityFinding } from './quality-finding-pipeline'

export const PARSER_REGISTRY: Record<string, QualityParser> = {
  'oxlint': oxlintParser,
  'tsc': tscParser,
  'vitest': vitestParser,
  'go-vet': goVetParser,
  'go-test': goTestParser,
  'golangci-lint': golangciParser,
  'buf-lint': bufLintParser,
  'buf-breaking': bufBreakingParser,
  'opa-test': opaParser,
  'orca-check-max-lines': maxLinesRatchetParser,
  'orca-check-styled-scrollbars': styledScrollbarsParser,
  'orca-check-reliability-gates': reliabilityGatesParser
}

export function getParser(key: string): QualityParser | undefined {
  return PARSER_REGISTRY[key]
}

export interface StepContext {
  stepId: string
  tool: string
  toolVersion: string
  repoRoot: string
  cwd: string
  home?: string
  tmp?: string
  platform: 'linux' | 'darwin' | 'win32'
  scopeFiles: string[] | null
  readSourceLine: (file: string, line: number) => Promise<string | null>
  remainingRunBudget: number
  exitCode: number
  findingsExitCodes: number[]
  skipDriftGuard?: boolean
  timeout?: boolean
  cancelled?: boolean
}

export interface ManagerResult {
  stepResult: any // QualityStepResult
  findings: QualityFinding[]
}

export function createStepParser(parserKey: string, ctx: StepContext) {
  return {
    name: parserKey,
    parse: async (stdoutPath: string, stderrPath: string): Promise<ManagerResult> => {
      const parser = getParser(parserKey)
      if (!parser) {
        return {
          stepResult: {
            id: ctx.stepId,
            tool: ctx.tool,
            state: 'failed',
            failureKind: 'parser_error',
            durationMs: 0
          },
          findings: []
        }
      }

      if (ctx.timeout) {
        return {
          stepResult: {
            id: ctx.stepId, tool: ctx.tool, state: 'timeout', durationMs: 0
          }, findings: []
        }
      }
      if (ctx.cancelled) {
        return {
          stepResult: {
            id: ctx.stepId, tool: ctx.tool, state: 'cancelled', durationMs: 0
          }, findings: []
        }
      }

      const input: QualityParserInput = {
        stepId: ctx.stepId,
        stdoutPath,
        stderrPath,
        exitCode: ctx.exitCode,
        timedOut: false,
        cancelled: false,
        cwd: ctx.cwd,
        repoRoot: ctx.repoRoot,
        platform: ctx.platform,
        toolVersion: ctx.toolVersion,
        scopeFiles: ctx.scopeFiles,
        readSourceLine: ctx.readSourceLine
      }

      const output = await parser.parse(input)
      
      const pipelineCtx: PipelineContext = {
        stepId: ctx.stepId,
        tool: ctx.tool,
        toolVersion: ctx.toolVersion,
        repoRoot: ctx.repoRoot,
        cwd: ctx.cwd,
        home: ctx.home,
        tmp: ctx.tmp,
        scopeFiles: ctx.scopeFiles,
        readSourceLine: ctx.readSourceLine,
        remainingRunBudget: ctx.remainingRunBudget
      }

      const { findings, counts, truncated, outsideRepoCount } = await runFindingPipeline(output.findings, pipelineCtx)

      const status = inferStepStatus({
        exitCode: ctx.exitCode,
        hasFindings: findings.length > 0,
        failureKind: output.failure ? output.failure.kind : null,
        envReason: output.failure?.envReason,
        skipDriftGuard: ctx.skipDriftGuard,
        findingsExitCodes: ctx.findingsExitCodes
      })

      return {
        stepResult: {
          id: ctx.stepId,
          tool: ctx.tool,
          state: status.status,
          failureKind: status.failureKind,
          envReason: status.envReason,
          durationMs: 0 // to be filled by caller
        },
        findings
      }
    }
  }
}
