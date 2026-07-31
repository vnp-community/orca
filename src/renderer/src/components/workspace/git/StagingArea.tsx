import { useGit } from '../../../hooks/useGit'
import { Button } from '../../ui/button'
import { ChevronDown, ChevronRight, Plus, Minus, Eye } from 'lucide-react'
import { useState } from 'react'
import type { GitFileChange } from '../../../store/slices/git-panel'
import { cn } from '../../../utils'

const STATUS_LABELS: Record<string, string> = {
  M: 'Modified', A: 'Added', D: 'Deleted', R: 'Renamed', U: 'Untracked'
}

interface FileRowProps {
  file:       GitFileChange
  actionIcon: ReactNode
  onAction:   (path: string) => void
  onViewDiff: (path: string) => void
}

function FileRow({ file, actionIcon, onAction, onViewDiff }: FileRowProps) {
  return (
    <div
      className="flex items-center gap-1 px-2 py-0.5 text-sm hover:bg-accent/50 rounded-sm group"
      data-testid={`file-row-${file.path}`}
    >
      <span className="text-xs font-mono text-muted-foreground w-4 shrink-0">
        {file.status}
      </span>
      <span className="truncate flex-1">{file.path}</span>
      <div className="hidden group-hover:flex gap-1 shrink-0">
        <Button size="icon" variant="ghost" className="h-5 w-5" onClick={() => onViewDiff(file.path)}>
          <Eye size={10} />
        </Button>
        <Button
          size="icon"
          variant="ghost"
          className="h-5 w-5"
          onClick={() => onAction(file.path)}
          data-testid={`action-btn-${file.path}`}
        >
          {actionIcon}
        </Button>
      </div>
    </div>
  )
}

export function StagingArea({ onViewDiff }: { onViewDiff: (path: string) => void }) {
  const { stagedFiles, unstagedFiles, stageFile, unstageFile, stageAll, unstageAll } = useGit()
  const [stagedOpen,   setStagedOpen]   = useState(true)
  const [unstagedOpen, setUnstagedOpen] = useState(true)

  return (
    <div className="staging-area" data-testid="staging-area">
      {/* Staged */}
      <div className="staged-section">
        <div
          className="flex items-center justify-between px-2 py-1 cursor-pointer hover:bg-accent/30"
          onClick={() => setStagedOpen(v => !v)}
        >
          <div className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
            {stagedOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            Staged ({stagedFiles.length})
          </div>
          {stagedFiles.length > 0 && (
            <Button size="sm" variant="ghost" className="h-5 text-xs" onClick={e => { e.stopPropagation(); unstageAll() }}>
              Unstage All
            </Button>
          )}
        </div>
        {stagedOpen && stagedFiles.map(f => (
          <FileRow
            key={f.path}
            file={f}
            actionIcon={<Minus size={10} />}
            onAction={unstageFile}
            onViewDiff={onViewDiff}
          />
        ))}
      </div>

      {/* Unstaged */}
      <div className="unstaged-section mt-2">
        <div
          className="flex items-center justify-between px-2 py-1 cursor-pointer hover:bg-accent/30"
          onClick={() => setUnstagedOpen(v => !v)}
        >
          <div className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
            {unstagedOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            Unstaged ({unstagedFiles.length})
          </div>
          {unstagedFiles.length > 0 && (
            <Button size="sm" variant="ghost" className="h-5 text-xs" onClick={e => { e.stopPropagation(); stageAll() }}>
              Stage All
            </Button>
          )}
        </div>
        {unstagedOpen && unstagedFiles.map(f => (
          <FileRow
            key={f.path}
            file={f}
            actionIcon={<Plus size={10} />}
            onAction={stageFile}
            onViewDiff={onViewDiff}
          />
        ))}
      </div>
    </div>
  )
}
