// Why: how desktop main reaches api-gateway is undecided (CONTRACT-open-issues O-4),
// so the host depends on this port instead of a gateway client. The default
// port reports "unavailable" until a real gateway-backed port is registered.
export type CodeIntelSummaryPortRiskReason = {
  messageKey: string
  params?: Record<string, string>
}

export type CodeIntelSummaryPortOverlay = {
  changedFiles: { path: string }[]
  changedSymbols?: unknown[]
  affectedFlows?: unknown[]
  touchedTables?: unknown[]
  touchedContracts?: unknown[]
  uncoveredSymbols?: unknown[]
  risk: { level: string; reasons: CodeIntelSummaryPortRiskReason[] }
  indexFreshness?: {
    state?: string
    indexedCommit?: string
    headOid?: string
    generatedAt?: string
  }
  limits?: { totalCounts?: Record<string, number> }
}

export type CodeIntelSummaryPortFinding = {
  findingKey: string
  rule: string
  kind: string
  severity: string
  titleKey: string
  params?: Record<string, string>
  origin: string
  evidence?: { path: string; line?: number }[]
}

export type CodeIntelSummaryPortFindings = {
  findings: CodeIntelSummaryPortFinding[]
  totalCount?: number
}

export type CodeIntelSummaryPortStatus = {
  overall: string
}

export type CodeIntelSummaryPort = {
  getOverlay: (worktreeId: string) => Promise<CodeIntelSummaryPortOverlay>
  getFindings: (
    worktreeId: string,
    options: { limit: number; scope: 'changed' }
  ) => Promise<CodeIntelSummaryPortFindings>
  getStatus: (worktreeId: string) => Promise<CodeIntelSummaryPortStatus>
}

export class CodeIntelPortError extends Error {
  constructor(
    readonly code: string,
    message?: string
  ) {
    super(message ?? code)
    this.name = 'CodeIntelPortError'
  }
}

// Why: default until O-4 is decided; every call is "service unavailable".
export const NoGatewayCodeIntelPort: CodeIntelSummaryPort = {
  getOverlay: () => Promise.reject(new CodeIntelPortError('CODEINTEL_UNAVAILABLE')),
  getFindings: () => Promise.reject(new CodeIntelPortError('CODEINTEL_UNAVAILABLE')),
  getStatus: () => Promise.reject(new CodeIntelPortError('CODEINTEL_UNAVAILABLE'))
}

let activePort: CodeIntelSummaryPort = NoGatewayCodeIntelPort

export function getCodeIntelSummaryPort(): CodeIntelSummaryPort {
  return activePort
}

export function setCodeIntelSummaryPort(port: CodeIntelSummaryPort | null): void {
  activePort = port ?? NoGatewayCodeIntelPort
}
