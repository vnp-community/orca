/**
 * Request artifact parsers — FE-REQ-TASK-036-01
 *
 * Never throw. Unknown enums become 'unknown'; missing `id` returns null so
 * callers drop the item. JSON-in-string fields are parsed when possible and
 * kept raw when broken. Split by domain; this barrel keeps the import path stable.
 *
 * @module shared/request-artifact-parsers
 */

export { parseClarification } from './request-clarification-parsers'
export { parseDecision } from './request-decision-parsers'
export {
  parseImpactComparison,
  parseImpactDrift,
  parseImpactFinding,
  parseImpactSummary
} from './request-impact-parsers'
export { parseExecutionResult, parseTaskReadinessReport } from './request-task-run-parsers'
