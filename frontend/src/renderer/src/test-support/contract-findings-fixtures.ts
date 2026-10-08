/**
 * contract-findings-fixtures.ts — fake backend data for the Contract lens and the Findings panel
 * (CONTRACT-codeintel-ui-api §4.5). Used by FE-CV-TASK-059-* tests.
 */

import type { ContractChange, ContractDiff, Finding } from '../../../shared/code-intel-types'

export function makeContractChange(overrides: Partial<ContractChange> = {}): ContractChange {
  return {
    id: 'c1',
    kind: 'proto-field',
    name: 'RelayByDevServer.timeout_ms',
    service: 'infra-fleet-service',
    change: 'removed',
    compatibility: 'breaking',
    ruleId: 'proto.field.removed',
    details: { before: 'int32 timeout_ms = 3;' },
    files: ['proto/relay.proto'],
    consumers: [],
    evidence: [{ path: 'proto/relay.proto', line: 12, kind: 'proto' }],
    ...overrides
  }
}

export const CONTRACT_CHANGES_MIXED: ContractChange[] = [
  makeContractChange(),
  makeContractChange({
    id: 'c2',
    kind: 'proto-rpc',
    name: 'Relay.Start',
    compatibility: 'compatible',
    change: 'added',
    ruleId: 'proto.rpc.added',
    details: {},
    files: ['proto/relay.proto']
  }),
  makeContractChange({
    id: 'c3',
    kind: 'ws-channel',
    name: 'codeIntel.findings',
    service: 'api-gateway',
    change: 'modified',
    compatibility: 'risky',
    ruleId: 'ws.arg.optional-added',
    details: { before: 'findings(sel)', after: 'findings(sel, scope?)' },
    files: ['gateway/ws.go']
  }),
  makeContractChange({
    id: 'c4',
    kind: 'route',
    name: 'GET /v1/things',
    service: 'api-gateway',
    change: 'modified',
    compatibility: 'unknown',
    ruleId: 'route.unclassified',
    details: { note: 'dsn=postgres://user:hunter2@db/prod' },
    files: ['gateway/routes.go']
  }),
  makeContractChange({
    id: 'c5',
    kind: 'sql-column',
    name: 'orders.tenant_id',
    service: undefined,
    compatibility: 'breaking',
    ruleId: 'sql.column.dropped',
    details: {},
    files: ['db/migrations/0042.sql']
  })
]

export const CONTRACT_DIFF_MIXED: ContractDiff = {
  scope: { baseRef: 'main', mode: 'worktree', includesUncommitted: true },
  summary: { breaking: 2, risky: 1, compatible: 1, unknown: 1 },
  changes: CONTRACT_CHANGES_MIXED,
  migrations: [],
  truncated: false,
  totalCount: 5
}

export function makeFinding(overrides: Partial<Finding> = {}): Finding {
  return {
    findingKey: 'layer_violation:a->b',
    rule: 'layer.violation',
    kind: 'layer_violation',
    severity: 'error',
    titleKey: 'finding.layer_violation.title',
    params: { from: 'handlers', to: 'repository' },
    subject: 'handlers -> repository',
    evidence: [{ path: 'svc/handlers/order.go', line: 40 }],
    metrics: {},
    scope: { service: 'order-service' },
    origin: 'introduced',
    confidence: 'high',
    ...overrides
  }
}

export const FINDINGS_MIXED: Finding[] = [
  makeFinding(),
  makeFinding({
    findingKey: 'hotspot:order.go',
    rule: 'hotspot',
    kind: 'hotspot',
    severity: 'warning',
    titleKey: 'finding.hotspot.title',
    origin: 'touched',
    evidence: [{ path: 'svc/order.go' }]
  }),
  makeFinding({
    findingKey: 'dead:util',
    rule: 'dead.code',
    kind: 'dead_code',
    severity: 'info',
    titleKey: 'finding.dead_code.title',
    origin: 'preexisting',
    confidence: 'low'
  }),
  makeFinding({
    findingKey: 'tenant:orders',
    rule: 'sql.missing_tenant_id',
    kind: 'missing_tenant_id',
    severity: 'error',
    titleKey: 'finding.missing_tenant_id.title',
    params: { table: 'orders' },
    origin: 'introduced'
  })
]
