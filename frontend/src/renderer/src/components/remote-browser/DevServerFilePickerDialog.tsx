// Web/server-mode replacement for the desktop-only shell.pick* native OS
// dialogs (desktop/src/main/ipc/shell.ts's pickDirectory/pickAttachment/
// pickImage/pickRepoIconImage/pickAudio) — there is no OS-native file dialog
// reachable from a headless server or a remote browser, so this browses the
// connected Dev Server's filesystem instead, over the same devServer.browseDir
// relay RemoteFileBrowser.tsx already uses (see useDevServerFilePicker.ts).
//
// Renders as its own <Dialog>, so callers can drop it in next to an existing
// dialog/form and just toggle `open` — no restructuring of the caller's own
// dialog required.
import { ArrowUp, Folder, Home, LoaderCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { getFileTypeIcon } from '@/lib/file-type-icons'
import { useActiveDevServer } from '../../store/slices/dev-servers-selectors'
import { useDevServerFilePicker, type DevServerFilePickerMode } from './useDevServerFilePicker'

export type DevServerFilePickerDialogProps = {
  open: boolean
  mode: DevServerFilePickerMode
  title: string
  description?: string
  /** Lowercase, no leading dot (e.g. ['png', 'jpg']). Ignored in 'directory' mode. */
  extensions?: string[]
  initialPath?: string
  onSelect: (path: string) => void
  onClose: () => void
}

export function DevServerFilePickerDialog({
  open,
  mode,
  title,
  description,
  extensions,
  initialPath,
  onSelect,
  onClose
}: DevServerFilePickerDialogProps): React.JSX.Element {
  const devServer = useActiveDevServer()
  const devServerId = devServer?.id ?? null
  const { currentPath, entries, loading, error, navigate, navigateUp, joinCurrent } =
    useDevServerFilePicker({ devServerId, open, initialPath, mode, extensions })

  const handleEntryClick = (entry: { name: string; isDirectory: boolean }): void => {
    if (entry.isDirectory) {
      navigate(joinCurrent(entry.name))
      return
    }
    onSelect(joinCurrent(entry.name))
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-sm">{title}</DialogTitle>
          {description && <DialogDescription className="text-xs">{description}</DialogDescription>}
        </DialogHeader>

        {!devServerId ? (
          <p className="text-xs text-muted-foreground py-6 text-center">
            Connect a Dev Server to browse its files.
          </p>
        ) : (
          <div className="flex flex-col gap-2 min-w-0">
            <div className="flex items-center gap-0.5 min-h-[28px] overflow-x-auto scrollbar-none">
              <button
                type="button"
                onClick={navigateUp}
                disabled={loading || currentPath === '/'}
                className="shrink-0 p-1 rounded hover:bg-accent disabled:opacity-30 transition-colors cursor-pointer disabled:cursor-default"
                aria-label="Navigate up"
              >
                <ArrowUp className="size-3.5" />
              </button>
              <button
                type="button"
                onClick={() => navigate('~')}
                disabled={loading}
                className="shrink-0 p-1 rounded hover:bg-accent transition-colors cursor-pointer"
                aria-label="Go to home directory"
              >
                <Home className="size-3.5" />
              </button>
              <code className="ml-1 text-[11px] text-muted-foreground truncate">
                {currentPath || '…'}
              </code>
            </div>

            <div className="border border-border rounded-md overflow-hidden bg-background">
              <div className="h-[240px] overflow-y-auto scrollbar-sleek">
                {loading ? (
                  <div className="flex items-center justify-center h-full">
                    <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
                  </div>
                ) : error ? (
                  <div className="flex items-center justify-center h-full px-4">
                    <p className="text-xs text-destructive text-center">{error}</p>
                  </div>
                ) : entries.length === 0 ? (
                  <div className="flex items-center justify-center h-full">
                    <p className="text-xs text-muted-foreground">Empty directory</p>
                  </div>
                ) : (
                  entries.map((entry) => {
                    const FileIcon = entry.isDirectory ? Folder : getFileTypeIcon(entry.name)
                    return (
                      <button
                        key={entry.name}
                        type="button"
                        onClick={() => handleEntryClick(entry)}
                        className={cn(
                          'w-full flex items-center gap-2 px-3 py-1.5 text-xs text-left transition-colors cursor-pointer',
                          'hover:bg-accent/60'
                        )}
                      >
                        <FileIcon className="size-3.5 text-muted-foreground shrink-0" />
                        <span className="truncate flex-1 min-w-0">{entry.name}</span>
                      </button>
                    )
                  })
                )}
              </div>
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" size="sm" className="h-7 text-xs" onClick={onClose}>
            Cancel
          </Button>
          {mode === 'directory' && (
            <Button
              size="sm"
              className="h-7 text-xs"
              disabled={!devServerId || loading || !currentPath}
              onClick={() => onSelect(currentPath)}
            >
              <Folder className="size-3.5" />
              Select folder
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
