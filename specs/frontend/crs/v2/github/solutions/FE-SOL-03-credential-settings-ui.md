# FE-SOL-03: Credential Settings UI — Bitbucket, Azure DevOps, Gitea, Linear, Jira (Web mode)

> **CRs:** CR-INT-002, CR-INT-003, CR-INT-004  
> **Backend SOL tương ứng:** SOL-04-Credential-Store  
> **TDD:** TDD-FE-05 (UI Components), TDD-FE-06 (Web Client)  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Tasks:** [FE-TASK-01](../tasks/FE-TASK-01-preload-api-types.md), [FE-TASK-05](../tasks/FE-TASK-05-credential-input-form.md), [FE-TASK-06](../tasks/FE-TASK-06-token-cards-credential-form.md), [FE-TASK-07](../tasks/FE-TASK-07-task-tracker-credential-form.md)

---

## Vấn đề

Hiện tại trong Web mode, các integration cards (Bitbucket, Azure DevOps, Gitea, Linear, Jira) chỉ hiển thị hướng dẫn set biến môi trường:

```
Set ORCA_BITBUCKET_EMAIL and ORCA_BITBUCKET_API_TOKEN, or set ORCA_BITBUCKET_ACCESS_TOKEN
```

Trong Web mode (multi-user), người dùng không thể set env vars. Backend đã implement `WebCredentialStore` và `credentials.*` RPC methods. Cần UI để người dùng nhập credentials trực tiếp.

---

## Thiết kế giải pháp

### 1. Shared `CredentialInputForm` Component

```typescript
// src/renderer/src/components/settings/CredentialInputForm.tsx [NEW]

type CredentialField = {
  key: string
  label: string
  placeholder: string
  type: 'text' | 'password' | 'url'
  required: boolean
}

type CredentialInputFormProps = {
  service: 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'
  fields: CredentialField[]
  onSave: (values: Record<string, string>) => Promise<void>
  onRevoke: () => Promise<void>
  isConfigured: boolean
}

export function CredentialInputForm({
  service,
  fields,
  onSave,
  onRevoke,
  isConfigured
}: CredentialInputFormProps): React.JSX.Element {
  const [values, setValues] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSave = async () => {
    setSaving(true)
    setError(null)
    try {
      await onSave(values)
      setValues({}) // clear sensitive fields after save
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save credentials')
    } finally {
      setSaving(false)
    }
  }

  const handleRevoke = async () => {
    if (!confirm(`Remove ${service} credentials?`)) return
    await onRevoke()
  }

  return (
    <div className="flex flex-col gap-3">
      {fields.map(field => (
        <div key={field.key} className="flex flex-col gap-1">
          <label className="text-xs font-medium text-muted-foreground">
            {field.label}
            {field.required && <span className="text-destructive ml-1">*</span>}
          </label>
          <input
            type={field.type}
            placeholder={field.placeholder}
            value={values[field.key] ?? ''}
            onChange={e => setValues(v => ({ ...v, [field.key]: e.target.value }))}
            className="h-8 rounded-md border bg-background px-3 text-sm"
          />
        </div>
      ))}
      {error && <p className="text-xs text-destructive">{error}</p>}
      <div className="flex gap-2">
        <Button
          variant="default"
          size="sm"
          disabled={saving}
          onClick={handleSave}
        >
          {saving ? <Loader2 className="size-3.5 mr-1.5 animate-spin" /> : null}
          Save Credentials
        </Button>
        {isConfigured && (
          <Button variant="ghost" size="sm" onClick={handleRevoke}>
            Revoke
          </Button>
        )}
      </div>
    </div>
  )
}
```

### 2. Hook `useCredentialManager` — wrapper cho `credentials.*` RPC

```typescript
// src/renderer/src/hooks/useCredentialManager.ts [NEW]

type CredentialService = 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'

type CredentialStatus = {
  configured: boolean
  mode: 'web' | 'electron'
  config?: Record<string, string>
}

export function useCredentialManager(service: CredentialService) {
  const [status, setStatus] = useState<CredentialStatus | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    window.api.credentials
      .status(service)
      .then(setStatus)
      .finally(() => setLoading(false))
  }, [service])

  const save = useCallback(
    async (token: string, config?: Record<string, string>) => {
      await window.api.credentials.set(service, token, config)
      // Refresh status
      const updated = await window.api.credentials.status(service)
      setStatus(updated)
    },
    [service]
  )

  const revoke = useCallback(async () => {
    await window.api.credentials.revoke(service)
    setStatus(s => s ? { ...s, configured: false, config: undefined } : s)
  }, [service])

  return { status, loading, save, revoke }
}
```

### 3. Cập nhật `BitbucketIntegrationCard` — Web mode credential input

```typescript
// src/renderer/src/components/settings/token-source-control-integration-cards.tsx [MODIFY]

export function BitbucketIntegrationCard(): React.JSX.Element {
  const { statuses, unavailable, refresh } = usePreflightCardStatuses('bitbucket')
  const status = unavailable ? 'unavailable' : statuses.bitbucketStatus
  const connected = status === 'connected'
  const { status: credStatus, save, revoke } = useCredentialManager('bitbucket')
  const isWebMode = status === 'unavailable' || credStatus?.mode === 'web'

  // ... existing card shell ...
  
  // THÊM VÀO phần not-configured trong Web mode:
  {status === 'not-configured' && isWebMode ? (
    <CredentialInputForm
      service="bitbucket"
      isConfigured={credStatus?.configured ?? false}
      fields={[
        { key: 'token', label: 'App Password / Access Token', placeholder: 'atb-...', type: 'password', required: true },
        { key: 'email', label: 'Email', placeholder: 'user@example.com', type: 'text', required: true },
        { key: 'apiBaseUrl', label: 'API Base URL (optional)', placeholder: 'https://api.bitbucket.org/2.0', type: 'url', required: false },
      ]}
      onSave={async (values) => {
        await save(values.token, { email: values.email, apiBaseUrl: values.apiBaseUrl })
        refresh()
      }}
      onRevoke={async () => {
        await revoke()
        refresh()
      }}
    />
  ) : status === 'not-configured' ? (
    // Electron mode: giữ nguyên env var instructions
    <EnvVarInstructions service="bitbucket" />
  ) : null}
}
```

### 4. Tương tự cho Azure DevOps, Gitea, Linear, Jira

**Azure DevOps fields:**
```typescript
fields = [
  { key: 'token', label: 'Personal Access Token', type: 'password', required: true },
  { key: 'username', label: 'Username (optional)', type: 'text', required: false },
  { key: 'apiBaseUrl', label: 'API Base URL', placeholder: 'https://dev.azure.com', type: 'url', required: false },
]
```

**Gitea fields:**
```typescript
fields = [
  { key: 'token', label: 'API Token', type: 'password', required: true },
  { key: 'apiBaseUrl', label: 'Gitea Base URL', placeholder: 'https://gitea.example.com', type: 'url', required: true },
]
```

**Linear fields:**
```typescript
fields = [
  { key: 'token', label: 'Linear API Key', placeholder: 'lin_api_...', type: 'password', required: true },
]
```

**Jira fields:**
```typescript
fields = [
  { key: 'token', label: 'API Token', type: 'password', required: true },
  { key: 'email', label: 'Email', type: 'text', required: true },
  { key: 'apiBaseUrl', label: 'Jira Base URL', placeholder: 'https://yourorg.atlassian.net', type: 'url', required: true },
]
```

### 5. Web Preload API — `credentials.*` đã expose

```typescript
// src/renderer/src/web/web-preload-api.ts [VERIFY — đã implement]

credentials: {
  set: (service, token, config?) =>
    callRuntimeResult<{ success: boolean }>('credentials.set', { service, token, config }),
  revoke: (service) =>
    callRuntimeResult<{ success: boolean }>('credentials.revoke', { service }),
  status: (service) =>
    callRuntimeResult<{ configured: boolean; mode: string; config?: Record<string, string> }>(
      'credentials.status', { service }
    ),
  list: () =>
    callRuntimeResult<{ services: string[]; mode: string }>('credentials.list', null)
}
```

---

## Files cần thay đổi

### [NEW] `src/renderer/src/components/settings/CredentialInputForm.tsx`
- Form component cho credential input
- Hỗ trợ nhiều fields (token, email, baseUrl, v.v.)
- Gọi `onSave(values)` → `window.api.credentials.set(...)`
- Nút Revoke → `onRevoke()` → `window.api.credentials.revoke(...)`
- Clear password fields sau khi save

### [NEW] `src/renderer/src/hooks/useCredentialManager.ts`
- Hook wrapper cho `credentials.status`, `credentials.set`, `credentials.revoke`
- Auto-fetch status on mount
- Trả về `{ status, loading, save, revoke }`

### [MODIFY] `src/renderer/src/components/settings/token-source-control-integration-cards.tsx`
- `BitbucketIntegrationCard`: thêm Web mode credential form
- `AzureDevOpsIntegrationCard`: thêm Web mode credential form
- `GiteaIntegrationCard`: thêm Web mode credential form

### [MODIFY] `src/renderer/src/components/settings/task-tracker-integration-cards.tsx`
- `LinearIntegrationCard`: thêm Web mode credential form
- `JiraIntegrationCard`: thêm Web mode credential form

### [VERIFY] `src/renderer/src/web/web-preload-api.ts`
- Verify `credentials.set/revoke/status/list` available (đã implement trong backend tasks)

---

## Acceptance Criteria

1. ✅ Trong Web mode + `not-configured` → hiển thị form nhập credentials (không phải env var instructions)
2. ✅ Nhập credentials → click Save → `credentials.set(service, token, config)` → trạng thái cập nhật
3. ✅ Card refresh sau khi save (`onSaved` → `credRefresh()` + `refresh()`)
4. ✅ Revoke credentials → card trở về "Not configured" (`onRevoked` → same refresh)
5. ✅ Password fields bị clear sau khi save (`setValues({})` sau save thành công)
6. ✅ Trong Electron mode → giữ nguyên env var instructions (`isWebMode` check)
7. ✅ Error handling: required fields missing → validation error trước khi submit

---

## Implementation Verified

| File | Thay đổi | Status |
|------|---------|--------|
| `src/preload/api-types.ts` | `credentials` namespace added | ✅ FE-TASK-01 |
| `src/renderer/src/web/web-preload-api.ts` | `credentials.*` methods exposed | ✅ FE-TASK-01 |
| `src/renderer/src/components/settings/CredentialInputForm.tsx` | NEW (189 lines) | ✅ FE-TASK-05 |
| `token-source-control-integration-cards.tsx` | Bitbucket + AzDO + Gitea web mode form | ✅ FE-TASK-06 |
| `task-tracker-integration-cards.tsx` | Linear web mode form | ✅ FE-TASK-07 |
| `jira-integration-card.tsx` | Jira web mode form | ✅ FE-TASK-07 |

---

## Security Notes

- Password fields dùng `type="password"` + `autoComplete="current-password"` — không visible
- Sau khi save thành công, clear `values` state (`setValues({})`) → token không còn trong React tree
- `credentials.status` không trả về token — chỉ trả về `configured: true/false` và safe config fields
- Token được mã hóa bằng AES-256-GCM bởi `WebCredentialStore` (xem SOL-04)
- SessionManager inject credentials vào env của child process tại spawn time (transparent với integration clients)

