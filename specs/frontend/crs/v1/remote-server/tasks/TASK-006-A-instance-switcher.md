# TASK-006-A — Tạo OrcaInstanceSwitcher (Phase 1 RBAC)

**Task ID:** TASK-006-A  
**CR:** CR-006 — Team-based Access Control  
**Solution Ref:** SOL-CR-006, Section 2  
**Dependencies:** Không (độc lập)  
**Estimated:** 2–3 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `OrcaInstanceSwitcher` cho web mode — cho phép developer lưu và chọn nhiều Orca server instances (phase 1 workaround cho RBAC). Dùng localStorage.

---

## Files cần tạo/sửa

| File | Action |
|------|--------|
| `src/renderer/src/web/OrcaInstanceSwitcher.tsx` | CREATE |
| `src/renderer/src/web/AddInstanceForm.tsx` | CREATE |
| `src/renderer/src/hooks/useSavedOrcaInstances.ts` | CREATE |
| `src/renderer/src/web/main.tsx` (hoặc web entry) | MODIFY |

---

## Bước 1: Tạo useSavedOrcaInstances.ts

```typescript
// src/renderer/src/hooks/useSavedOrcaInstances.ts
import { useState, useCallback } from 'react'

export type OrcaInstance = {
  id: string
  label: string       // "Team Backend — vnp-blc"
  url: string         // "https://orca.team.internal"
  team?: string
  lastConnectedAt?: number
}

const STORAGE_KEY = 'orca.saved-instances'

function loadInstances(): OrcaInstance[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

function saveInstances(instances: OrcaInstance[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(instances))
  } catch {
    // quota exceeded or incognito mode
  }
}

export function useSavedOrcaInstances() {
  const [instances, setInstancesState] = useState<OrcaInstance[]>(loadInstances)

  const addInstance = useCallback((instance: OrcaInstance) => {
    setInstancesState((prev) => {
      const next = [...prev, instance]
      saveInstances(next)
      return next
    })
  }, [])

  const removeInstance = useCallback((id: string) => {
    setInstancesState((prev) => {
      const next = prev.filter((i) => i.id !== id)
      saveInstances(next)
      return next
    })
  }, [])

  const updateLastConnected = useCallback((id: string) => {
    setInstancesState((prev) => {
      const next = prev.map((i) =>
        i.id === id ? { ...i, lastConnectedAt: Date.now() } : i
      )
      saveInstances(next)
      return next
    })
  }, [])

  return { instances, addInstance, removeInstance, updateLastConnected }
}
```

## Bước 2: Tạo AddInstanceForm.tsx

```typescript
// src/renderer/src/web/AddInstanceForm.tsx
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { translate } from '@/i18n/i18n'
import type { OrcaInstance } from '@/hooks/useSavedOrcaInstances'

type AddInstanceFormProps = {
  onAdd: (instance: OrcaInstance) => void
  onCancel: () => void
}

export function AddInstanceForm({ onAdd, onCancel }: AddInstanceFormProps) {
  const [label, setLabel] = useState('')
  const [url, setUrl] = useState('https://')
  const [team, setTeam] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!label.trim() || !url.trim()) return
    onAdd({
      id: crypto.randomUUID(),
      label: label.trim(),
      url: url.trim().replace(/\/$/, ''),
      team: team.trim() || undefined,
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      <div className="space-y-1.5">
        <Label htmlFor="instance-label">
          {translate('instanceForm.label', 'Display name')}
        </Label>
        <Input
          id="instance-label"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          placeholder={translate('instanceForm.labelPlaceholder', 'e.g. Team Backend')}
          required
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="instance-url">
          {translate('instanceForm.url', 'Orca server URL')}
        </Label>
        <Input
          id="instance-url"
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://orca.yourteam.internal"
          required
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="instance-team">
          {translate('instanceForm.team', 'Team (optional)')}
        </Label>
        <Input
          id="instance-team"
          value={team}
          onChange={(e) => setTeam(e.target.value)}
          placeholder={translate('instanceForm.teamPlaceholder', 'e.g. backend, frontend')}
        />
      </div>

      <div className="flex gap-2 pt-1">
        <Button type="button" variant="ghost" onClick={onCancel} className="flex-1">
          {translate('common.cancel', 'Cancel')}
        </Button>
        <Button type="submit" className="flex-1" disabled={!label || !url}>
          {translate('instanceForm.add', 'Add Server')}
        </Button>
      </div>
    </form>
  )
}
```

## Bước 3: Tạo OrcaInstanceSwitcher.tsx

```typescript
// src/renderer/src/web/OrcaInstanceSwitcher.tsx
import { useState } from 'react'
import { ServerIcon, PlusIcon, TrashIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import {
  useSavedOrcaInstances,
  type OrcaInstance,
} from '@/hooks/useSavedOrcaInstances'
import { AddInstanceForm } from './AddInstanceForm'

function formatRelativeTime(ts?: number): string {
  if (!ts) return ''
  const diff = Math.round((Date.now() - ts) / 1000)
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`
  if (diff < 86400) return `${Math.round(diff / 3600)}h ago`
  return `${Math.round(diff / 86400)}d ago`
}

export function OrcaInstanceSwitcher({
  onSelect,
}: {
  onSelect: (instance: OrcaInstance) => void
}) {
  const { instances, addInstance, removeInstance, updateLastConnected } =
    useSavedOrcaInstances()
  const [showAddForm, setShowAddForm] = useState(false)

  const handleSelect = (instance: OrcaInstance) => {
    updateLastConnected(instance.id)
    onSelect(instance)
  }

  return (
    <div className="w-[400px] space-y-5">
      {/* Header */}
      <div>
        <h1 className="text-xl font-semibold">
          {translate('instanceSwitcher.title', 'Connect to Orca')}
        </h1>
        <p className="text-sm text-muted-foreground">
          {translate(
            'instanceSwitcher.subtitle',
            'Select your team server or add a new one.'
          )}
        </p>
      </div>

      {/* Instance list */}
      {instances.length > 0 && !showAddForm && (
        <div className="space-y-1.5">
          {instances
            .slice()
            .sort((a, b) => (b.lastConnectedAt ?? 0) - (a.lastConnectedAt ?? 0))
            .map((instance) => (
              <div key={instance.id} className="group relative">
                <button
                  className="flex w-full items-center gap-3 rounded-md border px-4 py-3 text-left transition-colors hover:bg-muted/50"
                  onClick={() => handleSelect(instance)}
                >
                  <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-primary/10">
                    <ServerIcon className="h-4 w-4 text-primary" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium">{instance.label}</p>
                    <p className="text-xs text-muted-foreground truncate">
                      {instance.url}
                    </p>
                  </div>
                  {instance.lastConnectedAt && (
                    <span className="text-xs text-muted-foreground flex-shrink-0">
                      {formatRelativeTime(instance.lastConnectedAt)}
                    </span>
                  )}
                </button>
                {/* Delete button */}
                <button
                  className="absolute right-2 top-1/2 -translate-y-1/2 hidden rounded p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground group-hover:flex"
                  onClick={(e) => {
                    e.stopPropagation()
                    removeInstance(instance.id)
                  }}
                >
                  <TrashIcon className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
        </div>
      )}

      {/* Add form */}
      {showAddForm ? (
        <AddInstanceForm
          onAdd={(instance) => {
            addInstance(instance)
            setShowAddForm(false)
          }}
          onCancel={() => setShowAddForm(false)}
        />
      ) : (
        <Button
          variant="outline"
          className="w-full gap-2"
          onClick={() => setShowAddForm(true)}
        >
          <PlusIcon className="h-4 w-4" />
          {translate('instanceSwitcher.addServer', 'Add Orca server')}
        </Button>
      )}
    </div>
  )
}
```

## Bước 4: Tích hợp vào web entry point

Tìm web mode entry:

```bash
find src/renderer/src/web -name "*.tsx" | head -10
grep -rn "WebConnect\|WebApp" src/renderer/src/web/ | head -10
```

Thêm instance switcher flow:

```typescript
// Trong web app root component:
function WebApp() {
  const { instances } = useSavedOrcaInstances()
  const [targetUrl, setTargetUrl] = useState<string | null>(null)

  // Show instance switcher if >1 saved instances and no target selected
  if (!targetUrl && instances.length > 1) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <OrcaInstanceSwitcher
          onSelect={(instance) => setTargetUrl(instance.url)}
        />
      </div>
    )
  }

  // Existing WebConnect flow (with or without URL)
  return <WebConnect url={targetUrl ?? undefined} />
}
```

---

## Acceptance Criteria

- [x] `OrcaInstanceSwitcher` hiển thị khi web mode và instances > 1
- [x] Click instance → navigate đến WebConnect với URL đó
- [x] Sort by lastConnectedAt (most recent first)
- [x] Hover → hiện delete button (trashcan)
- [x] "Add Orca server" button → form với label, URL, team fields
- [x] Instances persist trong localStorage
- [x] TypeScript compile clean

---

## Implementation Notes

> **Completed:** 2026-07-23 | `hooks/useSavedOrcaInstances.ts`: localStorage CRUD + OrcaInstance type. `web/AddInstanceForm.tsx`: label/url/team form. `web/OrcaInstanceSwitcher.tsx`: sorted by lastConnectedAt desc, hover→delete trashcan, Add form toggle, instances persist. TypeScript: ✅ 0 errors.
