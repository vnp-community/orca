import { useEffect, useMemo, useRef, useState } from 'react'
import { Loader2Icon, TriangleAlertIcon } from 'lucide-react'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { McpExternalServerArgsEditor } from './McpExternalServerArgsEditor'
import { McpExternalServerRefRows } from './McpExternalServerRefRows'
import { McpExternalServerTeamSelect } from './McpExternalServerTeamSelect'
import { scopeLabel } from './McpExternalServerStatus'
import type { McpExternalServersApi } from './use-external-servers'
import {
  buildUpsertBody,
  draftFromServer,
  editNeedsReReview,
  emptyServerDraft,
  externalServerErrorText,
  hasServerErrors,
  validateServerDraft,
  type ServerDraft
} from './mcp-external-server-validation'

type Props = {
  server: McpExternalServer | null
  isAdmin: boolean
  api: Pick<McpExternalServersApi, 'upsert' | 'setSecret'>
  onClose: () => void
}

const SCOPES: McpExternalServer['scope'][] = ['tenant', 'team', 'user']
const refKey = (kind: 'env' | 'header', name: string): string => `${kind}:${name}`

export function McpExternalServerDialog({
  server,
  isAdmin,
  api,
  onClose
}: Props): React.JSX.Element {
  const initial = useMemo(
    () => (server ? draftFromServer(server) : emptyServerDraft(isAdmin)),
    [server, isAdmin]
  )
  const [draft, setDraft] = useState<ServerDraft>(initial)
  const [base, setBase] = useState<McpExternalServer | null>(server)
  const [understood, setUnderstood] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [saving, setSaving] = useState(false)
  const [banner, setBanner] = useState<string | null>(null)
  const [serverFieldErr, setServerFieldErr] = useState<{ url?: string; name?: string }>({})
  const [staged, setStaged] = useState<ReadonlySet<string>>(new Set())
  const [rowErrors, setRowErrors] = useState<ReadonlyMap<string, string>>(new Map())
  // Why: plaintext secrets live only in this ref (never state/store) until Save sends them once.
  const secrets = useRef(new Map<string, string>())
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const held = secrets.current
    return () => held.clear()
  }, [])

  const errors = validateServerDraft(draft)
  const isStdio = draft.transport === 'stdio'
  const reReview = base?.status === 'approved' && editNeedsReReview(initial, draft)
  const canSave = !saving && (!isStdio || understood)
  const patch = (p: Partial<ServerDraft>): void => {
    setDraft((d) => ({ ...d, ...p }))
    setServerFieldErr({})
    setBanner(null)
  }

  const refProps = (kind: 'env' | 'header') => {
    const names = kind === 'env' ? draft.envNames : draft.headerNames
    const refs = (kind === 'env' ? base?.envRefs : base?.headerRefs) ?? []
    const prefix = `${kind}:`
    return {
      noun:
        kind === 'env'
          ? translate('auto.mcp.external.nounEnv', 'environment variable')
          : translate('auto.mcp.external.nounHeader', 'header'),
      names,
      savedNames: new Set(refs.map((r) => r.name)),
      hasSecret: new Map(refs.map((r) => [r.name, r.hasSecret] as const)),
      staged: new Set(
        [...staged].filter((k) => k.startsWith(prefix)).map((k) => k.slice(prefix.length))
      ),
      rowErrors: new Map(
        [...rowErrors]
          .filter(([k]) => k.startsWith(prefix))
          .map(([k, v]) => [k.slice(prefix.length), v])
      ),
      error: submitted ? errors[kind === 'env' ? 'envNames' : 'headerNames'] : undefined,
      onAdd: (n: string) =>
        patch(kind === 'env' ? { envNames: [...names, n] } : { headerNames: [...names, n] }),
      onRemove: (n: string) => {
        secrets.current.delete(refKey(kind, n))
        setStaged((s) => new Set([...s].filter((k) => k !== refKey(kind, n))))
        patch(
          kind === 'env'
            ? { envNames: names.filter((x) => x !== n) }
            : { headerNames: names.filter((x) => x !== n) }
        )
      },
      onStage: (n: string, v: string) => {
        secrets.current.set(refKey(kind, n), v)
        setStaged((s) => new Set(s).add(refKey(kind, n)))
      },
      onUnstage: (n: string) => {
        secrets.current.delete(refKey(kind, n))
        setStaged((s) => new Set([...s].filter((k) => k !== refKey(kind, n))))
      }
    }
  }

  const save = async (): Promise<void> => {
    setSubmitted(true)
    setBanner(null)
    if (hasServerErrors(errors) || !canSave) {
      return
    }
    setSaving(true)
    let saved: McpExternalServer
    try {
      saved = await api.upsert(buildUpsertBody({ ...draft, id: draft.id ?? base?.id }))
    } catch (e) {
      const err = parseMcpError(e)
      const text = externalServerErrorText(err.code, err.detail)
      if (err.code === 'MCP_SERVER_SSRF_BLOCKED') {
        setServerFieldErr({ url: text })
      } else if (err.code === 'MCP_SERVER_NAME_CONFLICT') {
        setServerFieldErr({ name: text })
      } else {
        setBanner(text)
      }
      setSaving(false)
      return
    }
    setBase(saved)
    setDraft((d) => ({ ...d, id: saved.id }))
    const failures = new Map<string, string>()
    for (const key of Array.from(secrets.current.keys())) {
      const value = secrets.current.get(key) ?? ''
      // Cleared before and regardless of the outcome: a retry means re-entering the value.
      secrets.current.delete(key)
      const [kind, ...rest] = key.split(':')
      try {
        await api.setSecret({
          serverId: saved.id,
          kind: kind as 'env' | 'header',
          name: rest.join(':'),
          value
        })
      } catch (e) {
        const err = parseMcpError(e)
        failures.set(key, externalServerErrorText(err.code, err.detail))
      }
    }
    setStaged(new Set())
    setRowErrors(failures)
    setSaving(false)
    if (failures.size === 0) {
      onClose()
    } else {
      setBanner(
        translate(
          'auto.mcp.external.secretsFailed',
          'The server was saved, but some secrets were not stored. Enter them again and save.'
        )
      )
    }
  }

  const title = server
    ? translate('auto.mcp.external.editTitle', 'Edit server {{name}}', { name: server.name })
    : translate('auto.mcp.external.addTitle', 'Add external MCP server')
  const fieldErr = (m?: string): React.JSX.Element | null =>
    m ? (
      <p role="alert" className="text-xs text-destructive">
        {m}
      </p>
    ) : null

  return (
    <Dialog open onOpenChange={(open) => !open && !saving && onClose()}>
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
              'auto.mcp.external.dialogDescription',
              'Agents started by Orca can use this server once an admin has approved it.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4" aria-live="polite">
          {banner ? (
            <div
              role="alert"
              className="rounded-md border border-destructive/40 px-3 py-2 text-sm text-destructive"
            >
              {banner}
            </div>
          ) : null}
          {reReview ? (
            <p role="status" className="rounded-md border border-border px-3 py-2 text-sm">
              {translate(
                'auto.mcp.external.reReview',
                'Changing this will send the server back to review.'
              )}
            </p>
          ) : null}
          <div className="space-y-1">
            <Label htmlFor="mcp-ext-name">{translate('auto.mcp.external.name', 'Name')}</Label>
            <Input
              ref={nameRef}
              id="mcp-ext-name"
              value={draft.name}
              onChange={(e) => patch({ name: e.target.value })}
              aria-invalid={submitted && (errors.name || serverFieldErr.name) ? true : undefined}
              className="font-mono"
            />
            {fieldErr(serverFieldErr.name ?? (submitted ? errors.name : undefined))}
          </div>
          <div className="space-y-1">
            <Label>{translate('auto.mcp.external.scopeField', 'Scope')}</Label>
            {isAdmin ? (
              <ToggleGroup
                type="single"
                variant="outline"
                size="sm"
                value={draft.scope}
                disabled={Boolean(server)}
                onValueChange={(v) =>
                  v && patch({ scope: v as ServerDraft['scope'], scopeId: undefined })
                }
                aria-label={translate('auto.mcp.external.scopeField', 'Scope')}
              >
                {SCOPES.map((s) => (
                  <ToggleGroupItem key={s} value={s}>
                    {scopeLabel(s)}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            ) : (
              <p className="text-sm">{scopeLabel('user')}</p>
            )}
            {isAdmin && draft.scope === 'team' ? (
              <McpExternalServerTeamSelect
                value={draft.scopeId}
                disabled={Boolean(server)}
                onChange={(id) => patch({ scopeId: id })}
              />
            ) : null}
            {fieldErr(submitted ? errors.team : undefined)}
          </div>
          <div className="space-y-1">
            <Label>{translate('auto.mcp.external.transport', 'Transport')}</Label>
            <ToggleGroup
              type="single"
              variant="outline"
              size="sm"
              value={draft.transport}
              onValueChange={(v) => {
                if (v) {
                  setUnderstood(false)
                  patch({ transport: v as ServerDraft['transport'] })
                }
              }}
              aria-label={translate('auto.mcp.external.transport', 'Transport')}
            >
              <ToggleGroupItem value="http">HTTP</ToggleGroupItem>
              <ToggleGroupItem value="stdio">stdio</ToggleGroupItem>
            </ToggleGroup>
          </div>
          {isStdio ? (
            <>
              <div
                role="alert"
                className="space-y-2 rounded-md border border-destructive px-3 py-2 text-sm"
              >
                <p className="flex items-center gap-2 font-medium text-destructive">
                  <TriangleAlertIcon className="size-4" aria-hidden />
                  {translate('auto.mcp.external.stdioWarnTitle', 'This runs a program')}
                </p>
                <p>
                  {translate(
                    'auto.mcp.external.stdioWarn',
                    'This runs a program on the machine where the agent runs, with the permissions of the agent. Only an admin can approve it, and it only runs if your organization has enabled stdio servers.'
                  )}
                </p>
                <p className="text-muted-foreground">
                  {translate(
                    'auto.mcp.external.stdioAlways',
                    'stdio servers always need admin review.'
                  )}
                </p>
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="mcp-ext-understand"
                    checked={understood}
                    onCheckedChange={(c) => setUnderstood(c === true)}
                  />
                  <Label htmlFor="mcp-ext-understand">
                    {translate(
                      'auto.mcp.external.stdioUnderstand',
                      'I understand this will execute code'
                    )}
                  </Label>
                </div>
              </div>
              <div className="space-y-1">
                <Label htmlFor="mcp-ext-command">
                  {translate('auto.mcp.external.command', 'Command')}
                </Label>
                <Input
                  id="mcp-ext-command"
                  value={draft.command}
                  placeholder="npx"
                  onChange={(e) => patch({ command: e.target.value })}
                  className="font-mono"
                />
                {fieldErr(submitted ? errors.command : undefined)}
              </div>
              <div className="space-y-1">
                <Label>{translate('auto.mcp.external.args', 'Arguments')}</Label>
                <McpExternalServerArgsEditor
                  args={draft.args}
                  onChange={(args) => patch({ args })}
                />
              </div>
              <div className="space-y-1">
                <Label>{translate('auto.mcp.external.envNames', 'Environment variables')}</Label>
                <McpExternalServerRefRows {...refProps('env')} />
              </div>
            </>
          ) : (
            <>
              <div className="space-y-1">
                <Label htmlFor="mcp-ext-url">{translate('auto.mcp.external.url', 'URL')}</Label>
                <Input
                  id="mcp-ext-url"
                  value={draft.url}
                  placeholder="https://mcp.example.com/mcp"
                  onChange={(e) => patch({ url: e.target.value })}
                  aria-invalid={submitted && (errors.url || serverFieldErr.url) ? true : undefined}
                  className="font-mono"
                />
                {fieldErr(serverFieldErr.url ?? (submitted ? errors.url : undefined))}
              </div>
              <div className="space-y-1">
                <Label>{translate('auto.mcp.external.headerNames', 'Headers')}</Label>
                <McpExternalServerRefRows {...refProps('header')} />
              </div>
            </>
          )}
          <p className="text-xs text-muted-foreground">
            {translate(
              'auto.mcp.external.secretNote',
              'Sent over TLS and encrypted at rest by the server.'
            )}
          </p>
        </div>
        <DialogFooter>
          <Button type="button" variant="ghost" disabled={saving} onClick={onClose}>
            {translate('auto.mcp.external.cancel', 'Cancel')}
          </Button>
          <Button type="button" disabled={!canSave} onClick={() => void save()}>
            {saving ? <Loader2Icon className="animate-spin" aria-hidden /> : null}
            {translate('auto.mcp.external.save', 'Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
