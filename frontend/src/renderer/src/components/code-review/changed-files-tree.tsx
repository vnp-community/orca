// src/renderer/src/components/code-review/changed-files-tree.tsx
// BL-CR-03: Files tree với change counts và directory grouping
// Used in CodeReviewPanel to show all changed files in a PR/diff

import { useState } from 'react'
import { ChevronRight, ChevronDown, FileCode, FilePlus, FileMinus, FileEdit } from 'lucide-react'
import { cn } from '@/lib/utils'

export type ChangeType = 'added' | 'deleted' | 'modified' | 'renamed'

export type ChangedFile = {
  path: string
  changeType: ChangeType
  additions: number
  deletions: number
  /** Only present for renamed files */
  oldPath?: string
}

type ChangedFilesTreeProps = {
  files: ChangedFile[]
  selectedFile: string | null
  onSelectFile: (path: string) => void
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function groupByDirectory(files: ChangedFile[]): Map<string, ChangedFile[]> {
  const groups = new Map<string, ChangedFile[]>()
  for (const file of files) {
    const lastSlash = file.path.lastIndexOf('/')
    const dir = lastSlash >= 0 ? file.path.slice(0, lastSlash) : '.'
    const existing = groups.get(dir) ?? []
    existing.push(file)
    groups.set(dir, existing)
  }
  return groups
}

function dirStats(files: ChangedFile[]): { additions: number; deletions: number } {
  return files.reduce(
    (acc, f) => ({ additions: acc.additions + f.additions, deletions: acc.deletions + f.deletions }),
    { additions: 0, deletions: 0 }
  )
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function ChangeTypeIcon({ type }: { type: ChangeType }) {
  switch (type) {
    case 'added':    return <FilePlus  size={12} className="text-green-500 shrink-0" />
    case 'deleted':  return <FileMinus size={12} className="text-red-500 shrink-0" />
    case 'modified': return <FileEdit  size={12} className="text-blue-500 shrink-0" />
    case 'renamed':  return <FileCode  size={12} className="text-yellow-500 shrink-0" />
  }
}

function ChangeStats({ additions, deletions, compact = false }: {
  additions: number
  deletions: number
  compact?: boolean
}) {
  return (
    <span className={cn('ml-auto shrink-0 font-mono text-[10px]', compact ? 'gap-1' : 'gap-2')}>
      {additions > 0 && (
        <span className="text-green-500">+{additions}</span>
      )}
      {deletions > 0 && (
        <span className="text-red-500 ml-1">-{deletions}</span>
      )}
    </span>
  )
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function ChangedFilesTree({
  files,
  selectedFile,
  onSelectFile,
}: ChangedFilesTreeProps) {
  const [collapsedDirs, setCollapsedDirs] = useState<Set<string>>(new Set())

  const toggleDir = (dir: string) => {
    setCollapsedDirs(prev => {
      const next = new Set(prev)
      if (next.has(dir)) {next.delete(dir)}
      else {next.add(dir)}
      return next
    })
  }

  const groups = groupByDirectory(files)
  const totalAdditions = files.reduce((s, f) => s + f.additions, 0)
  const totalDeletions = files.reduce((s, f) => s + f.deletions, 0)

  if (files.length === 0) {
    return (
      <div className="flex items-center justify-center h-16 text-xs text-muted-foreground">
        No changed files
      </div>
    )
  }

  return (
    <div className="changed-files-tree text-xs select-none">
      {/* Summary header */}
      <div className="px-3 py-2 border-b flex items-center gap-2 text-muted-foreground">
        <span className="font-medium text-foreground">{files.length} files</span>
        <ChangeStats additions={totalAdditions} deletions={totalDeletions} />
      </div>

      {/* Directory groups */}
      <div className="overflow-y-auto">
        {[...groups.entries()].map(([dir, groupFiles]) => {
          const collapsed = collapsedDirs.has(dir)
          const { additions, deletions } = dirStats(groupFiles)
          const isRoot = dir === '.'

          return (
            <div key={dir}>
              {/* Directory row */}
              {!isRoot && (
                <button
                  className="w-full flex items-center gap-1 px-3 py-1 hover:bg-muted/50 text-muted-foreground"
                  onClick={() => toggleDir(dir)}
                >
                  {collapsed
                    ? <ChevronRight size={12} className="shrink-0" />
                    : <ChevronDown  size={12} className="shrink-0" />
                  }
                  <span className="font-medium truncate">{dir}</span>
                  <ChangeStats additions={additions} deletions={deletions} compact />
                </button>
              )}

              {/* File rows */}
              {!collapsed && groupFiles.map(file => {
                const fileName = file.path.split('/').pop() ?? file.path
                const isSelected = file.path === selectedFile
                return (
                  <button
                    key={file.path}
                    className={cn(
                      'w-full flex items-center gap-1.5 py-1 text-left',
                      isRoot ? 'px-3' : 'px-6',
                      isSelected
                        ? 'bg-accent text-accent-foreground'
                        : 'hover:bg-muted/50 text-foreground'
                    )}
                    onClick={() => onSelectFile(file.path)}
                    title={file.path}
                  >
                    <ChangeTypeIcon type={file.changeType} />
                    <span className="truncate flex-1">{fileName}</span>
                    {file.changeType === 'renamed' && file.oldPath && (
                      <span className="text-muted-foreground text-[9px] truncate max-w-[60px]">
                        ← {file.oldPath.split('/').pop()}
                      </span>
                    )}
                    <ChangeStats additions={file.additions} deletions={file.deletions} compact />
                  </button>
                )
              })}
            </div>
          )
        })}
      </div>
    </div>
  )
}
