# TASK-001-C — Tạo FleetImportDialog + FleetImportProgress Components

**Task ID:** TASK-001-C  
**CR:** CR-001 — Fleet Inventory Config  
**Solution Ref:** SOL-CR-001, Section 4.1 và 4.2  
**Dependencies:** TASK-001-A, TASK-001-B  
**Estimated:** 2–3 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo 2 React components mới:
1. `FleetImportDialog` — dialog chính cho import fleet config
2. `FleetImportProgress` — sub-component hiển thị tiến trình import

---

## Files cần tạo

| File | Loại |
|------|------|
| `src/renderer/src/components/settings/ssh/FleetImportDialog.tsx` | NEW |
| `src/renderer/src/components/settings/ssh/FleetImportProgress.tsx` | NEW |

---

## Bước 1: Khám phá cấu trúc thư mục

```bash
ls src/renderer/src/components/settings/ssh/
ls src/renderer/src/components/ui/
# Kiểm tra shadcn/ui components có sẵn: dialog, button, input, progress
```

### Bước 2: Tạo `FleetImportProgress.tsx`

Tạo file mới với nội dung sau (component đơn giản hơn, tạo trước):

```typescript
// src/renderer/src/components/settings/ssh/FleetImportProgress.tsx
import { Progress } from '@/components/ui/progress'
import { translate } from '@/i18n/i18n'
import { FleetImportStatus } from '@/store/types'

export function FleetImportProgress({ status }: { status: FleetImportStatus }) {
  const progress =
    status.totalServers > 0
      ? Math.round((status.importedServers / status.totalServers) * 100)
      : 0

  return (
    <div className="space-y-3 rounded-md border p-3">
      {/* Progress bar */}
      <div className="space-y-1">
        <div className="flex justify-between text-sm text-muted-foreground">
          <span>{translate('fleet.import.progress', 'Importing servers...')}</span>
          <span>
            {status.importedServers}/{status.totalServers}
          </span>
        </div>
        <Progress value={progress} className="h-2" />
      </div>

      {/* Stats */}
      <div className="flex gap-4 text-sm">
        <span className="text-green-500">
          ✓ {status.importedServers}{' '}
          {translate('fleet.import.imported', 'imported')}
        </span>
        {status.skippedServers > 0 && (
          <span className="text-muted-foreground">
            ↷ {status.skippedServers}{' '}
            {translate('fleet.import.skipped', 'skipped')}
          </span>
        )}
        {status.failedServers > 0 && (
          <span className="text-destructive">
            ✗ {status.failedServers}{' '}
            {translate('fleet.import.failed', 'failed')}
          </span>
        )}
      </div>

      {/* Error list */}
      {status.errors.length > 0 && (
        <div className="space-y-1 rounded border border-destructive/30 bg-destructive/10 p-2">
          {status.errors.map((err, i) => (
            <p key={i} className="text-xs text-destructive">
              {err}
            </p>
          ))}
        </div>
      )}

      {/* Done state */}
      {status.phase === 'done' && (
        <p className="text-sm text-green-500">
          ✅ {translate('fleet.import.done', 'Import complete!')}
        </p>
      )}
    </div>
  )
}
```

### Bước 3: Tạo `FleetImportDialog.tsx`

```typescript
// src/renderer/src/components/settings/ssh/FleetImportDialog.tsx
import { useState } from 'react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { FleetImportProgress } from './FleetImportProgress'

type FleetImportDialogProps = {
  open: boolean
  onClose: () => void
}

export function FleetImportDialog({ open, onClose }: FleetImportDialogProps) {
  const fleetImportStatus = useAppStore((s) => s.fleetImportStatus)
  const clearFleetImportStatus = useAppStore((s) => s.clearFleetImportStatus)
  const [selectedPath, setSelectedPath] = useState<string | null>(null)

  const handlePickFile = async () => {
    try {
      const path = await window.api.ssh.pickFleetConfigFile()
      if (path) setSelectedPath(path)
    } catch (err) {
      toast.error(translate('fleet.import.pickError', 'Could not open file picker'))
    }
  }

  const handleImport = async () => {
    if (!selectedPath) return
    try {
      await window.api.ssh.importFleetConfig(selectedPath)
      // Progress được update qua IPC events trong useIpcEvents()
    } catch (err) {
      toast.error(translate('fleet.import.error', 'Import failed'))
    }
  }

  const handleClose = () => {
    clearFleetImportStatus()
    setSelectedPath(null)
    onClose()
  }

  const isImporting = fleetImportStatus?.phase === 'importing'
  const isDone = fleetImportStatus?.phase === 'done'

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>
            {translate('fleet.import.title', 'Import Fleet Config')}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'fleet.import.description',
              'Import servers from an orca-fleet.yaml file to add them to your SSH hosts.'
            )}
          </DialogDescription>
        </DialogHeader>

        {/* File picker row */}
        <div className="flex gap-2">
          <Input
            readOnly
            value={selectedPath ?? ''}
            placeholder={translate(
              'fleet.import.placeholder',
              'Select orca-fleet.yaml...'
            )}
            className="flex-1"
          />
          <Button variant="outline" onClick={handlePickFile} disabled={isImporting}>
            {translate('fleet.import.browse', 'Browse')}
          </Button>
        </div>

        {/* Import progress */}
        {fleetImportStatus && (
          <FleetImportProgress status={fleetImportStatus} />
        )}

        <DialogFooter>
          <Button variant="ghost" onClick={handleClose} disabled={isImporting}>
            {isDone
              ? translate('common.close', 'Close')
              : translate('common.cancel', 'Cancel')}
          </Button>
          {!isDone && (
            <Button
              onClick={handleImport}
              disabled={!selectedPath || isImporting}
            >
              {isImporting
                ? translate('fleet.import.importing', 'Importing...')
                : translate('fleet.import.import', 'Import')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

### Bước 4: Verify imports

Kiểm tra rằng `@/store` export `useAppStore`, `@/i18n/i18n` export `translate`, và các shadcn components tồn tại:

```bash
grep -r "export.*useAppStore" src/renderer/src/store/
grep -r "export.*translate" src/renderer/src/i18n/
ls src/renderer/src/components/ui/dialog.tsx
ls src/renderer/src/components/ui/progress.tsx
```

### Bước 5: Verify TypeScript

```bash
npx tsc --noEmit 2>&1 | grep -E "FleetImport|fleet-import" | head -20
```

---

## Acceptance Criteria

- [x] `FleetImportDialog.tsx` tạo thành công, không lỗi TypeScript
- [x] `FleetImportProgress.tsx` tạo thành công, không lỗi TypeScript
- [x] Dialog hiển thị file picker input + Browse button
- [x] Progress component hiển thị progress bar, stats, errors
- [x] "Close" button thay "Cancel" khi done
- [x] `clearFleetImportStatus()` gọi khi dialog đóng

---

## Notes cho AI

- Import path convention: `@/` → `src/renderer/src/`
- Dùng `sonner` toast: `import { toast } from 'sonner'`
- `translate(key, fallback)` — key là i18n key, fallback là English text
- Không dùng `useTranslation` hook (dùng sync `translate` function)
- shadcn Dialog: `open` prop controlled từ parent

---

## Implementation Notes

> **Completed:** 2026-07-23 | `FleetImportDialog.tsx`: wizard dialog with file picker, server preview, real-time progress. `ssh/FleetImportProgress.tsx`: [NEW] standalone progress indicator with phase label, progress bar, stats badges, error list. TypeScript: ✅ 0 errors.
