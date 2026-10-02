import { useState } from 'react'
import type { McpToolPolicy } from '../../../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { buildUpsertPayload, type McpPolicyDraft } from './mcp-policy-form'

export type McpPolicyEditorFailure =
  | { kind: 'hard_deny'; message: string }
  | { kind: 'conflict'; latest: McpToolPolicy }
  | { kind: 'not_found' }
  | { kind: 'forbidden' }
  | { kind: 'other'; message: string }

/** Save flow with optimistic concurrency: a version conflict never overwrites unless asked. */
export function useMcpPolicyEditor(onSaved: (p: McpToolPolicy) => void) {
  const [saving, setSaving] = useState(false)
  const [failure, setFailure] = useState<McpPolicyEditorFailure | null>(null)

  const save = async (draft: McpPolicyDraft): Promise<void> => {
    setSaving(true)
    setFailure(null)
    try {
      onSaved(await mcpClient.call('mcp.admin.policy.upsert', buildUpsertPayload(draft)))
    } catch (e) {
      const err = parseMcpError(e)
      if (err.code === 'MCP_POLICY_VERSION_CONFLICT') {
        try {
          const latest = (await mcpClient.call('mcp.admin.policy.list')).find(
            (p) => p.id === draft.id
          )
          setFailure(latest ? { kind: 'conflict', latest } : { kind: 'not_found' })
        } catch (e2) {
          setFailure({ kind: 'other', message: parseMcpError(e2).detail })
        }
      } else if (err.code === 'MCP_POLICY_HARD_DENY') {
        setFailure({ kind: 'hard_deny', message: err.detail })
      } else if (err.code === 'MCP_NOT_FOUND') {
        setFailure({ kind: 'not_found' })
      } else if (err.code === 'MCP_NOT_ADMIN') {
        setFailure({ kind: 'forbidden' })
      } else {
        setFailure({ kind: 'other', message: err.detail })
      }
    } finally {
      setSaving(false)
    }
  }

  return { save, saving, failure, clearFailure: () => setFailure(null) }
}
