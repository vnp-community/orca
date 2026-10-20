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
  /** The flow of one request type and size (analysis, plan, gates), straight from the backend registry. */
  FLOW: 'request.flow',

  // Request CRUD
  CREATE: 'request.create',
  /** CR-REQ-039: change the branch a Request's worktrees start from, while no worktree exists. */
  SET_BASE_REF: 'request.setBaseRef',
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
  // Result of a background `generatePlan` propose run (CONTRACT Q2, option a)
  PLAN_PROPOSAL: 'request.planProposal',
  /** CR-REQ-045: one file of the OpenSpec change behind a Plan version. */
  PLAN_BUNDLE_FILE: 'request.planBundleFile',
  /** CR-REQ-045: the versions of a Plan, newest first, with what was asked for each. */
  PLAN_REVISIONS: 'request.planRevisions',
  /** CR-REQ-044: start the server-side Plan generation again after it gave up (or commit a Plan that was written but not saved). */
  RETRY_PLAN: 'request.retryPlan',
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
  CLARIFICATION_REQUEST: 'clarification.request',
  REQUEST_READINESS: 'request.readiness',
  DECISION_LIST: 'decision.list',
  DECISION_CONFIRM: 'decision.confirm',

  // Task readiness / execution result (CR-REQ-029; `execution.get` is a proposed name)
  READINESS_CHECK: 'readiness.check',
  READINESS_GET: 'readiness.get',
  READINESS_LIST: 'readiness.list',
  EXECUTION_GET: 'execution.get',

  // Per-project engine and execution-mode switches (admin writes); CR-REQ-043 reads and writes the whole set
  ENGINE_SETTINGS: 'request.engineSettings',
  ENGINE_SETTINGS_SET: 'request.engineSettingsSet',

  // Request detail read models (CR-REQ-038)
  ACTIVITY: 'request.activity',
  EXECUTION_SUMMARY: 'request.executionSummary',

  // Outcome Review (CR-OR-001..030): a review is always read through its Request
  REVIEW_ESTIMATE: 'request.review.estimate',
  REVIEW_GENERATE: 'request.review.generate',
  REVIEW_GET: 'request.review.get',
  REVIEW_RUNS: 'request.review.runs',
  REVIEW_ARTIFACT: 'request.review.artifact',
  REVIEW_REVISIONS: 'request.review.revisions',
  REVIEW_REGENERATE: 'request.review.regenerate',
  REVIEW_RESTORE: 'request.review.restore',
  REVIEW_COMPARE: 'request.review.compare',
  REVIEW_DECIDE: 'request.review.decide',
  REVIEW_APPROVE: 'request.review.approve',
  REVIEW_DECISIONS: 'request.review.decisions',
  REVIEW_LINK_CHILD: 'request.review.linkChild',
  // Test specs generated next to the code (docs/integrate/TDD-REQ-TESTSPEC-001)
  TEST_SPECS: 'request.testSpecs',
  TEST_SPECS_START: 'request.testSpecsStart',
  TEST_SPEC_DOCUMENT: 'request.testSpecDocument'
} as const

export type RequestRpcMethod = (typeof REQUEST_RPC_METHODS)[keyof typeof REQUEST_RPC_METHODS]
