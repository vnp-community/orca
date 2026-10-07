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
  BACKLOG_EXECUTE: 'backlog.execute'
} as const

export type RequestRpcMethod = (typeof REQUEST_RPC_METHODS)[keyof typeof REQUEST_RPC_METHODS]
