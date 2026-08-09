import { useState, useEffect, useCallback } from 'react'
import { toast } from 'sonner'
import { Upload, FileText, CheckCircle2, XCircle, Loader2 } from 'lucide-react'
import { Button } from '../ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '../ui/dialog'
import { Progress } from '../ui/progress'
import { Badge } from '../ui/badge'
import { ScrollArea } from '../ui/scroll-area'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import type { FleetImportStatus } from '@/store/slices/ssh'

type FleetImportStep = 'pick-file' | 'importing' | 'done' | 'error'

type FleetImportDialogProps = {
  open: boolean
  onClose: () => void
  /** Called after a successful import so SshPane can reload targets */
  onImportComplete?: () => void
}

export function FleetImportDialog({
  open,
  onClose,
  onImportComplete
}: FleetImportDialogProps): React.JSX.Element {
  const [step, setStep] = useState<FleetImportStep>('pick-file')
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [overwrite, setOverwrite] = useState(false)
  const [importResult, setImportResult] = useState<{
    imported: number
    skipped: number
    failed: number
    errors: string[]
  } | null>(null)

  const fleetImportStatus = useAppStore((s) => s.fleetImportStatus)
  const setFleetImportStatus = useAppStore((s) => s.setFleetImportStatus)
  const clearFleetImportStatus = useAppStore((s) => s.clearFleetImportStatus)

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      setStep('pick-file')
      setSelectedFile(null)
      setOverwrite(false)
      setImportResult(null)
      clearFleetImportStatus()
    }
  }, [open, clearFleetImportStatus])

  const handlePickFile = useCallback(async (): Promise<void> => {
    try {
      const filePath = await window.api.ssh.pickFleetConfigFile?.()
      if (filePath) {
        setSelectedFile(filePath)
      }
    } catch {
      toast.error(
        translate(
          'auto.components.settings.FleetImportDialog.pickError',
          'Failed to open file picker'
        )
      )
    }
  }, [])

  const handleImport = useCallback(async (): Promise<void> => {
    if (!selectedFile) {return}

    setStep('importing')
    setFleetImportStatus({ phase: 'reading', totalServers: 0, importedServers: 0, skippedServers: 0, failedServers: 0, errors: [], configFilePath: selectedFile })

    // Subscribe to progress events
    const unsub = window.api.ssh.onFleetImportProgress?.((status: FleetImportStatus) => {
      setFleetImportStatus(status)
    })

    try {
      const result = await window.api.ssh.importFleetConfig?.({
        filePath: selectedFile,
        overwrite
      })
      setImportResult(result ?? { imported: 0, skipped: 0, failed: 0, errors: [] })
      setStep('done')
      onImportComplete?.()
    } catch (err) {
      setStep('error')
      setFleetImportStatus({
        phase: 'error',
        totalServers: 0,
        importedServers: 0,
        skippedServers: 0,
        failedServers: 1,
        errors: [err instanceof Error ? err.message : String(err)],
        configFilePath: selectedFile
      })
    } finally {
      unsub?.()
    }
  }, [selectedFile, overwrite, setFleetImportStatus, onImportComplete])

  const handleClose = useCallback((): void => {
    clearFleetImportStatus()
    onClose()
  }, [clearFleetImportStatus, onClose])

  // Compute progress percentage from store status
  const progressPercent = fleetImportStatus
    ? fleetImportStatus.totalServers > 0
      ? Math.round(
          ((fleetImportStatus.importedServers + fleetImportStatus.skippedServers + fleetImportStatus.failedServers) /
            fleetImportStatus.totalServers) *
            100
        )
      : 20 // Indeterminate while reading/validating
    : 0

  const fileBasename = selectedFile ? selectedFile.split(/[\\/]/).pop() ?? selectedFile : null

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Upload className="size-4" />
            {translate(
              'auto.components.settings.FleetImportDialog.title',
              'Import Fleet Config'
            )}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'auto.components.settings.FleetImportDialog.description',
              'Import SSH targets from a fleet YAML/JSON configuration file.'
            )}
          </DialogDescription>
        </DialogHeader>

        {/* ── Step: Pick File ── */}
        {step === 'pick-file' && (
          <div className="space-y-4">
            {/* File picker area */}
            <button
              className="flex w-full cursor-pointer flex-col items-center gap-2 rounded-lg border-2 border-dashed border-border/60 bg-card/30 px-6 py-8 text-center transition-colors hover:border-border hover:bg-card/60"
              onClick={() => void handlePickFile()}
            >
              <FileText className="size-8 text-muted-foreground" />
              {fileBasename ? (
                <>
                  <p className="text-sm font-medium">{fileBasename}</p>
                  <p className="text-xs text-muted-foreground">{selectedFile}</p>
                </>
              ) : (
                <>
                  <p className="text-sm font-medium">
                    {translate(
                      'auto.components.settings.FleetImportDialog.pickPrompt',
                      'Click to select a fleet config file'
                    )}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {translate(
                      'auto.components.settings.FleetImportDialog.pickHint',
                      'YAML or JSON fleet configuration'
                    )}
                  </p>
                </>
              )}
            </button>

            {/* Overwrite toggle */}
            {selectedFile ? (
              <label className="flex cursor-pointer items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={overwrite}
                  onChange={(e) => setOverwrite(e.target.checked)}
                  className="rounded border-border"
                />
                {translate(
                  'auto.components.settings.FleetImportDialog.overwrite',
                  'Update existing targets if fleetId matches'
                )}
              </label>
            ) : null}

            <DialogFooter>
              <Button variant="ghost" onClick={handleClose}>
                {translate('auto.components.settings.FleetImportDialog.cancel', 'Cancel')}
              </Button>
              <Button
                onClick={() => void handleImport()}
                disabled={!selectedFile}
              >
                {translate(
                  'auto.components.settings.FleetImportDialog.importBtn',
                  'Import'
                )}
              </Button>
            </DialogFooter>
          </div>
        )}

        {/* ── Step: Importing ── */}
        {step === 'importing' && fleetImportStatus && (
          <div className="space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span className="flex items-center gap-1.5 text-muted-foreground">
                  <Loader2 className="size-3.5 animate-spin" />
                  {phaseLabel(fleetImportStatus.phase)}
                </span>
                {fleetImportStatus.totalServers > 0 && (
                  <span className="tabular-nums text-xs text-muted-foreground">
                    {fleetImportStatus.importedServers}/{fleetImportStatus.totalServers}
                  </span>
                )}
              </div>
              <Progress value={progressPercent} className="h-2" />
            </div>
            {fleetImportStatus.errors.length > 0 && (
              <ScrollArea className="max-h-[120px]">
                <div className="space-y-0.5">
                  {fleetImportStatus.errors.map((err, i) => (
                    <p key={i} className="text-xs text-destructive">
                      {err}
                    </p>
                  ))}
                </div>
              </ScrollArea>
            )}
          </div>
        )}

        {/* ── Step: Done ── */}
        {step === 'done' && importResult && (
          <div className="space-y-4">
            <div className="flex flex-col items-center gap-2 py-4 text-center">
              <CheckCircle2 className="size-10 text-green-500" />
              <p className="text-base font-semibold">
                {translate(
                  'auto.components.settings.FleetImportDialog.doneTitle',
                  'Import complete'
                )}
              </p>
            </div>
            <div className="flex justify-center gap-3">
              <div className="flex flex-col items-center gap-1">
                <Badge variant="secondary" className="tabular-nums text-green-600">
                  {importResult.imported}
                </Badge>
                <p className="text-xs text-muted-foreground">
                  {translate(
                    'auto.components.settings.FleetImportDialog.imported',
                    'Imported'
                  )}
                </p>
              </div>
              {importResult.skipped > 0 && (
                <div className="flex flex-col items-center gap-1">
                  <Badge variant="secondary" className="tabular-nums text-yellow-600">
                    {importResult.skipped}
                  </Badge>
                  <p className="text-xs text-muted-foreground">
                    {translate(
                      'auto.components.settings.FleetImportDialog.skipped',
                      'Skipped'
                    )}
                  </p>
                </div>
              )}
              {importResult.failed > 0 && (
                <div className="flex flex-col items-center gap-1">
                  <Badge variant="destructive" className="tabular-nums">
                    {importResult.failed}
                  </Badge>
                  <p className="text-xs text-muted-foreground">
                    {translate(
                      'auto.components.settings.FleetImportDialog.failed',
                      'Failed'
                    )}
                  </p>
                </div>
              )}
            </div>
            {importResult.errors.length > 0 && (
              <ScrollArea className="max-h-[120px] rounded border bg-muted/30 p-2">
                <div className="space-y-0.5">
                  {importResult.errors.map((err, i) => (
                    <p key={i} className="text-xs text-destructive">
                      {err}
                    </p>
                  ))}
                </div>
              </ScrollArea>
            )}
            <DialogFooter>
              <Button onClick={handleClose} className="w-full">
                {translate('auto.components.settings.FleetImportDialog.close', 'Close')}
              </Button>
            </DialogFooter>
          </div>
        )}

        {/* ── Step: Error ── */}
        {step === 'error' && (
          <div className="space-y-4">
            <div className="flex flex-col items-center gap-2 py-4 text-center">
              <XCircle className="size-10 text-destructive" />
              <p className="text-base font-semibold">
                {translate(
                  'auto.components.settings.FleetImportDialog.errorTitle',
                  'Import failed'
                )}
              </p>
              {fleetImportStatus?.errors[0] && (
                <p className="text-sm text-muted-foreground">
                  {fleetImportStatus.errors[0]}
                </p>
              )}
            </div>
            <DialogFooter className="gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  setStep('pick-file')
                  clearFleetImportStatus()
                }}
              >
                {translate('auto.components.settings.FleetImportDialog.retry', 'Try Again')}
              </Button>
              <Button variant="ghost" onClick={handleClose}>
                {translate('auto.components.settings.FleetImportDialog.closeError', 'Close')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function phaseLabel(phase: FleetImportStatus['phase']): string {
  switch (phase) {
    case 'reading':
      return translate(
        'auto.components.settings.FleetImportDialog.phaseReading',
        'Reading config file...'
      )
    case 'validating':
      return translate(
        'auto.components.settings.FleetImportDialog.phaseValidating',
        'Validating entries...'
      )
    case 'importing':
      return translate(
        'auto.components.settings.FleetImportDialog.phaseImporting',
        'Importing servers...'
      )
    default:
      return translate(
        'auto.components.settings.FleetImportDialog.phaseProcessing',
        'Processing...'
      )
  }
}
