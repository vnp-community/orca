import { useState, useEffect, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { WorkflowDefinition } from '@shared/workflow-types'

// Giá trị THẬT khớp workflow.proto's WorkflowTemplate.scope comment ("company"|"team"|
// "personal") — KHÔNG dùng WorkflowScope (đó là khái niệm khác, scope cục bộ của 1
// WorkflowDefinition trong Builder, có 'project' thay vì 'team').
export type LibraryScope = 'company' | 'team' | 'personal'

export function useWorkflowLibrary(scope: LibraryScope, search: string) {
  const [templates, setTemplates] = useState<WorkflowDefinition[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError(false)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<{ templates: WorkflowDefinition[] }>(
        target,
        'workflow.template.list',
        { scope }
      )
      // workflow.template.list KHÔNG hỗ trợ search server-side (chưa có SearchTemplates,
      // BE-SOL-005 📋 Proposed) — lọc client-side tạm thời trên tập đã load; nâng cấp sang
      // server-side search khi BE-SOL-005 merge (không đổi chữ ký hook, chỉ đổi thân hàm này).
      const filtered = search.trim()
        ? result.templates.filter((t) => t.name.toLowerCase().includes(search.trim().toLowerCase()))
        : result.templates
      setTemplates(filtered)
    } catch {
      setLoadError(true)
    } finally {
      setLoading(false)
    }
  }, [scope, search])

  useEffect(() => {
    void load()
  }, [load])

  return { templates, loading, loadError, reload: load }
}
