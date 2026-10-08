import { describe, expect, it } from 'vitest'
import { classifyApprovalDecisionError } from './approval-decision-outcome'

const rpc = (message: string, code = 'internal') => ({ code, message })

describe('classifyApprovalDecisionError', () => {
  const table: Array<[string, string]> = [
    ['APPROVAL_ALREADY_DECIDED', 'closed'], ['APPROVAL_EXPIRED', 'closed'], ['APPROVAL_NOT_FOUND', 'closed'],
    ['REQUEST_NOT_FOUND', 'closed'],
    ['APPROVAL_VERSION_CONFLICT', 'changed'], ['APPROVAL_DIGEST_MISMATCH', 'changed'],
    ['APPROVAL_STAGE_MISMATCH', 'changed'],
    ['APPROVAL_NOT_APPROVER', 'forbidden'], ['APPROVAL_FORBIDDEN', 'forbidden'],
    ['APPROVAL_SELF_APPROVAL_FORBIDDEN', 'forbidden'], ['APPROVAL_AGENT_FORBIDDEN', 'forbidden'],
    ['APPROVAL_COMMENT_REQUIRED', 'validation'], ['SOMETHING_ELSE', 'unknown']
  ]
  it.each(table)('%s -> %s for both prefixes', (code, outcome) => {
    const bare = code.replace(/^REQUEST_/, '')
    expect(classifyApprovalDecisionError(rpc(`${bare}: x`)).outcome).toBe(outcome)
    expect(classifyApprovalDecisionError(rpc(`REQUEST_${bare}: x`)).outcome).toBe(outcome)
  })

  it('maps method_not_found to unsupported and transport errors to network', () => {
    expect(classifyApprovalDecisionError({ code: 'method_not_found', message: 'nope' }).outcome).toBe('unsupported')
    expect(classifyApprovalDecisionError(new Error('network down')).outcome).toBe('network')
  })

  it('falls back to unknown for plain errors', () => {
    expect(classifyApprovalDecisionError(new Error('boom')).outcome).toBe('unknown')
    expect(classifyApprovalDecisionError('weird').outcome).toBe('unknown')
  })
})
