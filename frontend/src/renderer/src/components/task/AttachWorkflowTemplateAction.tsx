import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '../ui/select'
import type { OrcaTask } from '../../../../shared/task-types'

type TemplateOption = { id: string; name: string }

// `workflow.template.list` mixes JSON conventions in the same response object: the
// wrapper (`templates`/`nextPageToken`) is camelCase (built by hand in wscompat), but
// each `WorkflowTemplate` inside is snake_case (protoc-gen-go's default struct tag) —
// map field-by-field, never cast straight to WorkflowTemplate[].
type RawWorkflowTemplateListItem = { id: string; name: string }

function toTemplateOption(raw: unknown): TemplateOption | null {
  const r = raw as Partial<RawWorkflowTemplateListItem>
  if (!r.id || !r.name) {
    return null
  }
  return { id: r.id, name: r.name }
}

export function AttachWorkflowTemplateAction({ task }: { task: OrcaTask }) {
  const [options, setOptions] = useState<TemplateOption[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    setLoading(true)
    callRuntimeRpc<{ templates: unknown[] }>(target, 'workflow.template.list', {})
      .then((res) => {
        if (cancelled) {
          return
        }
        const mapped = (res.templates ?? [])
          .map(toTemplateOption)
          .filter((t): t is TemplateOption => t !== null)
        setOptions(mapped)
      })
      .catch(() => {
        if (cancelled) {
          return
        }
        toast.error('Failed to load workflow templates')
        setOptions([])
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  // Optimistic-only: `UpdateTaskRequest` (task.proto) only has id/title/status, so
  // backend-go silently cannot persist workflowTemplateId today — set the store
  // immediately so the badge (FE-TASK-001) reflects the choice for this session, and
  // best-effort the RPC without letting a failure roll the optimistic state back.
  const attach = async (templateId: string) => {
    useAppStore.getState().updateTask(task.id, { workflowTemplateId: templateId })
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'task.update', {
        taskId: task.id,
        patch: { workflowTemplateId: templateId }
      })
    } catch {
      // Best-effort — task.update → UpdateTaskRequest has no field to persist this yet.
    }
  }

  return (
    <Select value={task.workflowTemplateId ?? ''} onValueChange={attach}>
      <SelectTrigger className="w-[220px]" data-testid="attach-workflow-template-trigger">
        <SelectValue placeholder={loading ? 'Loading templates…' : 'Attach Workflow Template'} />
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => (
          <SelectItem key={o.id} value={o.id}>
            {o.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
