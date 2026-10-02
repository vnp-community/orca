import { useMemo, useRef, useState } from 'react'
import { Loader2Icon, LockIcon } from 'lucide-react'
import type { McpPrompt } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { McpRpcError, parseMcpError } from '@/runtime/runtime-mcp-error'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { McpPromptArgumentsEditor } from './McpPromptArgumentsEditor'
import { McpPromptTemplateField } from './McpPromptTemplateField'
import { promptIssueMessage } from './mcp-prompt-issue-messages'
import { PROMPT_LIMITS, findUnsupportedSyntax } from './mcp-prompt-template'
import {
  hasPromptErrors,
  parseServerPromptError,
  validatePromptDraft,
  type PromptDraft,
  type PromptFieldErrors,
  type PromptIssue
} from './mcp-prompt-validation'

export type McpPromptEditorMode =
  | { kind: 'create' }
  | { kind: 'edit'; prompt: McpPrompt }
  | { kind: 'view'; prompt: McpPrompt }
  | { kind: 'duplicate'; from: McpPrompt }

type Props = {
  mode: McpPromptEditorMode
  /** All current prompt names; the edited prompt's own name is excluded here. */
  existingNames: ReadonlySet<string>
  onSave: (prompt: McpPrompt) => Promise<void>
  /** Fetches the latest stored copy for the version-conflict Reload action. */
  onReload?: (id: string) => Promise<McpPrompt | undefined>
  onClose: () => void
}

function initialDraft(mode: McpPromptEditorMode): PromptDraft {
  if (mode.kind === 'create') {
    return { name: '', description: '', arguments: [], template: '' }
  }
  const src = mode.kind === 'duplicate' ? mode.from : mode.prompt
  const base = {
    name: src.name,
    description: src.description,
    arguments: src.arguments.map((a) => ({ ...a })),
    template: src.template
  }
  return mode.kind === 'duplicate'
    ? { ...base, name: `${src.name}_copy`.slice(0, 48) }
    : { ...base, id: src.id, version: src.version }
}

function FieldError({
  id,
  issues
}: {
  id: string
  issues?: PromptIssue[]
}): React.JSX.Element | null {
  return issues?.length ? (
    <p id={id} role="alert" className="text-xs text-destructive">
      {issues.map(promptIssueMessage).join(' ')}
    </p>
  ) : null
}

export function McpPromptEditorDialog({
  mode,
  existingNames,
  onSave,
  onReload,
  onClose
}: Props): React.JSX.Element {
  const confirm = useConfirmationDialog()
  const initial = useMemo(() => initialDraft(mode), [mode])
  const [draft, setDraft] = useState<PromptDraft>(initial)
  const [baseline, setBaseline] = useState(() => JSON.stringify(initial))
  const [serverErrors, setServerErrors] = useState<PromptFieldErrors>({})
  const [yourChanges, setYourChanges] = useState<PromptDraft | null>(null)
  const [saving, setSaving] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  const readOnly = mode.kind === 'view'
  const locked =
    readOnly || serverErrors.kind === 'not_admin' || serverErrors.kind === 'builtin_readonly'
  const ownName = mode.kind === 'edit' ? mode.prompt.name : null
  const names = useMemo(() => {
    const s = new Set(existingNames)
    if (ownName) {
      s.delete(ownName)
    }
    return s
  }, [existingNames, ownName])
  const clientErrors = useMemo(() => validatePromptDraft(draft, names), [draft, names])
  const dirty = JSON.stringify(draft) !== baseline
  const advanced = mode.kind === 'duplicate' && findUnsupportedSyntax(draft.template).length > 0
  // Server errors stay visible until the user edits again; client rules always apply.
  const merged: PromptFieldErrors = {
    ...clientErrors,
    name: [...(clientErrors.name ?? []), ...(serverErrors.name ?? [])],
    description: [...(clientErrors.description ?? []), ...(serverErrors.description ?? [])],
    template: [...(clientErrors.template ?? []), ...(serverErrors.template ?? [])],
    arguments: { ...serverErrors.arguments, ...clientErrors.arguments }
  }
  const canSave = !locked && !saving && !hasPromptErrors(clientErrors) && draft.name !== ''

  const edit = (patch: Partial<PromptDraft>): void => {
    setDraft((d) => ({ ...d, ...patch }))
    setServerErrors({})
  }

  const requestClose = async (): Promise<void> => {
    if (saving) {
      return
    }
    if (dirty && !readOnly) {
      const ok = await confirm({
        title: translate('auto.mcp.prompts.discardTitle', 'Discard changes?'),
        description: translate(
          'auto.mcp.prompts.discardBody',
          'Your edits to this prompt have not been saved.'
        ),
        confirmLabel: translate('auto.mcp.prompts.discard', 'Discard')
      })
      if (!ok) {
        return
      }
    }
    onClose()
  }

  const save = async (): Promise<void> => {
    setSaving(true)
    setServerErrors({})
    const prev = mode.kind === 'edit' ? mode.prompt : null
    try {
      await onSave({
        id: draft.id ?? '',
        name: draft.name,
        description: draft.description,
        version: draft.version ?? 0,
        updatedAt: prev?.updatedAt ?? '',
        arguments: draft.arguments,
        template: draft.template,
        builtin: false
      })
      onClose()
    } catch (e) {
      const err = e instanceof McpRpcError ? e : parseMcpError(e)
      setServerErrors(parseServerPromptError(err.code, err.detail))
      setSaving(false)
    }
  }

  const reload = async (): Promise<void> => {
    if (!draft.id || !onReload) {
      return
    }
    const latest = await onReload(draft.id)
    if (!latest) {
      return
    }
    setYourChanges(draft)
    const next = initialDraft({ kind: 'edit', prompt: latest })
    setDraft(next)
    setBaseline(JSON.stringify(next))
    setServerErrors({})
  }

  const title =
    mode.kind === 'create'
      ? translate('auto.mcp.prompts.newTitle', 'New prompt')
      : mode.kind === 'view'
        ? translate('auto.mcp.prompts.viewTitle', 'Prompt {{name}}', { name: mode.prompt.name })
        : mode.kind === 'duplicate'
          ? translate('auto.mcp.prompts.duplicateTitle', 'Duplicate prompt')
          : translate('auto.mcp.prompts.editTitle', 'Edit prompt {{name}}', {
              name: mode.prompt.name
            })
  const formIssues = [...(merged.form ?? []), ...(serverErrors.form ?? [])].filter(
    (i, idx, all) => all.indexOf(i) === idx
  )

  return (
    <Dialog open onOpenChange={(open) => !open && void requestClose()}>
      <DialogContent
        className="max-h-[90vh] overflow-y-auto sm:max-w-2xl"
        onOpenAutoFocus={(e) => {
          e.preventDefault()
          nameRef.current?.focus()
        }}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {translate(
              'auto.mcp.prompts.dialogDescription',
              'Name and arguments are shown to users in their MCP client.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {readOnly ? (
            <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <LockIcon className="size-3.5" aria-hidden />
              {translate('auto.mcp.prompts.builtinReadOnly', 'Built-in — read only')}
              {' · '}
              {translate(
                'auto.mcp.prompts.builtinNote',
                'Built-in prompts are maintained by Orca and change with releases. Use Duplicate to start a custom copy.'
              )}
            </p>
          ) : null}
          {advanced ? (
            <p role="status" className="rounded-md border border-border px-3 py-2 text-sm">
              {translate(
                'auto.mcp.prompts.advancedSyntax',
                "This text uses advanced syntax that custom prompts don't support. Edit it before saving."
              )}
            </p>
          ) : null}
          {serverErrors.kind === 'version_conflict' ? (
            <div
              role="alert"
              className="flex items-center justify-between gap-2 rounded-md border border-destructive/40 px-3 py-2 text-sm text-destructive"
            >
              <span>
                {translate(
                  'auto.mcp.prompts.versionConflict',
                  'This prompt was changed by someone else.'
                )}
              </span>
              {onReload ? (
                <Button type="button" variant="outline" size="sm" onClick={() => void reload()}>
                  {translate('auto.mcp.prompts.reload', 'Reload')}
                </Button>
              ) : null}
            </div>
          ) : formIssues.length ? (
            <div
              role="alert"
              className="rounded-md border border-destructive/40 px-3 py-2 text-sm text-destructive"
            >
              {formIssues.map((i, n) => (
                <p key={n}>{promptIssueMessage(i)}</p>
              ))}
            </div>
          ) : null}
          {yourChanges ? (
            <details className="rounded-md border border-border px-3 py-2 text-sm">
              <summary>{translate('auto.mcp.prompts.yourChanges', 'Your changes')}</summary>
              <pre className="mt-2 max-h-40 overflow-auto text-xs break-words whitespace-pre-wrap">
                {yourChanges.template}
              </pre>
            </details>
          ) : null}
          <div className="space-y-1">
            <Label htmlFor="mcp-prompt-name">{translate('auto.mcp.prompts.name', 'Name')}</Label>
            <Input
              ref={nameRef}
              id="mcp-prompt-name"
              value={draft.name}
              readOnly={locked}
              onChange={(e) => edit({ name: e.target.value })}
              aria-invalid={merged.name?.length ? true : undefined}
              aria-describedby="mcp-prompt-name-err"
              className="font-mono"
            />
            <FieldError
              id="mcp-prompt-name-err"
              issues={draft.name === '' ? serverErrors.name : merged.name}
            />
            {ownName && ownName !== draft.name ? (
              <p className="text-xs text-muted-foreground">
                {translate(
                  'auto.mcp.prompts.renameWarning',
                  'Clients that saved this prompt by name will no longer find it.'
                )}
              </p>
            ) : null}
          </div>
          <div className="space-y-1">
            <Label htmlFor="mcp-prompt-description">
              {translate('auto.mcp.prompts.fieldDescription', 'Description')}
            </Label>
            <Input
              id="mcp-prompt-description"
              value={draft.description}
              readOnly={locked}
              onChange={(e) => edit({ description: e.target.value })}
              aria-invalid={merged.description?.length ? true : undefined}
              aria-describedby="mcp-prompt-description-err"
            />
            <div className="flex justify-between">
              <FieldError id="mcp-prompt-description-err" issues={merged.description} />
              <span className="ml-auto text-xs text-muted-foreground">{`${draft.description.length} / ${PROMPT_LIMITS.description}`}</span>
            </div>
          </div>
          <div className="space-y-1">
            <Label>{translate('auto.mcp.prompts.arguments', 'Arguments')}</Label>
            <McpPromptArgumentsEditor
              args={draft.arguments}
              readOnly={locked}
              errors={merged.arguments}
              onChange={(args) => edit({ arguments: args })}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="mcp-prompt-template">
              {translate('auto.mcp.prompts.template', 'Template')}
            </Label>
            <McpPromptTemplateField
              value={draft.template}
              readOnly={locked}
              argNames={draft.arguments.map((a) => a.name)}
              issues={merged.template}
              onChange={(template) => edit({ template })}
            />
          </div>
          {mode.kind === 'edit' || mode.kind === 'view' ? (
            <p className="text-xs text-muted-foreground">
              {translate('auto.mcp.prompts.versionLine', 'Version {{version}}', {
                version: draft.version ?? mode.prompt.version
              })}
            </p>
          ) : null}
        </div>
        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => void requestClose()}>
            {readOnly
              ? translate('auto.mcp.prompts.close', 'Close')
              : translate('auto.mcp.prompts.cancel', 'Cancel')}
          </Button>
          {readOnly ? null : (
            <Button type="button" disabled={!canSave} onClick={() => void save()}>
              {saving ? <Loader2Icon className="animate-spin" aria-hidden /> : null}
              {translate('auto.mcp.prompts.save', 'Save')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
