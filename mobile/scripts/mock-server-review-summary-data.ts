import type { RpcRequest, RpcResponse } from './mock-server-rpc-handlers'

type Respond = (response: RpcResponse) => void
type Success = (id: string, result: unknown) => RpcResponse
type ErrorResponse = (id: string, code: string, message: string) => RpcResponse

// Why: lets the review-summary screen and branch-card chip be exercised on a
// device/emulator without a real desktop index. Pick a state with
// MOCK_REVIEW_SUMMARY=ready|stale|truncated|empty|flag_off|no_binding|index_missing|
// tool_unavailable|unsupported|error (default: ready).
export const MOCK_REVIEW_SUMMARY_SCENARIOS = [
  'ready',
  'stale',
  'truncated',
  'empty',
  'flag_off',
  'no_binding',
  'index_missing',
  'tool_unavailable',
  'unsupported',
  'error'
] as const

export type MockReviewSummaryScenario = (typeof MOCK_REVIEW_SUMMARY_SCENARIOS)[number]

function readScenario(raw: string | undefined): MockReviewSummaryScenario {
  return (MOCK_REVIEW_SUMMARY_SCENARIOS as readonly string[]).includes(raw ?? '')
    ? (raw as MockReviewSummaryScenario)
    : 'ready'
}

const FINDINGS = [
  {
    key: 'f-1',
    kind: 'quality',
    severity: 'error',
    title: 'Unhandled promise rejection',
    summary: 'connect() can reject without a catch on the reconnect path.',
    origin: 'introduced',
    filePath: 'src/transport/rpc-client.ts',
    startLine: 42,
    inChangedFiles: true
  },
  {
    key: 'f-2',
    kind: 'structure',
    severity: 'warning',
    title: 'Layer violation',
    summary: 'A UI component imports a transport module directly.',
    origin: 'touched',
    filePath: 'src/components/HostList.tsx',
    startLine: 7,
    inChangedFiles: true
  },
  {
    key: 'f-3',
    kind: 'quality',
    severity: 'info',
    title: 'Long function',
    summary: 'renderRow is longer than the configured limit.',
    origin: 'preexisting',
    filePath: 'src/components/LegacyList.tsx',
    startLine: 120,
    inChangedFiles: false
  }
]

export function buildMockReviewSummary(scenario: MockReviewSummaryScenario): unknown {
  switch (scenario) {
    case 'flag_off':
    case 'no_binding':
    case 'index_missing':
    case 'tool_unavailable':
      return { available: false, reason: scenario }
    case 'empty':
      return {
        available: true,
        index: { state: 'ready', headCommit: 'abc1234' },
        counts: { files: 0, symbols: 0, flows: 0, tables: 0, contracts: 0, uncovered: 0 },
        risk: { level: 'LOW', reasons: [] },
        findings: {
          totalOpen: 0,
          bySeverity: { error: 0, warning: 0, info: 0 },
          items: [],
          truncated: false
        }
      }
    default: {
      const truncated = scenario === 'truncated'
      return {
        available: true,
        index: { state: scenario === 'stale' ? 'stale' : 'ready', headCommit: 'abc1234' },
        counts: { files: 6, symbols: 31, flows: 2, tables: 1, contracts: 1, uncovered: 3 },
        risk: { level: 'HIGH', reasons: ['Touches the reconnect flow used by every session.'] },
        findings: {
          totalOpen: truncated ? 120 : FINDINGS.length,
          bySeverity: truncated
            ? { error: 20, warning: 60, info: 40 }
            : { error: 1, warning: 1, info: 1 },
          items: FINDINGS,
          truncated
        },
        stale: scenario === 'stale',
        truncated,
        headCommit: 'abc1234'
      }
    }
  }
}

export function handleMockReviewSummaryRequest(
  request: RpcRequest,
  respond: Respond,
  success: Success,
  error: ErrorResponse,
  scenarioEnv: string | undefined = process.env.MOCK_REVIEW_SUMMARY
): boolean {
  if (request.method !== 'codeIntel.reviewSummary') {
    return false
  }
  const scenario = readScenario(scenarioEnv)
  if (scenario === 'unsupported') {
    respond(error(request.id, 'method_not_found', 'Unknown method: codeIntel.reviewSummary'))
  } else if (scenario === 'error') {
    respond(error(request.id, 'internal_error', 'Mock review summary failure'))
  } else {
    respond(success(request.id, buildMockReviewSummary(scenario)))
  }
  return true
}
