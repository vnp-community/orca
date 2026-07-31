# TDD-FE-13: AI Provider Admin UI

**Document:** TDD-FE-13 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** AI Provider — provider list, credential input, health status, quota chart
**Feature:** F35
**ADR:** ADR-008
**HLD Ref:** C3.11a
**Backend TDD:** TDD-16
**Source files (to create):**
- `src/renderer/src/components/ai-provider/ProviderList.tsx`
- `src/renderer/src/components/ai-provider/ProviderForm.tsx`
- `src/renderer/src/components/ai-provider/CredentialInput.tsx`
- `src/renderer/src/components/ai-provider/HealthStatusBadge.tsx`
- `src/renderer/src/components/ai-provider/UsageChart.tsx`
- `src/renderer/src/hooks/useAIProviders.ts`

> **Status: ❌ TODO** — v5.0 proposed; critical: credential NEVER logged or sent in plaintext

---

## 1. ProviderList Component

```typescript
// src/renderer/src/components/ai-provider/ProviderList.tsx

// Page: /admin/ai-providers (Admin SPA)
// or: Settings > AI Providers tab

// Layout:
// ┌──────────────────────────────────────────────────────────────────┐
// │ AI Provider Accounts                    [+ Add Account]          │
// │ Filter: [All Servers ▼] [All Scopes ▼] [All Statuses ▼]        │
// ├──────────────────────────────────────────────────────────────────┤
// │ Provider    Label         Server         Scope   Status   Usage  │
// │ 🤖 Anthropic Main Key   linux-srv1    Server  ● Active  1.2K/d │
// │ 🔷 OpenAI  GPT-4o Proj  linux-srv1  Project  ⚠ Quota  8K/10K │
// │ 🟢 Ollama  Local LLM    linux-srv2   Server  ● Active     —   │
// │ 🔴 Gemini  Failed Key   linux-srv1    User   ✕ Invalid    —   │
// ├──────────────────────────────────────────────────────────────────┤
// │ [← Prev] 1-4 of 4 [Next →]                                      │
// └──────────────────────────────────────────────────────────────────┘

export function ProviderList() {
  const { accounts, isLoading, refresh } = useAIProviders()
  const [filterServer, setFilterServer] = useState<string>('all')
  const [filterScope, setFilterScope] = useState<string>('all')
  const [filterStatus, setFilterStatus] = useState<string>('all')

  const filtered = accounts.filter(a => {
    if (filterServer !== 'all' && a.devServerId !== filterServer) return false
    if (filterScope !== 'all' && a.scope !== filterScope) return false
    if (filterStatus !== 'all' && a.status !== filterStatus) return false
    return true
  })

  return (
    <div className="provider-list">
      <ProviderListHeader onAdd={() => openProviderForm()} />
      <ProviderFilters ... />
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Provider</TableHead>
            <TableHead>Label</TableHead>
            <TableHead>Server</TableHead>
            <TableHead>Scope</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Today's Usage</TableHead>
            <TableHead>Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {filtered.map(account => (
            <ProviderRow key={account.id} account={account} onEdit={openEdit} onTest={testConnection} />
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
```

---

## 2. ProviderForm + CredentialInput

```typescript
// src/renderer/src/components/ai-provider/ProviderForm.tsx

// CRITICAL: API key NEVER stored in React state in plaintext longer than needed for encryption
// Flow: user types key → immediate SubtleCrypto encrypt → store encryptedBlob
//       on save → POST encryptedBlob to server → relay to dev server → server decrypts

export function ProviderForm({ account, onClose }: ProviderFormProps) {
  const [formData, setFormData] = useState({
    provider: account?.provider ?? 'anthropic' as AIProviderType,
    label: account?.label ?? '',
    model: account?.model ?? '',
    baseUrl: account?.baseUrl ?? '',   // Ollama/vLLM
    scope: account?.scope ?? 'server' as AIProviderScope,
    scopeRefId: account?.scopeRefId ?? '',
    devServerId: account?.devServerId ?? '',
    quotaLimitDay: account?.quotaLimitDay ?? 0,
  })
  const [hasNewCredential, setHasNewCredential] = useState(false)
  const [encryptedCredential, setEncryptedCredential] = useState<{
    encryptedBlob: string; iv: string
  } | null>(null)

  const handleSave = async () => {
    // 1. Create or update account metadata
    let accountId = account?.id
    if (!accountId) {
      const created = await rpc.call('aiProvider.create', formData)
      accountId = (created as any).id
    } else {
      await rpc.call('aiProvider.update', { accountId, ...formData })
    }

    // 2. If new credential provided → relay write (server never sees plaintext)
    if (hasNewCredential && encryptedCredential) {
      await rpc.call('aiProvider.writeCredential', {
        accountId,
        encryptedBlob: encryptedCredential.encryptedBlob,
        iv: encryptedCredential.iv,
      })
    }

    onClose()
  }

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{account ? 'Edit' : 'Add'} AI Provider</DialogTitle>
        </DialogHeader>

        <ProviderTypeSelector
          value={formData.provider}
          onChange={p => setFormData(d => ({ ...d, provider: p }))}
        />

        <Input label="Label" value={formData.label} onChange={...} />

        <ScopeSelector
          scope={formData.scope}
          scopeRefId={formData.scopeRefId}
          onChange={...}
        />

        <DevServerSelector
          value={formData.devServerId}
          onChange={...}
        />

        {/* Ollama/vLLM: custom base URL */}
        {['ollama', 'vllm'].includes(formData.provider) && (
          <Input label="Base URL" value={formData.baseUrl} onChange={...} placeholder="http://localhost:11434" />
        )}

        <Input label="Default Model" value={formData.model} onChange={...} />

        <Input
          label="Daily Token Quota (0 = unlimited)"
          type="number"
          value={formData.quotaLimitDay}
          onChange={...}
        />

        {/* Credential: encrypted client-side */}
        <CredentialInput
          provider={formData.provider}
          hasExisting={!!account?.id}
          onEncrypted={(blob, iv) => {
            setEncryptedCredential({ encryptedBlob: blob, iv })
            setHasNewCredential(true)
          }}
          onClear={() => setHasNewCredential(false)}
        />

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSave}>Save</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

---

## 3. CredentialInput — Client-side Encryption

```typescript
// src/renderer/src/components/ai-provider/CredentialInput.tsx

// CRITICAL: API key field uses type="password", not persisted in state after encryption

interface CredentialInputProps {
  provider: AIProviderType
  hasExisting: boolean
  onEncrypted: (encryptedBlob: string, iv: string) => void
  onClear: () => void
}

const CREDENTIAL_LABELS: Record<AIProviderType, string> = {
  anthropic: 'Anthropic API Key (sk-ant-...)',
  openai:    'OpenAI API Key (sk-...)',
  gemini:    'Google API Key (AIza...)',
  azure:     'Azure OpenAI API Key',
  bedrock:   'AWS Access Key + Secret + Region (JSON)',
  ollama:    'No credential needed (base URL only)',
  vllm:      'vLLM API Key (optional)',
}

export function CredentialInput({ provider, hasExisting, onEncrypted, onClear }: CredentialInputProps) {
  const [rawValue, setRawValue] = useState('')
  const [isEncrypting, setIsEncrypting] = useState(false)
  const [isEncrypted, setIsEncrypted] = useState(false)

  if (provider === 'ollama') return null  // No credential needed

  const handleChange = async (value: string) => {
    setRawValue(value)
    setIsEncrypted(false)
    onClear()

    if (value.length > 10) {
      setIsEncrypting(true)
      try {
        // SubtleCrypto encrypt in browser (NEVER send plaintext)
        const iv = crypto.getRandomValues(new Uint8Array(16))
        const keyMaterial = await crypto.subtle.importKey(
          'raw',
          new TextEncoder().encode(value),
          { name: 'AES-GCM' },
          false,
          ['encrypt']
        )
        // Use a session-derived key for transport encryption
        const sessionKey = await deriveSessionKey()
        const encrypted = await crypto.subtle.encrypt(
          { name: 'AES-GCM', iv },
          sessionKey,
          new TextEncoder().encode(value)
        )
        const encryptedBlob = btoa(String.fromCharCode(...new Uint8Array(encrypted)))
        const ivB64 = btoa(String.fromCharCode(...iv))
        setIsEncrypted(true)
        onEncrypted(encryptedBlob, ivB64)
      } finally {
        setIsEncrypting(false)
        // Clear raw value from state after encryption
        setRawValue('') // GC will collect the string
      }
    }
  }

  return (
    <div className="credential-input">
      <Label>{CREDENTIAL_LABELS[provider]}</Label>
      {hasExisting && !isEncrypted && (
        <p className="text-xs text-muted-foreground mb-1">
          Leave blank to keep existing credential
        </p>
      )}
      <div className="relative">
        <Input
          type="password"
          placeholder="Enter API key..."
          value={rawValue}
          onChange={e => handleChange(e.target.value)}
          autoComplete="off"
          autoCorrect="off"
          spellCheck={false}
        />
        {isEncrypting && <Loader2 className="absolute right-2 top-2 animate-spin" size={16} />}
        {isEncrypted && <Lock className="absolute right-2 top-2 text-green-500" size={16} />}
      </div>
      {isEncrypted && (
        <p className="text-xs text-green-600 mt-1">
          ✓ Credential encrypted in browser — will be stored securely on dev server
        </p>
      )}
    </div>
  )
}
```

---

## 4. HealthStatusBadge + UsageChart

```typescript
// src/renderer/src/components/ai-provider/HealthStatusBadge.tsx

const STATUS_CONFIG: Record<AIProviderStatus, { label: string; color: string; icon: ReactNode }> = {
  active:         { label: 'Active',         color: 'text-green-600',  icon: <CheckCircle size={12} /> },
  pending:        { label: 'Pending',         color: 'text-yellow-600', icon: <Clock size={12} /> },
  invalid:        { label: 'Invalid Key',     color: 'text-red-600',    icon: <XCircle size={12} /> },
  quota_exceeded: { label: 'Quota Exceeded',  color: 'text-orange-600', icon: <AlertTriangle size={12} /> },
  unreachable:    { label: 'Unreachable',     color: 'text-gray-600',   icon: <WifiOff size={12} /> },
}

export function HealthStatusBadge({ status }: { status: AIProviderStatus }) {
  const { label, color, icon } = STATUS_CONFIG[status]
  return (
    <span className={cn('flex items-center gap-1 text-xs font-medium', color)}>
      {icon} {label}
    </span>
  )
}

// src/renderer/src/components/ai-provider/UsageChart.tsx
// Simple bar chart: tokens used today vs quota
export function UsageChart({ accountId }: { accountId: string }) {
  const usage = useAppStore(s => s.usageByAccount[accountId])
  const account = useAppStore(s => s.accounts.find(a => a.id === accountId))
  if (!usage || !account) return null

  const quotaLimit = account.quotaLimitDay
  const tokensUsed = usage.tokens
  const pct = quotaLimit > 0 ? Math.min(100, (tokensUsed / quotaLimit) * 100) : 0
  const isWarning = pct >= 80
  const isExceeded = pct >= 100

  return (
    <div className="usage-chart">
      <div className="flex justify-between text-xs mb-1">
        <span>{tokensUsed.toLocaleString()} tokens</span>
        <span className="text-muted-foreground">
          {quotaLimit > 0 ? `/ ${quotaLimit.toLocaleString()}` : 'unlimited'}
        </span>
      </div>
      {quotaLimit > 0 && (
        <Progress
          value={pct}
          className={cn('h-1.5', isExceeded ? 'bg-red-100' : isWarning ? 'bg-yellow-100' : 'bg-gray-100')}
          indicatorClassName={cn(isExceeded ? 'bg-red-500' : isWarning ? 'bg-yellow-500' : 'bg-green-500')}
        />
      )}
    </div>
  )
}
```

---

## 5. useAIProviders Hook

```typescript
// src/renderer/src/hooks/useAIProviders.ts

export function useAIProviders(devServerId?: string) {
  const { accounts, isLoadingAccounts } = useAppStore(s => ({
    accounts: devServerId
      ? s.accounts.filter(a => a.devServerId === devServerId)
      : s.accounts,
    isLoadingAccounts: s.isLoadingAccounts,
  }))

  const refresh = useCallback(async () => {
    const result = await rpc.call('aiProvider.list', { devServerId }) as AIProviderAccount[]
    useAppStore.getState().setAccounts(result)
  }, [devServerId])

  useEffect(() => { refresh() }, [refresh])

  const testConnection = useCallback(async (accountId: string) => {
    const result = await rpc.call('aiProvider.testConnection', { accountId })
      as { ok: boolean; latencyMs: number; error?: string }
    useAppStore.getState().updateAccountStatus(
      accountId,
      result.ok ? 'active' : 'invalid'
    )
    return result
  }, [])

  return { accounts, isLoading: isLoadingAccounts, refresh, testConnection }
}
```

---

## 6. Test Coverage

```
src/renderer/src/components/ai-provider/__tests__/
├── ProviderList.test.tsx
│   ├── renders account rows
│   ├── filters by server, scope, status
│   └── Add Account opens ProviderForm
├── ProviderForm.test.tsx
│   ├── creates account (no existing)
│   ├── updates account (existing)
│   ├── calls aiProvider.writeCredential when credential encrypted
│   └── does NOT call writeCredential when credential unchanged
├── CredentialInput.test.tsx
│   ├── type password → encrypts in browser
│   ├── short value (< 10 chars) → NOT encrypted yet
│   ├── after encryption → raw value cleared from state
│   ├── shows 🔒 icon when encrypted
│   └── ollama provider → input not shown
├── HealthStatusBadge.test.tsx
│   ├── active → green CheckCircle
│   ├── invalid → red XCircle
│   └── quota_exceeded → orange AlertTriangle
└── hooks/__tests__/useAIProviders.test.ts
    ├── fetches accounts on mount
    ├── testConnection: ok → status active
    └── testConnection: fail → status invalid
```

**Target:** ≥ 25 tests; CredentialInput: verify raw value NOT in state after encrypt

---

## Addendum: HLD Cross-References (v5.0 — 2026-07-30)

> **Nguồn:** [HLD C3.11a](../../../docs/hld/v1/C3-components.md), [HLD C4.9](../../../docs/hld/v1/C4-code.md), [security.md §AI Credentials](../../../docs/hld/v1/security.md), [web-server-architecture.md §10.4](../../../docs/hld/web-server-architecture.md)

### Credential Security Flow — CRITICAL (từ HLD security.md + ADR-008)

```
RULE: Orca Backend Server KHÔNG BAO GIỜ thấy plaintext API key

Flow:
    CredentialInput component (browser)
        ↓ user types API key
        ↓ SubtleCrypto.encrypt(sessionKey, apiKey)    ← client-side encrypt
        ↓ rawKey = null   ← immediately clear from memory
        ↓ POST /rpc { method: 'ai-providers.rotateKey',
                      params: { accountId, encryptedKey: '<blob>' } }
        ↓
    Orca Backend Server
        ↓ KHÔNG decrypt — chỉ forward blob
        ↓ relay.call('aiProvider.writeCredential', { accountId, encryptedKey })
        ↓ SSH tunnel → Dev Server
        ↓
    Dev Server Agent
        ↓ decrypt(ORCA_AI_CREDENTIAL_KEY, encryptedKey)  ← server env key
        ↓ write ~/.orca/ai-providers/<accountId>.enc (AES-256-GCM)
        ↓ chmod 0600
```

**Frontend MUST enforce:**
- Input type="password" để tránh browser autofill lưu
- Clear state ngay sau khi encrypt thành công
- KHÔNG log credential value
- Hiển thị last 4 chars của key masked: `sk-...ab12` sau khi set thành công

### AIProviderAccount — Full Type (từ HLD C4.9)

```typescript
interface AIProviderAccount {
  id: string
  devServerId: string
  label: string            // human-friendly name
  provider: 'anthropic' | 'openai' | 'gemini' | 'azure-openai' | 'bedrock' | 'ollama' | 'vllm'
  scope: 'server' | 'project' | 'user'
  scopeId?: string         // projectId or userId (if scope != 'server')
  model: string            // primary model ('claude-opus-4-5', 'gpt-4o', ...)
  endpoint?: string        // custom endpoint (Ollama, vLLM, Azure)
  status: 'healthy' | 'quota_exceeded' | 'invalid' | 'unreachable'
  lastTestedAt?: Date
  createdAt: Date
  usageToday?: number      // tokens today
  usageLimit?: number      // quota limit
}
```

### Provider Priority Cascade (từ HLD C4.9)

```
Khi agent spawn, backend resolve provider theo thứ tự priority:
    1. user-scope account (user chọn provider riêng)
    2. project-scope account (project default)
    3. server-default account (company default)

Frontend hiển thị trong AgentPanel: "Provider: Anthropic (company default)"
```

### ProviderHealthChecker — Background Cron (từ HLD)

```
Backend cron mỗi 15 phút:
    → test connection per provider account
    → UPDATE orca_ai_provider_accounts SET status=...
    → push event: 'ai-provider.status.updated' { accountId, status }

Frontend listens:
    useAIProviders hook: on('ai-provider.status.updated', refreshAccounts)
    HealthStatusBadge: reactive update không cần manual refresh
```

### Providers hỗ trợ (từ HLD)

| Provider | Icon | Endpoint | Auth | Notes |
|---------|------|---------|------|-------|
| Anthropic | 🤖 | api.anthropic.com | API key | Claude models |
| OpenAI | 🔷 | api.openai.com | API key | GPT-4o, o1 |
| Google Gemini | 🟢 | generativelanguage.googleapis.com | API key | Gemini 1.5/2.0 |
| Azure OpenAI | 🔵 | custom endpoint | API key + deployment | Enterprise |
| AWS Bedrock | 🟠 | AWS SDK | IAM credentials | Multi-model |
| Ollama | 🟡 | localhost/custom | none | Local LLM |
| vLLM | 🟣 | custom endpoint | optional key | Custom deploy |
