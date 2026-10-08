/**
 * Request RPC Methods — v6 (CR-REQ-018)
 *
 * Canonical list of all request-service RPC method names as defined in
 * CR-REQ-016 §2.2. Keep in sync with the backend gateway channels doc.
 * No `request.flowSet` — that is a backend-internal operation.
 *
 * @module shared/request-rpc-methods
 */

export const REQUEST_RPC_METHODS = {
  // Flow management
  FLOW_STATUS: 'request.flowStatus',

  // Request CRUD
  CREATE: 'request.create',
  GET: 'request.get',
  LIST: 'request.list',
  CANCEL: 'request.cancel',
  REOPEN: 'request.reopen',
  RETURN_TO_BACKLOG: 'request.returnToBacklog',

  // Classification
  CLASSIFY: 'request.classify',
  CONFIRM_TYPE: 'request.confirmType',
  CHANGE_TYPE: 'request.changeType',
  TYPE_HISTORY: 'request.typeHistory',

  // Relationships
  LINKS: 'request.links',
  SPAWN_CHILD: 'request.spawnChild',

  // Plan / execution
  GENERATE_PLAN: 'request.generatePlan',
  START_PHASE: 'request.startPhase',

  // Subscriptions
  SUBSCRIBE: 'request.subscribe',

  // Solution
  SOLUTION_LIST: 'solution.list',
  SOLUTION_GENERATE: 'solution.generate',
  SOLUTION_CHOOSE: 'solution.choose',

  // Approval
  APPROVAL_LIST: 'approval.list',
  APPROVAL_LIST_PENDING: 'approval.listPending',
  APPROVAL_APPROVE: 'approval.approve',
  APPROVAL_REJECT: 'approval.reject',

  // Backlog
  BACKLOG_REQUESTS: 'backlog.requests',
  BACKLOG_TASKS: 'backlog.tasks',
  BACKLOG_EXECUTE: 'backlog.execute',

  // Impact / risk graph (CR-REQ-030 2.8; channel names pending CONTRACT)
  IMPACT_GRAPH: 'impact.graph',
  IMPACT_HEATMAP: 'impact.heatmap',
  IMPACT_DRIFT: 'impact.drift',
  IMPACT_GET: 'impact.get',
  IMPACT_REQUEST: 'impact.request',
  IMPACT_COMPARE: 'impact.compare',
  IMPACT_FINDINGS: 'impact.findings',
  IMPACT_EVIDENCE: 'impact.evidence',
  IMPACT_ACCEPT: 'impact.accept',
  RISK_OVERRIDE: 'risk.override',

  // Clarification / decision (CR-REQ-028 section 9)
  CLARIFICATION_LIST: 'clarification.list',
  CLARIFICATION_GET: 'clarification.get',
  CLARIFICATION_ANSWER: 'clarification.answer',
  CLARIFICATION_CANCEL: 'clarification.cancel',
  REQUEST_READINESS: 'request.readiness',
  DECISION_LIST: 'decision.list',
  DECISION_CONFIRM: 'decision.confirm',

  // Task readiness / execution result (CR-REQ-029; `execution.get` is a proposed name)
  READINESS_CHECK: 'readiness.check',
  READINESS_GET: 'readiness.get',
  READINESS_LIST: 'readiness.list',
  EXECUTION_GET: 'execution.get'
} as const

export type RequestRpcMethod = (typeof REQUEST_RPC_METHODS)[keyof typeof REQUEST_RPC_METHODS]
