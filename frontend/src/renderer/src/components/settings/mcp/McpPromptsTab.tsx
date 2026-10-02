import { useMemo, useState } from 'react'
import { PlusIcon } from 'lucide-react'
import { toast } from 'sonner'
import type { McpPrompt } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { McpPromptEditorDialog, type McpPromptEditorMode } from './McpPromptEditorDialog'
import { McpPromptList } from './McpPromptList'
import { PROMPT_LIMITS } from './mcp-prompt-template'
import { sortMcpPrompts, useMcpPrompts } from './use-mcp-prompts'

const errorText = (e: unknown): string =>
  e instanceof McpRpcError ? e.detail : e instanceof Error ? e.message : String(e)

export function McpPromptsTab(): React.JSX.Element {
  const api = useMcpPrompts()
  const confirm = useConfirmationDialog()
  const [filter, setFilter] = useState('')
  const [editor, setEditor] = useState<McpPromptEditorMode | null>(null)
  const [busy, setBusy] = useState<Set<string>>(new Set())

  const sorted = useMemo(() => sortMcpPrompts(api.data), [api.data])
  const customCount = sorted.filter((p) => !p.builtin).length
  const atLimit = customCount >= PROMPT_LIMITS.customPerTenant
  const names = useMemo(() => new Set(sorted.map((p) => p.name)), [sorted])
  const rows = useMemo(() => {
    const q = filter.trim().toLowerCase()
    return q ? sorted.filter((p) => `${p.name} ${p.description}`.toLowerCase().includes(q)) : sorted
  }, [sorted, filter])

  const onDelete = async (p: McpPrompt): Promise<void> => {
    const ok = await confirm({
      title: translate('auto.mcp.prompts.deleteTitle', 'Delete prompt {{name}}?', { name: p.name }),
      description: translate(
        'auto.mcp.prompts.deleteBody',
        'Users will no longer see it in their MCP clients.'
      ),
      confirmLabel: translate('auto.mcp.prompts.delete', 'Delete'),
      confirmVariant: 'destructive'
    })
    if (!ok) {
      return
    }
    setBusy((s) => new Set(s).add(p.id))
    try {
      await api.remove(p.id)
      toast.success(translate('auto.mcp.prompts.deleted', 'Prompt deleted'))
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusy((s) => {
        const next = new Set(s)
        next.delete(p.id)
        return next
      })
    }
  }

  if (api.status === 'loading') {
    return <McpListSkeleton rows={6} />
  }
  if (api.status === 'forbidden') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.prompts.forbidden', 'You need admin rights to manage prompts.')}
      </McpMutedNote>
    )
  }
  if (api.status === 'unavailable') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.prompts.unavailable', 'MCP is turned off for this organization.')}
      </McpMutedNote>
    )
  }
  if (api.status === 'error') {
    return <McpInlineAlert message={api.error ?? ''} onRetry={api.reload} />
  }

  const newButton = (
    <Button
      size="sm"
      disabled={atLimit}
      title={
        atLimit
          ? translate('auto.mcp.prompts.limit', 'Limit reached ({{max}})', {
              max: PROMPT_LIMITS.customPerTenant
            })
          : undefined
      }
      onClick={() => setEditor({ kind: 'create' })}
    >
      <PlusIcon aria-hidden />
      {translate('auto.mcp.prompts.new', 'New prompt')}
    </Button>
  )

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.mcp.prompts.description',
          'Prompts are reusable instructions users can pick in their MCP client. Prompts cannot grant permissions — tools are still controlled by policies and approvals.'
        )}
      </p>
      <div className="flex items-center gap-2">
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label={translate('auto.mcp.prompts.filter', 'Filter prompts')}
          placeholder={translate('auto.mcp.prompts.filter', 'Filter prompts')}
          className="h-8 max-w-xs"
        />
        {newButton}
      </div>
      {customCount === 0 && !filter ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.prompts.noCustom',
            'No custom prompts yet. Create one to share a workflow with your team.'
          )}
        </McpMutedNote>
      ) : null}
      {rows.length === 0 ? (
        <div className="flex items-center gap-2">
          <McpMutedNote>{translate('auto.mcp.prompts.noMatch', 'No prompts match.')}</McpMutedNote>
          <Button variant="ghost" size="sm" onClick={() => setFilter('')}>
            {translate('auto.mcp.prompts.clear', 'Clear')}
          </Button>
        </div>
      ) : (
        <McpPromptList
          prompts={rows}
          busyIds={busy}
          canCreate={!atLimit}
          onView={(p) => setEditor({ kind: 'view', prompt: p })}
          onEdit={(p) => setEditor({ kind: 'edit', prompt: p })}
          onDuplicate={(p) => setEditor({ kind: 'duplicate', from: p })}
          onDelete={(p) => void onDelete(p)}
        />
      )}
      {editor ? (
        <McpPromptEditorDialog
          mode={editor}
          existingNames={names}
          onClose={() => setEditor(null)}
          onSave={async (p) => {
            await api.save(p)
            toast.success(translate('auto.mcp.prompts.saved', 'Prompt saved'))
          }}
          onReload={api.fetchLatest}
        />
      ) : null}
    </div>
  )
}
