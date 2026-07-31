# TASK-V5-08: ProviderList + ProviderForm + UsageChart

**Order:** 8  
**Prerequisite:** TASK-V5-07 (CredentialInput, HealthStatusBadge)  
**Solution Ref:** SOL-FE-V5-03 (section 5, 6, 7)  
**Est. effort:** ~60 min | **Tests:** 15

---

## Mô tả

Implement ProviderList (table + filters), ProviderForm (add/edit dialog), UsageChart. Mount trong Admin SPA `/admin/ai-providers`.

---

## Files Cần Tạo

### 1. `src/renderer/src/components/ai-provider/UsageChart.tsx`

```typescript
import { useAppStore } from '../../store'
import { Progress } from '../ui/progress'
import { cn } from '../../utils'

export function UsageChart({ accountId }: { accountId: string }) {
  const usage   = useAppStore(s => (s as any).usageByAccount?.[accountId])
  const account = useAppStore(s => (s as any).accounts?.find((a: any) => a.id === accountId))
  if (!usage || !account) return <span className="text-xs text-muted-foreground">—</span>

  const quotaLimit  = account.quotaLimitDay
  const tokensUsed  = usage.tokens
  const pct         = quotaLimit > 0 ? Math.min(100, (tokensUsed / quotaLimit) * 100) : 0
  const isWarning   = pct >= 80
  const isExceeded  = pct >= 100

  return (
    <div className="usage-chart min-w-[100px]">
      <div className="flex justify-between text-xs mb-1">
        <span>{tokensUsed.toLocaleString()}</span>
        <span className="text-muted-foreground">
          {quotaLimit > 0 ? `/ ${quotaLimit.toLocaleString()}` : 'unlimited'}
        </span>
      </div>
      {quotaLimit > 0 && (
        <Progress
          value={pct}
          className={cn('h-1.5', isExceeded ? 'bg-red-100' : isWarning ? 'bg-yellow-100' : 'bg-gray-100')}
        />
      )}
    </div>
  )
}
```

### 2. `src/renderer/src/components/ai-provider/ProviderList.tsx`

```typescript
import { useState, lazy, Suspense } from 'react'
import { useAIProviders } from '../../hooks/useAIProviders'
import { HealthStatusBadge } from './HealthStatusBadge'
import { UsageChart } from './UsageChart'
import { Button } from '../ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Plus, RefreshCw, TestTube } from 'lucide-react'
import type { AIProviderAccount, AIProviderScope, AIProviderStatus } from '@shared/ai-provider-types'

const ProviderForm = lazy(() =>
  import('./ProviderForm').then(m => ({ default: m.ProviderForm }))
)

type Filters = {
  devServerId: string
  scope:       string
  status:      string
}

export function ProviderList() {
  const { accounts, isLoading, refresh, testConnection } = useAIProviders()
  const [editingAccount, setEditingAccount] = useState<AIProviderAccount | null | 'new'>('new' as any)
  const [showForm, setShowForm]             = useState(false)
  const [filters, setFilters]               = useState<Filters>({ devServerId: 'all', scope: 'all', status: 'all' })

  const filtered = accounts.filter(a => {
    if (filters.devServerId !== 'all' && a.devServerId !== filters.devServerId) return false
    if (filters.scope       !== 'all' && a.scope       !== filters.scope)       return false
    if (filters.status      !== 'all' && a.status      !== filters.status)      return false
    return true
  })

  return (
    <div className="provider-list p-4 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">AI Provider Accounts</h2>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={refresh} disabled={isLoading}>
            <RefreshCw size={14} className={isLoading ? 'animate-spin' : ''} />
          </Button>
          <Button
            size="sm"
            onClick={() => { setEditingAccount(null); setShowForm(true) }}
            data-testid="add-account-btn"
          >
            <Plus size={14} className="mr-1" /> Add Account
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-3">
        <Select value={filters.scope} onValueChange={v => setFilters(f => ({ ...f, scope: v }))}>
          <SelectTrigger className="w-36" data-testid="filter-scope">
            <SelectValue placeholder="All Scopes" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Scopes</SelectItem>
            <SelectItem value="server">Server</SelectItem>
            <SelectItem value="project">Project</SelectItem>
            <SelectItem value="user">User</SelectItem>
          </SelectContent>
        </Select>
        <Select value={filters.status} onValueChange={v => setFilters(f => ({ ...f, status: v }))}>
          <SelectTrigger className="w-40" data-testid="filter-status">
            <SelectValue placeholder="All Statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Statuses</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="invalid">Invalid</SelectItem>
            <SelectItem value="quota_exceeded">Quota Exceeded</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Table */}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Provider</TableHead>
            <TableHead>Label</TableHead>
            <TableHead>Server</TableHead>
            <TableHead>Scope</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Today's Usage</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {filtered.map(account => (
            <TableRow key={account.id} data-testid={`account-row-${account.id}`}>
              <TableCell className="font-medium capitalize">{account.provider}</TableCell>
              <TableCell>{account.label}</TableCell>
              <TableCell className="text-sm text-muted-foreground">{account.devServerId}</TableCell>
              <TableCell className="text-sm capitalize">{account.scope}</TableCell>
              <TableCell><HealthStatusBadge status={account.status} /></TableCell>
              <TableCell><UsageChart accountId={account.id} /></TableCell>
              <TableCell>
                <div className="flex gap-1">
                  <Button size="sm" variant="ghost" onClick={() => testConnection(account.id)}>
                    <TestTube size={12} />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => { setEditingAccount(account); setShowForm(true) }}
                  >
                    Edit
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
          {filtered.length === 0 && (
            <TableRow>
              <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                No accounts found
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {/* Form dialog */}
      {showForm && (
        <Suspense>
          <ProviderForm
            account={editingAccount === null ? undefined : editingAccount as AIProviderAccount}
            onClose={() => { setShowForm(false); refresh() }}
          />
        </Suspense>
      )}
    </div>
  )
}
```

### 3. `src/renderer/src/components/ai-provider/ProviderForm.tsx`

```typescript
import { useState } from 'react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { CredentialInput } from './CredentialInput'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import type { AIProviderAccount, AIProviderType, AIProviderScope } from '@shared/ai-provider-types'
import toast from 'react-hot-toast'

interface ProviderFormProps {
  account?: AIProviderAccount
  onClose:  () => void
}

export function ProviderForm({ account, onClose }: ProviderFormProps) {
  const [provider,  setProvider]  = useState<AIProviderType>(account?.provider  ?? 'anthropic')
  const [label,     setLabel]     = useState(account?.label     ?? '')
  const [model,     setModel]     = useState(account?.model     ?? '')
  const [baseUrl,   setBaseUrl]   = useState(account?.baseUrl   ?? '')
  const [scope,     setScope]     = useState<AIProviderScope>(account?.scope ?? 'server')
  const [devServer, setDevServer] = useState(account?.devServerId ?? '')
  const [quota,     setQuota]     = useState(account?.quotaLimitDay ?? 0)
  const [isSaving,  setIsSaving]  = useState(false)

  const [encryptedCred, setEncryptedCred] = useState<{ encryptedBlob: string; iv: string } | null>(null)
  const [hasNewCred, setHasNewCred]       = useState(false)

  const handleSave = async () => {
    setIsSaving(true)
    try {
      const payload = { provider, label, model, baseUrl, scope, devServerId: devServer, quotaLimitDay: quota }
      let accountId = account?.id

      if (!accountId) {
        const created = await callRuntimeRpc('aiProvider.create', payload) as AIProviderAccount
        accountId = created.id
      } else {
        await callRuntimeRpc('aiProvider.update', { accountId, ...payload })
      }

      // Write credential if new one provided
      if (hasNewCred && encryptedCred) {
        await callRuntimeRpc('aiProvider.writeCredential', {
          accountId,
          encryptedBlob: encryptedCred.encryptedBlob,
          iv:            encryptedCred.iv,
        })
      }

      toast.success(account ? 'Account updated' : 'Account created')
      onClose()
    } catch (err: any) {
      toast.error(err?.message ?? 'Save failed')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="max-w-md" data-testid="provider-form">
        <DialogHeader>
          <DialogTitle>{account ? 'Edit' : 'Add'} AI Provider</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Provider type */}
          <div>
            <Label>Provider</Label>
            <Select value={provider} onValueChange={v => setProvider(v as AIProviderType)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(['anthropic', 'openai', 'gemini', 'azure', 'bedrock', 'ollama', 'vllm'] as AIProviderType[]).map(p => (
                  <SelectItem key={p} value={p} className="capitalize">{p}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div><Label>Label</Label><Input value={label} onChange={e => setLabel(e.target.value)} /></div>
          <div><Label>Default Model</Label><Input value={model} onChange={e => setModel(e.target.value)} /></div>

          {['ollama', 'vllm'].includes(provider) && (
            <div>
              <Label>Base URL</Label>
              <Input value={baseUrl} placeholder="http://localhost:11434" onChange={e => setBaseUrl(e.target.value)} />
            </div>
          )}

          <div>
            <Label>Scope</Label>
            <Select value={scope} onValueChange={v => setScope(v as AIProviderScope)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="server">Server</SelectItem>
                <SelectItem value="project">Project</SelectItem>
                <SelectItem value="user">User</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div><Label>Daily Quota (0 = unlimited)</Label>
            <Input type="number" value={quota} onChange={e => setQuota(+e.target.value)} />
          </div>

          <CredentialInput
            provider={provider}
            hasExisting={!!account?.id}
            onEncrypted={(blob, iv) => { setEncryptedCred({ encryptedBlob: blob, iv }); setHasNewCred(true) }}
            onClear={() => setHasNewCred(false)}
          />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSave} disabled={isSaving} data-testid="save-provider-btn">
            {isSaving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

---

## Files Cần Sửa (Additive)

### `src/renderer/src/components/admin/AdminApp.tsx`

```typescript
const ProviderList = lazy(() =>
  import('../ai-provider/ProviderList').then(m => ({ default: m.ProviderList }))
)
// Trong <Routes>:
<Route path="/ai-providers" element={<ProviderList />} />
```

---

## Tests — `src/renderer/src/components/ai-provider/__tests__/ProviderList.test.tsx`

```typescript
// @vitest-environment happy-dom — 5 tests:
// renders account rows | filters by scope | filters by status
// "Add Account" opens ProviderForm | test connection button visible
```

## Tests — `src/renderer/src/components/ai-provider/__tests__/ProviderForm.test.tsx`

```typescript
// @vitest-environment happy-dom — 6 tests:
// create account → aiProvider.create | update → aiProvider.update
// calls aiProvider.writeCredential when hasNewCred | does NOT call writeCredential when unchanged
// ollama → base URL field shown | toast.success on save
```

## Tests — `src/renderer/src/components/ai-provider/__tests__/UsageChart.test.tsx`

```typescript
// @vitest-environment happy-dom — 4 tests:
// no usage → shows "—" | shows tokens used / quota
// pct ≥80 → yellow progress | pct=100 → red + Quota Exceeded implied
```

---

## Acceptance Criteria

- [x] `ProviderList` hiển thị tất cả accounts từ store
- [x] Filter scope/status works
- [x] "Add Account" mở `ProviderForm` (lazy loaded)
- [x] `ProviderForm` create: gọi `aiProvider.create`
- [x] `ProviderForm` update: gọi `aiProvider.update`
- [x] `aiProvider.writeCredential` chỉ gọi khi `hasNewCred === true`
- [x] Route `/admin/ai-providers` accessible trong Admin SPA
- [x] 15/15 tests pass (5 ProviderList + 6 ProviderForm + 4 UsageChart)
