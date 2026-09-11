// src/renderer/src/components/code-review/code-review-panel.tsx
// BL-CR-01~05: Main code review panel — assembly of all code review sub-components
// Layout: [FileTree 256px] [DiffViewer]
//
// TASK-FE-ANNOTATE-003: this component (and everything under
// components/code-review/) has zero real callers anywhere in the app —
// confirmed via GitNexus context({name: "CodeReviewPanel"}) returning
// incoming: {} and a repo-wide grep for JSX usage. It predates the real,
// live diff-comments flow (components/diff-comments/,
// useDiffCommentDecorator.tsx) and was superseded by it without being
// retired. AnnotationPanel (formerly rendered below) was deleted here
// because it called annotation.create/list with a field shape
// (projectId/reviewId) that never matched backend-go's real
// annotation-service contract (repo_id/worktree_id) — see
// SOL-FE-ANNOTATE-001 §3. This file itself is left in place (out of this
// task's scope) but is dead code — a candidate for its own cleanup CR.

import { useState } from 'react'
import { GitPullRequest } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ChangedFilesTree } from './changed-files-tree'
import { CommitMessageGenerator } from './commit-message-generator'
import { useCodeReview } from '../../hooks/use-code-review'

// Lazy-import the DiffViewer that already exists in workspace/git
// (avoids duplicating Monaco setup)
import { DiffViewer } from '../workspace/git/DiffViewer'
import { PrCreateDialog } from './pr-create-dialog'
import { useWorkspace } from '../../context/WorkspaceContext'

type CodeReviewPanelProps = {
  /** null = show all files in working tree; string = scoped to a PR reviewId */
  reviewId?: string
}

export function CodeReviewPanel({ reviewId }: CodeReviewPanelProps) {
  const {
    changedFiles,
    selectedFile,
    setSelectedFile,
    annotationLine,
    isLoadingFiles,
    commitMessage,
    setCommitMessage,
    isCommitting,
    handleCommit
  } = useCodeReview({ reviewId })

  const { project } = useWorkspace()
  const [showPrDialog, setShowPrDialog] = useState(false)

  return (
    <div className="code-review-panel flex h-full overflow-hidden">
      {/* Left sidebar: file tree */}
      <div className="w-64 shrink-0 border-r flex flex-col overflow-hidden">
        <div className="flex items-center justify-between px-3 py-2 border-b">
          <span className="text-xs font-semibold">Changed Files</span>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 gap-1 text-xs"
            onClick={() => setShowPrDialog(true)}
          >
            <GitPullRequest size={12} />
            PR
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto">
          {isLoadingFiles ? (
            <div className="px-3 py-4 text-xs text-muted-foreground">Loading…</div>
          ) : (
            <ChangedFilesTree
              files={changedFiles}
              selectedFile={selectedFile}
              onSelectFile={setSelectedFile}
            />
          )}
        </div>

        {/* Commit message area */}
        <div className="border-t p-3">
          <CommitMessageGenerator
            value={commitMessage}
            onChange={setCommitMessage}
            onCommit={handleCommit}
            isCommitting={isCommitting}
          />
        </div>
      </div>

      {/* Main area: diff + optional annotation */}
      <div className="flex flex-1 overflow-hidden">
        {/* Diff viewer */}
        <div className={`flex-1 overflow-hidden ${annotationLine !== null ? 'border-r' : ''}`}>
          {selectedFile ? (
            <DiffViewer filePath={selectedFile} />
          ) : (
            <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
              Select a file to view diff
            </div>
          )}
        </div>
      </div>

      {/* PR Create Dialog */}
      {project && (
        <PrCreateDialog open={showPrDialog} onOpenChange={setShowPrDialog} projectId={project.id} />
      )}
    </div>
  )
}
