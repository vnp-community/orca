import { useRef, useState } from 'react'
import type { McpScopeDescriptor, McpScopeId, McpToken } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { isHighRisk } from '@/lib/mcp-risk'
import { mcpScopeDescription, mcpScopeLabel } from '@/lib/mcp-labels'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { trackMcpTokenCreated } from '@/lib/mcp-telemetry'
import { McpInlineAlert } from './McpListStates'
import { McpTokenSecretReveal } from './McpTokenSecretReveal'
import {
  TOKEN_NAME_MAX,
  defaultExpiryDays,
  expiryOptions,
  hasFormErrors,
  isScopeSelectable,
  mapCreateError,
  validateTokenForm,
  type TokenFormErrors
} from './mcp-token-form'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Metadata only. The secret never leaves this component. */
  onCreated: (token: McpToken) => void
  maxTokenDays: number | undefined
  scopes: McpScopeDescriptor[]
  role: string | undefined
  resourceUrl: string
  createBlocked: boolean
  onRefreshServerInfo?: () => void
}

type Reveal = { token: McpToken; secret: string }

export function McpCreateTokenDialog(props: Props): React.JSX.Element {
  const { open, onOpenChange, onCreated, maxTokenDays, scopes, role, resourceUrl } = props
  const maxOk = typeof maxTokenDays === 'number' && maxTokenDays > 0
  const [name, setName] = useState('')
  const [selected, setSelected] = useState<McpScopeId[]>(['orca:read'])
  const [pickedDays, setDays] = useState<number | null>(null)
  // Why: a refreshed (lower) cap must also pull a previously picked lifetime back inside it.
  const days = maxOk
    ? Math.min(pickedDays ?? defaultExpiryDays(maxTokenDays), maxTokenDays)
    : (pickedDays ?? 1)
  // Why: the too-long text is derived at render so it shows the refreshed cap, not the stale one.
  const [tooLong, setTooLong] = useState(false)
  const [errors, setErrors] = useState<TokenFormErrors>({})
  const [banner, setBanner] = useState<string | null>(null)
  const [blocked, setBlocked] = useState(false)
  const [creating, setCreating] = useState(false)
  const [reveal, setReveal] = useState<Reveal | null>(null)
  const [saved, setSaved] = useState(false)
  const inFlight = useRef(false)

  const close = (): void => {
    // Why: drop the secret from memory the moment the dialog is dismissed.
    setReveal(null)
    setSaved(false)
    setName('')
    setSelected(['orca:read'])
    setErrors({})
    setTooLong(false)
    setBanner(null)
    onOpenChange(false)
  }

  const submit = async (): Promise<void> => {
    if (inFlight.current || !maxOk) {
      return
    }
    const errs = validateTokenForm({ name, scopes: selected, expiresInDays: days }, maxTokenDays)
    setErrors(errs)
    setTooLong(false)
    if (hasFormErrors(errs)) {
      return
    }
    inFlight.current = true
    setCreating(true)
    setBanner(null)
    try {
      const res = await mcpClient.call('mcp.token.create', {
        name: name.trim(),
        scopes: selected,
        expiresInDays: days
      })
      trackMcpTokenCreated({ lifetimeDays: days, scopeCount: selected.length })
      onCreated(res.token)
      setReveal({ token: res.token, secret: res.secret })
    } catch (e) {
      const err = parseMcpError(e)
      const effect = mapCreateError(err.code, err.detail, maxTokenDays)
      if (err.code === 'MCP_TOKEN_TOO_LONG') {
        setErrors({})
        setTooLong(true)
      } else if (effect.field) {
        setErrors({ [effect.field]: effect.message })
      }
      if (effect.banner) {
        setBanner(effect.message)
      }
      if (effect.disableCreate) {
        setBlocked(true)
      }
      if (effect.refreshServerInfo) {
        props.onRefreshServerInfo?.()
      }
    } finally {
      inFlight.current = false
      setCreating(false)
    }
  }

  const toggleScope = (id: McpScopeId, on: boolean): void =>
    setSelected((prev) => (on ? [...new Set([...prev, id])] : prev.filter((s) => s !== id)))

  const expiryError =
    errors.expiresInDays ??
    (tooLong && maxOk
      ? translate('auto.mcp.tokens.errTooLong', 'Maximum lifetime is {{days}} days.', {
          days: maxTokenDays
        })
      : null)
  const revealing = reveal !== null
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        // The reveal step cannot be dismissed until the user confirms they saved the token.
        if (!next && revealing && !saved) {
          return
        }
        if (!next) {
          close()
        }
      }}
    >
      <DialogContent
        // Why: the reveal step is taller than a 720px viewport; without this its Done button is unreachable.
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto"
        showCloseButton={!revealing}
        onEscapeKeyDown={(e) => revealing && !saved && e.preventDefault()}
        onInteractOutside={(e) => revealing && !saved && e.preventDefault()}
      >
        {reveal ? (
          <McpTokenSecretReveal
            token={reveal.token}
            secret={reveal.secret}
            resourceUrl={resourceUrl}
            saved={saved}
            onSavedChange={setSaved}
            onDone={close}
          />
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>
                {translate('auto.mcp.tokens.createTitle', 'Create access token')}
              </DialogTitle>
              <DialogDescription>
                {translate(
                  'auto.mcp.tokens.createBody',
                  "Tokens let scripts and CI agents call Orca's MCP server without a browser."
                )}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4">
              {banner ? <McpInlineAlert message={banner} /> : null}
              <div className="space-y-1.5">
                <Label htmlFor="mcp-token-name">{translate('auto.mcp.tokens.name', 'Name')}</Label>
                <Input
                  id="mcp-token-name"
                  autoFocus
                  maxLength={TOKEN_NAME_MAX}
                  value={name}
                  placeholder={translate('auto.mcp.tokens.namePlaceholder', 'CI pipeline')}
                  aria-invalid={!!errors.name}
                  onChange={(e) => setName(e.target.value)}
                />
                {errors.name ? <p className="text-xs text-destructive">{errors.name}</p> : null}
              </div>
              <fieldset className="space-y-2">
                <legend className="mb-1 text-sm font-medium">
                  {translate('auto.mcp.tokens.permissions', 'Permissions')}
                </legend>
                {scopes.map((s) => {
                  const id = `mcp-token-scope-${s.id.replace(/[^A-Za-z0-9]/g, '_')}`
                  const selectable = isScopeSelectable(s.id, role)
                  return (
                    <div key={s.id} className="flex items-start gap-3">
                      <Checkbox
                        id={id}
                        className="mt-1"
                        aria-describedby={`${id}-desc`}
                        disabled={!selectable}
                        checked={selected.includes(s.id as McpScopeId)}
                        onCheckedChange={(v) => toggleScope(s.id as McpScopeId, v === true)}
                      />
                      <Label
                        htmlFor={id}
                        className="flex-1 flex-col items-start gap-0.5 font-normal"
                      >
                        <span className="text-sm font-medium">
                          {mcpScopeLabel(s)}
                          {isHighRisk(s.risk)
                            ? ` · ${translate('auto.mcp.risk.highRisk', 'High risk')}`
                            : ''}
                        </span>
                        <span id={`${id}-desc`} className="text-xs text-muted-foreground">
                          {selectable
                            ? mcpScopeDescription(s)
                            : translate(
                                'auto.mcp.tokens.adminOnly',
                                'Only administrators can grant this.'
                              )}
                        </span>
                      </Label>
                    </div>
                  )
                })}
                {errors.scopes ? <p className="text-xs text-destructive">{errors.scopes}</p> : null}
              </fieldset>
              <div className="space-y-1.5">
                <Label htmlFor="mcp-token-expiry">
                  {translate('auto.mcp.tokens.expiresIn', 'Expires in')}
                </Label>
                <Select
                  value={String(days)}
                  onValueChange={(v) => {
                    setDays(Number(v))
                    setTooLong(false)
                  }}
                >
                  <SelectTrigger id="mcp-token-expiry" className="w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(maxOk ? expiryOptions(maxTokenDays) : []).map((d) => (
                      <SelectItem key={d} value={String(d)}>
                        {translate('auto.mcp.tokens.days', '{{count}} days', {
                          count: d
                        })}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {maxOk ? (
                  <p className="text-xs text-muted-foreground">
                    {translate(
                      'auto.mcp.tokens.maxHelp',
                      'Your organization allows up to {{days}} days.',
                      { days: maxTokenDays }
                    )}
                  </p>
                ) : null}
                {expiryError ? <p className="text-xs text-destructive">{expiryError}</p> : null}
              </div>
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={close}>
                {translate('auto.mcp.common.cancel', 'Cancel')}
              </Button>
              <Button
                disabled={creating || !maxOk || props.createBlocked || blocked}
                onClick={() => void submit()}
              >
                {creating
                  ? translate('auto.mcp.tokens.creating', 'Creating…')
                  : translate('auto.mcp.tokens.create', 'Create token')}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
