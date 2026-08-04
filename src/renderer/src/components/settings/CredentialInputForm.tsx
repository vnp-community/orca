import { useCallback, useEffect, useState } from 'react'
import { Loader2, Save, Trash2 } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import { Tracers } from '../../../../shared/trace/tracers'

// Why: In Web Server mode (ORCA_MULTI_USER=1), users cannot configure integrations
// via env vars (shared process). This component provides a per-user credential input
// form that calls credentials.set(service, token, config) → WebCredentialStore (AES-256-GCM).
// The token is stored encrypted on the server; the child process receives it via env injection
// from SessionManager at spawn time (transparent to integration clients).
// (FE-SOL-03 — CR-INT-002, CR-INT-003, CR-INT-004)

type CredentialService = 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'

export type CredentialField = {
  key: string
  label: string
  placeholder: string
  type: 'text' | 'password' | 'url'
  required: boolean
}

type CredentialInputFormProps = {
  service: CredentialService
  fields: CredentialField[]
  isConfigured: boolean
  onSaved: () => void    // callback after successful save (triggers preflight refresh)
  onRevoked: () => void  // callback after successful revoke
}

export function CredentialInputForm({
  service,
  fields,
  isConfigured,
  onSaved,
  onRevoked,
}: CredentialInputFormProps): React.JSX.Element {
  const [values, setValues] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const [revoking, setRevoking] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  const handleSave = async () => {
    // Validate required fields
    const missing = fields.filter(f => f.required && !values[f.key]?.trim())
    if (missing.length > 0) {
      setError(`Required: ${missing.map(f => f.label).join(', ')}`)
      return
    }

    setSaving(true)
    setError(null)
    // Why: span bọc trước validate token/config để cover cả case validate fail —
    // nhưng KHÔNG đưa token/config vào TraceFields (bảo mật, CR-TRACE-014 §4).
    const span = Tracers.uiRemoteIntegrationCredentialStoreFlow.start({ service, op: 'set' })
    try {
      // Extract main token (first password field, or 'token' key)
      const tokenKey = fields.find(f => f.type === 'password')?.key ?? 'token'
      const token = values[tokenKey] ?? ''

      // Build config from remaining fields
      const config: Record<string, string> = {}
      for (const field of fields) {
        if (field.key !== tokenKey && values[field.key]?.trim()) {
          config[field.key] = values[field.key].trim()
        }
      }

      await window.api.credentials.set(service, token, Object.keys(config).length ? config : undefined)

      // Clear sensitive data from state after successful save
      setValues({})
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
      onSaved()
      span.ok({ service })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save credentials')
      span.fail(err, { service })
    } finally {
      setSaving(false)
    }
  }

  const handleRevoke = async () => {
    if (!confirm(`Remove ${service} credentials? This cannot be undone.`)) return
    setRevoking(true)
    const span = Tracers.uiRemoteIntegrationCredentialStoreFlow.start({ service, op: 'revoke' })
    try {
      await window.api.credentials.revoke(service)
      onRevoked()
      span.ok({ service })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke credentials')
      span.fail(err, { service })
    } finally {
      setRevoking(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 pt-1">
      {fields.map(field => (
        <div key={field.key} className="flex flex-col gap-1">
          <Label className="text-xs font-medium">
            {field.label}
            {field.required && <span className="text-destructive ml-1">*</span>}
          </Label>
          <Input
            id={`credential-${service}-${field.key}`}
            type={field.type}
            placeholder={field.placeholder}
            value={values[field.key] ?? ''}
            onChange={e => setValues(v => ({ ...v, [field.key]: e.target.value }))}
            className="h-7 text-xs"
            autoComplete={field.type === 'password' ? 'current-password' : 'off'}
          />
        </div>
      ))}

      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}

      {saved && (
        <p className="text-xs text-green-600 dark:text-green-400">✓ Credentials saved</p>
      )}

      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={saving}
          onClick={handleSave}
          id={`credential-${service}-save-btn`}
        >
          {saving ? (
            <Loader2 className="size-3.5 mr-1.5 animate-spin" />
          ) : (
            <Save className="size-3.5 mr-1.5" />
          )}
          Save
        </Button>

        {isConfigured && (
          <Button
            variant="ghost"
            size="sm"
            disabled={revoking}
            onClick={handleRevoke}
            id={`credential-${service}-revoke-btn`}
          >
            {revoking ? (
              <Loader2 className="size-3.5 mr-1.5 animate-spin" />
            ) : (
              <Trash2 className="size-3.5 mr-1.5" />
            )}
            Revoke
          </Button>
        )}
      </div>
    </div>
  )
}

// ── useCredentialManager hook ─────────────────────────────────────────────────

type CredentialStatus = {
  configured: boolean
  mode: string
  config?: Record<string, string>
}

export function useCredentialManager(service: CredentialService) {
  const [status, setStatus] = useState<CredentialStatus | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(() => {
    window.api.credentials
      .status(service)
      .then(setStatus)
      .catch(() => setStatus(null))
      .finally(() => setLoading(false))
  }, [service])

  useEffect(() => {
    refresh()
  }, [refresh])

  const isWebMode = status?.mode === 'web'
  const isConfigured = status?.configured ?? false

  return { status, loading, isWebMode, isConfigured, refresh }
}
