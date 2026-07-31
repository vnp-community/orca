# TDD-FE-11: Profile Hierarchy UI

**Document:** TDD-FE-11 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Profile UI — 3-layer profile editor, source badges, Company/Dept management
**Feature:** F33
**ADR:** ADR-007
**HLD Ref:** C3.10, C4.7
**Backend TDD:** TDD-14
**Source files (to create):**
- `src/renderer/src/components/profile/ProfileEditor.tsx`
- `src/renderer/src/components/profile/ProfileSourceBadge.tsx`
- `src/renderer/src/components/profile/ProfileFieldRow.tsx`
- `src/renderer/src/components/profile/CompanyProfileAdmin.tsx`
- `src/renderer/src/components/profile/DeptProfileAdmin.tsx`
- `src/renderer/src/hooks/useProfile.ts`
- `src/renderer/src/store/slices/profile.ts`

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. Mục tiêu

Cho phép user xem và chỉnh sửa profile của mình, đồng thời hiểu rõ giá trị nào đến từ tầng nào (Company/Dept/User). Admin có thể cấu hình Company profile và Department profiles.

---

## 2. ProfileEditor Component

```typescript
// src/renderer/src/components/profile/ProfileEditor.tsx

interface ProfileEditorProps {
  scope: 'user' | 'dept' | 'company'
  scopeId?: string            // deptId or companyId (undefined for user's own)
  readOnly?: boolean
}

// Layout:
// ┌────────────────────────────────────────────────┐
// │ Profile Settings                               │
// │ [User Settings] [Effective Settings]           │
// │                                                │
// │ ▼ Agent Settings                               │
// │   Preferred Model  [claude-opus-4-5 ▼] [Dept] │
// │   Trust Preset     [Standard ▼]        [User]  │
// │   Custom Instructions [textarea]       [User]  │
// │   MCP Servers      [+ Add Server]      [Dept]  │
// │                                                │
// │ ▼ Shell Settings                               │
// │   Default Shell    [/bin/zsh]          [Comp.] │
// │   PATH Additions   [/usr/local/bin]    [Concat]│
// │   Env Vars         [KEY=value ...]     [User]  │
// │                                                │
// │ ▼ Security (🔒 Company Only)                   │
// │   Approved Models  [claude-*, gpt-4o]  [Comp.] │
// │   Disallowed Cmds  [rm -rf /]          [Comp.] │
// │                                                │
// │ [Save Changes]                                 │
// └────────────────────────────────────────────────┘

export function ProfileEditor({ scope, scopeId, readOnly = false }: ProfileEditorProps) {
  const { resolvedProfile, userProfile } = useProfile()
  const [localProfile, setLocalProfile] = useState<OrcaProfile>(
    scope === 'user' ? (userProfile ?? {}) : {}
  )
  const [activeTab, setActiveTab] = useState<'own' | 'resolved'>('own')
  const { saveProfile } = useProfileActions()

  const displayProfile = activeTab === 'resolved' ? resolvedProfile : localProfile

  return (
    <div className="profile-editor">
      <Tabs value={activeTab} onValueChange={setActiveTab as any}>
        <TabsList>
          <TabsTrigger value="own">
            {scope === 'company' ? 'Company' : scope === 'dept' ? 'Department' : 'My'} Settings
          </TabsTrigger>
          {scope === 'user' && (
            <TabsTrigger value="resolved">Effective Settings</TabsTrigger>
          )}
        </TabsList>
      </Tabs>

      <ProfileSection title="Agent" icon={<Bot />}>
        <ProfileFieldRow
          label="Preferred Model"
          source={resolvedProfile?._sources['agent.preferredModel']}
          locked={false}
        >
          <ModelSelector
            value={localProfile.agent?.preferredModel}
            onChange={v => setField('agent.preferredModel', v)}
            disabled={readOnly}
          />
        </ProfileFieldRow>
        <ProfileFieldRow
          label="Trust Preset"
          source={resolvedProfile?._sources['agent.trustPreset']}
        >
          <TrustPresetSelector ... />
        </ProfileFieldRow>
        <McpServerList
          servers={localProfile.mcp?.servers}
          readOnly={readOnly}
          onChange={servers => setField('mcp.servers', servers)}
        />
      </ProfileSection>

      <ProfileSection title="Shell">
        <EnvVarsEditor
          vars={localProfile.shell?.envVars}
          readOnly={readOnly}
          onChange={vars => setField('shell.envVars', vars)}
        />
        <PathAdditionsList
          paths={localProfile.shell?.pathAdditions}
          readOnly={readOnly}
        />
      </ProfileSection>

      <ProfileSection title="Security" locked={scope !== 'company'}>
        <SecurityProfileForm
          profile={displayProfile?.security}
          readOnly={scope !== 'company' || readOnly}
        />
      </ProfileSection>

      {!readOnly && (
        <Button onClick={() => saveProfile(scope, localProfile)}>
          Save Changes
        </Button>
      )}
    </div>
  )
}
```

---

## 3. ProfileSourceBadge Component

```typescript
// src/renderer/src/components/profile/ProfileSourceBadge.tsx

type ProfileSource = 'company' | 'dept' | 'user' | 'concat'

interface ProfileSourceBadgeProps {
  source: ProfileSource
  locked?: boolean
}

// Visual:
// [Company]  → purple badge with building icon
// [Dept]     → blue badge with team icon
// [User]     → green badge with person icon
// [Concat]   → grey badge with merge icon
// [🔒 Company Only] → locked red badge

export function ProfileSourceBadge({ source, locked }: ProfileSourceBadgeProps) {
  const config = {
    company: { label: 'Company', color: 'bg-purple-100 text-purple-700', icon: <Building2 size={10} /> },
    dept:    { label: 'Dept', color: 'bg-blue-100 text-blue-700', icon: <Users size={10} /> },
    user:    { label: 'User', color: 'bg-green-100 text-green-700', icon: <User size={10} /> },
    concat:  { label: 'Concat', color: 'bg-gray-100 text-gray-600', icon: <GitMerge size={10} /> },
  }

  if (locked) return (
    <Badge variant="outline" className="bg-red-50 text-red-600 gap-1">
      <Lock size={10} /> Company Only
    </Badge>
  )

  const { label, color, icon } = config[source]
  return (
    <Badge className={cn('gap-1 text-xs', color)}>
      {icon} {label}
    </Badge>
  )
}
```

---

## 4. useProfile Hook

```typescript
// src/renderer/src/hooks/useProfile.ts

export function useProfile() {
  const { resolvedProfile, userProfile, isLoading } = useAppStore(s => ({
    resolvedProfile: s.resolvedProfile,
    userProfile: s.userProfile,
    isLoading: s.profileIsLoading,
  }))

  // Fetch resolved profile on mount
  useEffect(() => {
    rpc.call('profile.getResolved').then(p => {
      useAppStore.getState().setResolved(p as ResolvedProfile)
    })
  }, [])

  return { resolvedProfile, userProfile, isLoading }
}

export function useProfileActions() {
  const saveProfile = useCallback(async (scope: string, profile: OrcaProfile) => {
    if (scope === 'user') {
      await rpc.call('profile.updateUser', { profile })
      // Refetch resolved (cache invalidated server-side)
      const resolved = await rpc.call('profile.getResolved')
      useAppStore.getState().setResolved(resolved as ResolvedProfile)
      toast.success('Profile saved')
    } else if (scope === 'company') {
      await rpc.call('profile.updateCompany', { profile })
      toast.success('Company profile updated')
    } else if (scope === 'dept') {
      await rpc.call('profile.updateDept', { profile })
      toast.success('Department profile updated')
    }
  }, [])

  return { saveProfile }
}
```

---

## 5. Admin UI: Company & Dept Profile

```typescript
// src/renderer/src/components/profile/CompanyProfileAdmin.tsx
// Page within Admin SPA: /admin/profile

// Layout:
// ┌─────────────────────────────────────────────┐
// │ Company Profile                             │
// │ ─────────────────────────────────────────── │
// │ ▼ Departments                               │
// │   [Engineering] [Marketing] [+ Add Dept]    │
// │                                             │
// │ ▼ Company-wide Settings                     │
// │   <ProfileEditor scope="company" />         │
// │                                             │
// │ ▼ Department Override (Engineering)         │
// │   <ProfileEditor scope="dept" scopeId="..." │
// └─────────────────────────────────────────────┘

export function CompanyProfileAdmin() {
  const [depts, setDepts] = useState<Department[]>([])
  const [activeDeptId, setActiveDeptId] = useState<string | null>(null)

  useEffect(() => {
    rpc.call('profile.listDepts').then(d => setDepts(d as Department[]))
  }, [])

  return (
    <div className="company-profile-admin">
      <DeptSelector depts={depts} activeDeptId={activeDeptId} onChange={setActiveDeptId} />
      <ProfileEditor scope="company" />
      {activeDeptId && (
        <ProfileEditor scope="dept" scopeId={activeDeptId} />
      )}
    </div>
  )
}
```

---

## 6. ModelSelector Component

```typescript
// Sub-component: picks AI model from approved list
// Shows: provider icon + model name + context window

interface ModelSelectorProps {
  value?: string
  onChange: (model: string) => void
  disabled?: boolean
}

// Approved models come from company.security.approvedModels
// If null/empty → show all known models
export function ModelSelector({ value, onChange, disabled }: ModelSelectorProps) {
  const approvedModels = useAppStore(s => s.resolvedProfile?.security?.approvedModels ?? [])

  const KNOWN_MODELS = [
    { id: 'claude-opus-4-5', provider: 'anthropic', label: 'Claude Opus 4.5', context: '200K' },
    { id: 'gpt-4o', provider: 'openai', label: 'GPT-4o', context: '128K' },
    { id: 'gemini-2.5-pro', provider: 'google', label: 'Gemini 2.5 Pro', context: '1M' },
    // ...
  ]

  const available = approvedModels.length > 0
    ? KNOWN_MODELS.filter(m => approvedModels.some(ap => m.id.startsWith(ap.replace('*', ''))))
    : KNOWN_MODELS

  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger>
        <SelectValue placeholder="Select model..." />
      </SelectTrigger>
      <SelectContent>
        {available.map(m => (
          <SelectItem key={m.id} value={m.id}>
            <ProviderIcon provider={m.provider} size={14} />
            <span>{m.label}</span>
            <span className="text-xs text-muted-foreground ml-auto">{m.context}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
```

---

## 7. Test Coverage

```
src/renderer/src/components/profile/__tests__/
├── ProfileEditor.test.tsx
│   ├── renders 'My Settings' tab for user scope
│   ├── renders 'Effective Settings' tab showing resolved profile
│   ├── security section shows 🔒 when scope is not 'company'
│   ├── locked field cannot be edited by user scope
│   └── saveProfile called on Save Changes click
├── ProfileSourceBadge.test.tsx
│   ├── company → purple badge
│   ├── dept → blue badge
│   ├── locked=true → 🔒 Company Only
│   └── concat → grey badge
├── ModelSelector.test.tsx
│   ├── shows all models when approvedModels is empty
│   ├── filters to approvedModels when set
│   └── calls onChange with selected model
└── hooks/__tests__/useProfile.test.ts
    ├── fetches resolvedProfile on mount
    └── saveProfile calls correct RPC method per scope
```

**Target:** ≥ 25 tests

---

## Addendum: HLD Cross-References (v5.0 — 2026-07-30)

> **Nguồn:** [HLD C3.10](../../../docs/hld/v1/C3-components.md), [HLD C4.7](../../../docs/hld/v1/C4-code.md), [web-server-architecture.md §10.3](../../../docs/hld/web-server-architecture.md)

### Profile Resolution Flow (Backend → Frontend)

```
profile.getEffective(userId)
    │
    ├── Backend ProfileResolver:
    │     1. ProfileCache.get(userId) → HIT? return (TTL 60s)
    │     2. MISS: DB query 3 layers:
    │           orca_users.profile_json        (user personal)
    │           orca_departments.profile_json  (team defaults)
    │           orca_company.profile_json      (company policy)
    │     3. deepMergeProfiles(company ← dept ← user)
    │           Arrays: pathAdditions → concatenate all tiers
    │           Maps: user overrides dept overrides company
    │           LOCKED: security section → always company value
    │     4. Validate: preferredModel ∈ approvedModels
    │
    └── Frontend receives: ResolvedProfile { agent, editor, shell,
                           integrations, fleet, security, _sources }
        → useProfileStore.setResolved(profile)
        → AgentPanel.providerDisplay reads resolvedProfile.agent.preferredModel
```

### OrcaProfile Structure (từ HLD C4.7)

```typescript
interface OrcaProfile {
  agent?: {
    preferredModel?: string       // 'claude-opus-4-5' | 'gpt-4o' | ...
    trustPreset?: 'strict' | 'standard' | 'permissive'
    maxTokens?: number
    customInstructions?: string
    approvedModels?: string[]     // Company whitelist — LOCKED at company tier
  }
  editor?: {
    theme?: string
    fontSize?: number
    fontFamily?: string
    keybindings?: 'default' | 'vim' | 'emacs'
  }
  shell?: {
    defaultShell?: string         // '/bin/zsh'
    pathAdditions?: string[]      // CONCATENATED across tiers
    envVars?: Record<string, string>
    startupCommands?: string[]
  }
  integrations?: {
    githubOrg?: string
    linearWorkspace?: string
    prTemplate?: string
  }
  fleet?: {
    allowedServerTags?: string[]
    defaultConnectionType?: 'relay-ssh' | 'relay-websocket' | 'direct-websocket'
  }
  security?: {               // 🔒 Company ONLY — always locked
    require2FA?: boolean
    sessionTimeoutHours?: number
    allowedIpRanges?: string[]
  }
}
```

### ProfileSourceBadge — Source Values

| Source value | Badge display | Color | Mô tả |
|-------------|--------------|-------|-------|
| `'company'` | Company | Red | Giá trị từ company config |
| `'dept'` | Dept | Blue | Giá trị từ department config |
| `'user'` | User | Green | Giá trị do user tự chọn |
| `'concat'` | Concat | Grey | Array được merge từ nhiều tiers |
| `'locked'` | 🔒 Locked | Red | Chỉ Company admin mới thay đổi được |

### Cache Invalidation Logic (Frontend)

```typescript
// Khi nào invalidate profile cache (frontend phải gọi lại profile.getEffective):
// 1. User update profile thành công → invalidate('user')
// 2. Admin update company → broadcast 'profile.company.updated' → invalidate('all')
// 3. Lead update dept → broadcast 'profile.dept.updated' → invalidate('dept-members')
// 4. TTL = 60s (cache backend, không cache riêng ở frontend)

// Frontend: không cache profile, luôn lấy từ store.resolvedProfile
// Store được set 1 lần khi bootstrap + re-fetch sau mỗi save
```

### RPC Methods (chi tiết từ HLD)

```typescript
profile.getEffective()         // → ResolvedProfile (includes _sources map)
profile.updateUser(fields)     // Personal settings only — no security section
profile.getDepartment(deptId)  // Lead/admin
profile.updateDepartment(deptId, fields)  // Lead/admin — no security section
profile.getCompany()           // Admin only
profile.updateCompany(fields)  // Admin only — ALL sections including security
profile.listDepartments()      // → Department[] (id, name, leadId, memberCount)
profile.createDepartment(name) // Admin only
```
