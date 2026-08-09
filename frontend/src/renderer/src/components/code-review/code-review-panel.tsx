// src/renderer/src/components/code-review/code-review-panel.tsx
// BL-CR-01~05: Main code review panel — assembly of all code review sub-components
// Layout: [FileTree 256px] [DiffViewer | AnnotationPanel]

import { useState, useCallback } from 'react'
import { GitPullRequest } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ChangedFilesTree, type ChangedFile } from './changed-files-tree'
import { AnnotationPanel } from './annotation-panel'
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
    handleLineClick,
    closeAnnotation,
    isLoadingFiles,
    commitMessage,
    setCommitMessage,
    isCommitting,
    handleCommit,
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
            <DiffViewer
              filePath={selectedFile}
            />
          ) : (
            <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
              Select a file to view diff
            </div>
          )}
        </div>

        {/* Annotation panel — slides in when line is clicked */}
        {annotationLine !== null && selectedFile && (
          <AnnotationPanel
            filePath={selectedFile}
            lineNumber={annotationLine}
            reviewId={reviewId}
            onClose={closeAnnotation}
          />
        )}
      </div>

      {/* PR Create Dialog */}
      {project && (
        <PrCreateDialog
          open={showPrDialog}
          onOpenChange={setShowPrDialog}
          projectId={project.id}
        />
      )}
    </div>
  )
}
