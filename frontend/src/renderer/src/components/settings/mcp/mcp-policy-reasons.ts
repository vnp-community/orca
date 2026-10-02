import { translate } from '@/i18n/i18n'

const FIXED: Record<string, string> = {
  kill_switch_active: 'The kill switch is active',
  hard_deny: 'This tool is permanently denied',
  tool_descriptor_incomplete: 'The tool description is incomplete',
  mcp_disabled_for_tenant: 'MCP is turned off for the organization',
  client_not_allowed: 'This client is not allowed',
  scope_missing: 'The token does not have the required scope',
  admin_role_required: 'An admin role is required',
  depth_limit: 'Call depth limit reached',
  'risk_floor:exact_tool_policy_required':
    'Allowing this risk level needs a rule for one exact tool',
  open_world_after_untrusted_read: 'Open-world tool after reading untrusted content',
  policy_undefined: 'No policy result was produced',
  policy_unavailable: 'The policy engine is unavailable'
}

/** Maps a server reason code to readable text; unknown codes fall back to the raw code. */
export function reasonLabel(code: string): { text: string; raw: string } {
  const fixed = FIXED[code]
  if (fixed) {
    return {
      text: translate(`auto.mcp.reason.${code.replace(/[^A-Za-z0-9_]/g, '_')}`, fixed),
      raw: code
    }
  }
  const policy = /^policy:([^:]+):(allow|require_approval|deny)$/.exec(code)
  if (policy) {
    return {
      text: translate('auto.mcp.reason.policy', 'Rule {{id}} says {{decision}}', {
        id: policy[1],
        decision: policy[2]
      }),
      raw: code
    }
  }
  const risk = /^risk_default:([^=]+)=(allow|require_approval|deny)$/.exec(code)
  if (risk) {
    return {
      text: translate(
        'auto.mcp.reason.risk_default',
        'Default for {{risk}} tools is {{decision}}',
        {
          risk: risk[1],
          decision: risk[2]
        }
      ),
      raw: code
    }
  }
  return { text: code, raw: code }
}

export function policyIdFromReason(code: string): string | null {
  return /^policy:([^:]+):/.exec(code)?.[1] ?? null
}
