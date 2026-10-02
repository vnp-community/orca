import { translate } from '@/i18n/i18n'
import { mcpRiskLabel } from '@/lib/mcp-labels'
import type { McpPolicyIssue } from './mcp-policy-form'

export function policyIssueText(issue: McpPolicyIssue): string {
  switch (issue.kind) {
    case 'no_dimension':
      return translate('auto.mcp.policies.issue.noDimension', 'Choose at least one thing to match.')
    case 'hard_deny':
      return translate(
        'auto.mcp.policies.issue.hardDeny',
        'Permanently denied tools cannot be allowed or sent for approval: {{tools}}.',
        { tools: issue.tools.join(', ') }
      )
    case 'exact_tool_required':
      return translate(
        'auto.mcp.policies.issue.exactTool',
        'Allowing {{risks}} tools needs a rule for one exact tool.',
        { risks: issue.risks.map(mcpRiskLabel).join(', ') }
      )
    default:
      return translate(
        'auto.mcp.policies.issue.adminRole',
        'Admin tools still require an admin role even when allowed.'
      )
  }
}
