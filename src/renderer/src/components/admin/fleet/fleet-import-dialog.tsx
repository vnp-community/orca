// src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx
// BUG-FE-FLEET-001 CR-001: Import fleet configuration from orca-fleet.yaml
// Displays file picker, import progress, and per-server results

import { useState, useRef } from 'react'
import { Upload, CheckCircle, XCircle, Loader2, FileText } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { cn } from '@/lib/utils'
import { useAppStore } from '../../../store'
import { useShallow } from 'zustand/react/shallow'

interface FleetImportDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function FleetImportDialog({ open, onOpenChange }: FleetImportDialogProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [isDragging, setIsDragging] = useState(false)

  const { fleetImportStatus, setFleetImportStatus, clearFleetImportStatus } = useAppStore(
    useShallow(s => ({
      fleetImportStatus:    s.fleetImportStatus,
      setFleetImportStatus: s.setFleetImportStatus,
      clearFleetImportStatus: s.clearFleetImportStatus,
    }))
  )

  const phase = fleetImportStatus?.phase ?? 'idle'
  const isDone = phase === 'done' || phase === 'error'
  const isImporting = phase === 'importing' || phase === 'parsing' || phase === 'validating' || phase === 'reading'
  const progress = fleetImportStatus
    ? fleetImportStatus.totalServers > 0
      ? Math.round((fleetImportStatus.importedServers / fleetImportStatus.totalServers) * 100)
      : 0
    : 0

  async function handleFile(file: File) {
    if (!file.name.endsWith('.yaml') && !file.name.endsWith('.yml')) {
      setFleetImportStatus({ phase: 'error', configFilePath: file.name, totalServers: 0, importedServers: 0, skippedServers: 0, failedServers: 0, errors: ['Only .yaml/.yml files are supported'] })
      return
    }

    setFleetImportStatus({ phase: 'reading', configFilePath: file.name, totalServers: 0, importedServers: 0, skippedServers: 0, failedServers: 0, errors: [] })

    try {
      const yamlContent = await file.text()

      setFleetImportStatus({ phase: 'parsing', configFilePath: file.name, totalServers: 0, importedServers: 0, skippedServers: 0, failedServers: 0, errors: [] })

      // Call IPC to import the YAML on main process
      const result = await window.api.ssh.importFleetConfig({ yamlContent, configFilePath: file.name })

      setFleetImportStatus({
        phase: 'done',
        configFilePath: file.name,
        totalServers: result.totalServers,
        importedServers: result.importedServers,
        skippedServers: result.skippedServers,
        failedServers: result.failedServers,
        errors: result.errors ?? [],
      })
    } catch (err: any) {
      setFleetImportStatus({
        phase: 'error',
        configFilePath: file.name,
        totalServers: 0,
        importedServers: 0,
        skippedServers: 0,
        failedServers: 0,
        errors: [err?.message ?? 'Import failed'],
      })
    }
  }

  function handleClose() {
    clearFleetImportStatus()
    onOpenChange(false)
  }

  const phaseLabel: Record<string, string> = {
    idle:       'Ready to import',
    reading:    'Reading file…',
    parsing:    'Parsing YAML…',
    validating: 'Validating config…',
    importing:  'Importing servers…',
    done:       'Import complete',
    error:      'Import failed',
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Upload size={16} />
            Import Fleet Config
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Phase status */}
          <div className="flex items-center gap-2 text-sm">
            {isImporting && <Loader2 size={14} className="animate-spin text-blue-500" />}
            {phase === 'done' && <CheckCircle size={14} className="text-green-500" />}
            {phase === 'error' && <XCircle size={14} className="text-red-500" />}
            {!isImporting && phase !== 'done' && phase !== 'error' && (
              <FileText size={14} className="text-muted-foreground" />
            )}
            <span className={cn(
              'font-medium',
              phase === 'done' && 'text-green-600',
              phase === 'error' && 'text-red-600',
            )}>
              {phaseLabel[phase]}
            </span>
          </div>

          {/* File drop zone */}
          {phase === 'idle' && (
            <div
              className={cn(
                'border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors',
                isDragging ? 'border-blue-500 bg-blue-50 dark:bg-blue-950/20' : 'border-muted-foreground/30 hover:border-muted-foreground/50'
              )}
              onClick={() => fileInputRef.current?.click()}
              onDragOver={e => { e.preventDefault(); setIsDragging(true) }}
              onDragLeave={() => setIsDragging(false)}
              onDrop={e => {
                e.preventDefault()
                setIsDragging(false)
                const file = e.dataTransfer.files[0]
                if (file) handleFile(file)
              }}
            >
              <Upload size={24} className="mx-auto mb-2 text-muted-foreground" />
              <p className="text-sm font-medium">Drop orca-fleet.yaml here</p>
              <p className="text-xs text-muted-foreground mt-1">or click to browse</p>
              <input
                ref={fileInputRef}
                type="file"
                accept=".yaml,.yml"
                className="hidden"
                onChange={e => {
                  const file = e.target.files?.[0]
                  if (file) handleFile(file)
                }}
              />
            </div>
          )}

          {/* Progress bar */}
          {isImporting && fleetImportStatus && (
            <div className="space-y-1">
              <Progress value={progress} className="h-2" />
              <p className="text-xs text-muted-foreground">
                {fleetImportStatus.importedServers} / {fleetImportStatus.totalServers} servers
              </p>
            </div>
          )}

          {/* Done: stats grid */}
          {isDone && fleetImportStatus && (
            <div className="grid grid-cols-3 gap-2">
              <div className="rounded-md border p-2 text-center">
                <p className="text-lg font-bold text-green-600">{fleetImportStatus.importedServers}</p>
                <p className="text-[10px] text-muted-foreground">Added</p>
              </div>
              <div className="rounded-md border p-2 text-center">
                <p className="text-lg font-bold text-yellow-600">{fleetImportStatus.skippedServers}</p>
                <p className="text-[10px] text-muted-foreground">Skipped</p>
              </div>
              <div className="rounded-md border p-2 text-center">
                <p className="text-lg font-bold text-red-600">{fleetImportStatus.failedServers}</p>
                <p className="text-[10px] text-muted-foreground">Failed</p>
              </div>
            </div>
          )}

          {/* Errors list */}
          {isDone && fleetImportStatus && fleetImportStatus.errors.length > 0 && (
            <div className="rounded-md bg-destructive/10 p-2 max-h-20 overflow-y-auto space-y-0.5">
              {fleetImportStatus.errors.map((err, i) => (
                <p key={i} className="text-[10px] text-destructive font-mono">{err}</p>
              ))}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} size="sm">
            {isDone ? 'Close' : 'Cancel'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
