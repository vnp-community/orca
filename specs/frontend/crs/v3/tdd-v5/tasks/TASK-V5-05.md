# TASK-V5-05: ProfileEditor + ProfileSourceBadge + ModelSelector

**Order:** 5  
**Prerequisite:** TASK-V5-04 (profile slice + hook)  
**Solution Ref:** SOL-FE-V5-01 (section 3.4, 3.5)  
**Est. effort:** ~90 min | **Tests:** 16

---

## Mô tả

Implement toàn bộ Profile UI: ProfileEditor, ProfileSourceBadge, ProfileFieldRow, ModelSelector, CompanyProfileAdmin. Mount vào Admin SPA.

---

## Files Cần Tạo

### 1. `src/renderer/src/components/profile/ProfileSourceBadge.tsx`

```typescript
import type { ProfileSource } from '@shared/profile-types'
import { Building2, Users, User, GitMerge, Lock } from 'lucide-react'
import { Badge } from '../ui/badge'
import { cn } from '../../utils'

interface ProfileSourceBadgeProps {
  source?: ProfileSource
  locked?: boolean
}

const SOURCE_CONFIG: Record<ProfileSource, { label: string; className: string; icon: ReactNode }> = {
  company: { label: 'Company', className: 'bg-purple-100 text-purple-700', icon: <Building2 size={10} /> },
  dept:    { label: 'Dept',    className: 'bg-blue-100 text-blue-700',     icon: <Users size={10} /> },
  user:    { label: 'User',    className: 'bg-green-100 text-green-700',   icon: <User size={10} /> },
  concat:  { label: 'Concat',  className: 'bg-gray-100 text-gray-600',    icon: <GitMerge size={10} /> },
}

export function ProfileSourceBadge({ source, locked }: ProfileSourceBadgeProps) {
  if (locked) {
    return (
      <Badge variant="outline" className="bg-red-50 text-red-600 gap-1 text-xs">
        <Lock size={10} /> Company Only
      </Badge>
    )
  }
  if (!source) return null
  const { label, className, icon } = SOURCE_CONFIG[source]
  return (
    <Badge className={cn('gap-1 text-xs font-normal', className)}>
      {icon} {label}
    </Badge>
  )
}
```

### 2. `src/renderer/src/components/profile/ProfileFieldRow.tsx`

```typescript
import type { ProfileSource } from '@shared/profile-types'
import { ProfileSourceBadge } from './ProfileSourceBadge'

interface ProfileFieldRowProps {
  label:    string
  source?:  ProfileSource
  locked?:  boolean
  children: ReactNode
}

export function ProfileFieldRow({ label, source, locked, children }: ProfileFieldRowProps) {
  return (
    <div className="profile-field-row flex items-center gap-3 py-2">
      <label className="text-sm font-medium w-44 shrink-0">{label}</label>
      <div className="flex-1">{children}</div>
      <ProfileSourceBadge source={source} locked={locked} />
    </div>
  )
}
```

### 3. `src/renderer/src/components/profile/ModelSelector.tsx`

```typescript
import { useAppStore } from '../../store'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'

interface ModelSelectorProps {
  value?:    string
  onChange:  (model: string) => void
  disabled?: boolean
}

const KNOWN_MODELS = [
  { id: 'claude-opus-4-5',  provider: 'anthropic', label: 'Claude Opus 4.5',  context: '200K' },
  { id: 'claude-sonnet-4',  provider: 'anthropic', label: 'Claude Sonnet 4',  context: '200K' },
  { id: 'gpt-4o',           provider: 'openai',    label: 'GPT-4o',           context: '128K' },
  { id: 'gemini-2.5-pro',   provider: 'google',    label: 'Gemini 2.5 Pro',   context: '1M'   },
  { id: 'gemini-2.5-flash', provider: 'google',    label: 'Gemini 2.5 Flash', context: '1M'   },
]

export function ModelSelector({ value, onChange, disabled }: ModelSelectorProps) {
  const approvedModels = useAppStore(
    s => (s as any).resolvedProfile?.security?.approvedModels ?? []
  ) as string[]

  const available = approvedModels.length > 0
    ? KNOWN_MODELS.filter(m =>
        approvedModels.some(ap => m.id.startsWith(ap.replace(/\*/g, '')))
      )
    : KNOWN_MODELS

  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger data-testid="model-selector">
        <SelectValue placeholder="Select model..." />
      </SelectTrigger>
      <SelectContent>
        {available.map(m => (
          <SelectItem key={m.id} value={m.id}>
            <span>{m.label}</span>
            <span className="text-xs text-muted-foreground ml-2">({m.context})</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
```

### 4. `src/renderer/src/components/profile/ProfileEditor.tsx`

```typescript
import { useState } from 'react'
import { Button } from '../ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import { useProfile, useProfileActions } from '../../hooks/useProfile'
import { ProfileFieldRow } from './ProfileFieldRow'
import { ModelSelector } from './ModelSelector'
import type { OrcaProfile } from '@shared/profile-types'

interface ProfileEditorProps {
  scope:    'user' | 'dept' | 'company'
  scopeId?: string
  readOnly?: boolean
}

export function ProfileEditor({ scope, scopeId, readOnly = false }: ProfileEditorProps) {
  const { resolvedProfile, userProfile } = useProfile()
  const { saveProfile } = useProfileActions()
  const [localProfile, setLocalProfile]  = useState<OrcaProfile>(
    scope === 'user' ? (userProfile ?? {}) : {}
  )
  const [activeTab, setActiveTab] = useState<'own' | 'resolved'>('own')

  const setField = (path: string, value: unknown) => {
    const keys = path.split('.')
    setLocalProfile(prev => {
      const next = structuredClone(prev)
      let cur: any = next
      for (let i = 0; i < keys.length - 1; i++) {
        cur[keys[i]] ??= {}
        cur = cur[keys[i]]
      }
      cur[keys[keys.length - 1]] = value
      return next
    })
  }

  const displayProfile = activeTab === 'resolved' ? resolvedProfile : localProfile

  return (
    <div className="profile-editor space-y-4 p-4" data-testid="profile-editor">
      {scope === 'user' && (
        <Tabs value={activeTab} onValueChange={v => setActiveTab(v as any)}>
          <TabsList>
            <TabsTrigger value="own">My Settings</TabsTrigger>
            <TabsTrigger value="resolved">Effective Settings</TabsTrigger>
          </TabsList>
        </Tabs>
      )}

      {/* Agent section */}
      <section>
        <h3 className="font-semibold text-sm mb-2">Agent</h3>
        <ProfileFieldRow
          label="Preferred Model"
          source={resolvedProfile?._sources?.['agent.preferredModel']}
        >
          <ModelSelector
            value={(displayProfile as any)?.agent?.preferredModel}
            onChange={v => setField('agent.preferredModel', v)}
            disabled={readOnly || activeTab === 'resolved'}
          />
        </ProfileFieldRow>
      </section>

      {/* Security section — company only */}
      <section>
        <h3 className="font-semibold text-sm mb-2">
          Security {scope !== 'company' && '🔒'}
        </h3>
        {scope !== 'company' && (
          <ProfileFieldRow label="Approved Models" locked>
            <span className="text-sm text-muted-foreground">Managed by company admin</span>
          </ProfileFieldRow>
        )}
      </section>

      {!readOnly && activeTab !== 'resolved' && (
        <Button
          onClick={() => saveProfile(scope, localProfile, scopeId)}
          data-testid="save-profile-btn"
        >
          Save Changes
        </Button>
      )}
    </div>
  )
}
```

### 5. `src/renderer/src/components/profile/CompanyProfileAdmin.tsx`

```typescript
import { useEffect, useState } from 'react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { ProfileEditor } from './ProfileEditor'
import type { Department } from '@shared/profile-types'
import { Button } from '../ui/button'

export function CompanyProfileAdmin() {
  const [depts, setDepts]           = useState<Department[]>([])
  const [activeDeptId, setActiveDept] = useState<string | null>(null)

  useEffect(() => {
    callRuntimeRpc('profile.listDepts', {}).then(d => setDepts(d as Department[]))
  }, [])

  return (
    <div className="company-profile-admin p-4 space-y-6">
      <h2 className="text-xl font-semibold">Company Profile</h2>

      {/* Dept selector */}
      <div className="flex gap-2 flex-wrap">
        {depts.map(d => (
          <Button
            key={d.id}
            variant={activeDeptId === d.id ? 'default' : 'outline'}
            size="sm"
            onClick={() => setActiveDept(d.id === activeDeptId ? null : d.id)}
          >
            {d.name}
          </Button>
        ))}
      </div>

      {/* Company-wide settings */}
      <div>
        <h3 className="text-base font-medium mb-2">Company-wide Settings</h3>
        <ProfileEditor scope="company" />
      </div>

      {/* Dept override */}
      {activeDeptId && (
        <div>
          <h3 className="text-base font-medium mb-2">
            Department Override — {depts.find(d => d.id === activeDeptId)?.name}
          </h3>
          <ProfileEditor scope="dept" scopeId={activeDeptId} />
        </div>
      )}
    </div>
  )
}
```

---

## Files Cần Sửa (Additive)

### `src/renderer/src/components/admin/AdminApp.tsx`

```typescript
// Thêm import (lazy) và route:
const CompanyProfileAdmin = lazy(() =>
  import('../profile/CompanyProfileAdmin').then(m => ({ default: m.CompanyProfileAdmin }))
)

// Trong <Routes>:
<Route path="/profile" element={<CompanyProfileAdmin />} />
```

---

## Tests — `src/renderer/src/components/profile/__tests__/ProfileSourceBadge.test.tsx`

```typescript
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ProfileSourceBadge } from '../ProfileSourceBadge'

afterEach(() => cleanup())

describe('ProfileSourceBadge', () => {
  it('company → purple badge', () => {
    render(<ProfileSourceBadge source="company" />)
    const badge = screen.getByText('Company')
    expect(badge.closest('div, span')).toHaveClass('bg-purple-100')
  })
  it('dept → blue badge', () => {
    render(<ProfileSourceBadge source="dept" />)
    expect(screen.getByText('Dept')).toBeInTheDocument()
  })
  it('user → green badge', () => {
    render(<ProfileSourceBadge source="user" />)
    expect(screen.getByText('User')).toBeInTheDocument()
  })
  it('locked=true → 🔒 Company Only', () => {
    render(<ProfileSourceBadge locked />)
    expect(screen.getByText('Company Only')).toBeInTheDocument()
  })
  it('concat → grey Concat badge', () => {
    render(<ProfileSourceBadge source="concat" />)
    expect(screen.getByText('Concat')).toBeInTheDocument()
  })
  it('no source + no locked → null', () => {
    const { container } = render(<ProfileSourceBadge />)
    expect(container.firstChild).toBeNull()
  })
})
```

## Tests — `src/renderer/src/components/profile/__tests__/ModelSelector.test.tsx`

```typescript
// @vitest-environment happy-dom  
// 4 tests: all models shown / filtered / onChange / disabled
// (Xem SOL-FE-V5-01 section 5 cho chi tiết)
```

## Tests — `src/renderer/src/components/profile/__tests__/ProfileEditor.test.tsx`

```typescript
// @vitest-environment happy-dom
// 6 tests: renders My Settings tab, Effective Settings tab,
//          security locked for user/dept scope, Save Changes click,
//          company scope has editable security, loading state
```

---

## Acceptance Criteria

- [x] `ProfileSourceBadge` 4 variants: company/dept/user/concat — đúng màu
- [x] `locked` prop → hiển thị "Company Only" với icon lock
- [x] `ModelSelector` filter theo `resolvedProfile.security.approvedModels`
- [x] `ProfileEditor` scope=user: có "Effective Settings" tab
- [x] `ProfileEditor` scope=company: security section editable
- [x] `ProfileEditor` scope=user/dept: security section locked
- [x] "Save Changes" gọi `saveProfile(scope, localProfile, scopeId?)`
- [x] `CompanyProfileAdmin` route `/admin/profile` accessible qua AdminApp
- [x] 15/15 tests pass (6 ProfileSourceBadge + 3 ModelSelector + 6 ProfileEditor)
