// src/renderer/src/components/code-review/annotation-panel.tsx
// BL-CR-02: Inline code annotation / comment panel for a specific diff line
// Appears alongside DiffViewer when user clicks on a line number

import { useState, useEffect } from 'react'
import { X, Send, Loader2, MessageSquare } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { toast } from 'sonner'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { useWorkspace } from '../../context/WorkspaceContext'
import { formatDistanceToNow } from 'date-fns'
import { Tracers } from '../../../../shared/trace/tracers'

interface Annotation {
  id: string
  lineNumber: number
  filePath: string
  content: string
  author: string
  authorInitials: string
  createdAt: number
}

interface AnnotationPanelProps {
  filePath: string
  lineNumber: number | null
  reviewId?: string
  onClose: () => void
}

export function AnnotationPanel({
  filePath,
  lineNumber,
  reviewId,
  onClose,
}: AnnotationPanelProps) {
  const [annotations, setAnnotations] = useState<Annotation[]>([])
  const [newComment, setNewComment] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const { project } = useWorkspace()

  // Load existing annotations for this line
  useEffect(() => {
    if (!project || lineNumber === null) return
    setIsLoading(true)

    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<Annotation[]>(target, 'annotation.list', {
      projectId: project.id,
      reviewId,
      filePath,
      lineNumber,
    })
      .then(list => setAnnotations(list ?? []))
      .catch(() => { /* No annotations yet — silent */ })
      .finally(() => setIsLoading(false))
  }, [project, filePath, lineNumber, reviewId])

  const submit = async () => {
    if (!newComment.trim() || !project || lineNumber === null) return
    setIsSaving(true)
    const span = Tracers.codeReviewAnnotateFlow.start({ filePath, lineNumber, reviewId: reviewId ?? '' })
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const created = await callRuntimeRpc<Annotation>(target, 'annotation.create', {
        projectId: project.id,
        reviewId,
        filePath,
        lineNumber,
        content: newComment.trim(),
        traceId: span.id,
      })
      setAnnotations(prev => [...prev, created])
      setNewComment('')
      span.ok({ annotationId: created.id })
    } catch (err) {
      toast.error('Failed to save comment')
      span.fail(err, { filePath, lineNumber })
    } finally {
      setIsSaving(false)
    }
  }

  if (lineNumber === null) return null

  const fileName = filePath.split('/').pop() ?? filePath

  return (
    <div className="annotation-panel flex flex-col border-l bg-background h-full min-w-[280px] max-w-[360px]">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b">
        <div className="flex items-center gap-2 text-xs">
          <MessageSquare size={12} className="text-muted-foreground" />
          <span className="font-medium">Line {lineNumber}</span>
          <span className="text-muted-foreground truncate max-w-[120px]">{fileName}</span>
        </div>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}>
          <X size={12} />
        </Button>
      </div>

      {/* Existing annotations */}
      <ScrollArea className="flex-1 px-3 py-2">
        {isLoading && (
          <div className="flex justify-center py-4">
            <Loader2 size={16} className="animate-spin text-muted-foreground" />
          </div>
        )}
        {!isLoading && annotations.length === 0 && (
          <p className="text-xs text-muted-foreground text-center py-4">No comments yet</p>
        )}
        {annotations.map(a => (
          <div key={a.id} className="flex gap-2 mb-3">
            <Avatar className="h-6 w-6 shrink-0">
              <AvatarFallback className="text-[9px]">{a.authorInitials}</AvatarFallback>
            </Avatar>
            <div>
              <div className="flex items-baseline gap-2">
                <span className="text-xs font-medium">{a.author}</span>
                <span className="text-[10px] text-muted-foreground">
                  {formatDistanceToNow(a.createdAt, { addSuffix: true })}
                </span>
              </div>
              <p className="text-xs mt-0.5 leading-relaxed whitespace-pre-wrap">{a.content}</p>
            </div>
          </div>
        ))}
      </ScrollArea>

      {/* New comment input */}
      <div className="px-3 pb-3 pt-2 border-t space-y-2">
        <Textarea
          value={newComment}
          onChange={e => setNewComment(e.target.value)}
          placeholder="Add a comment..."
          rows={3}
          className="text-sm resize-none"
          onKeyDown={e => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
              submit()
            }
          }}
        />
        <div className="flex justify-end">
          <Button
            size="sm"
            className="h-7 gap-1 text-xs"
            onClick={submit}
            disabled={!newComment.trim() || isSaving}
          >
            {isSaving
              ? <Loader2 size={12} className="animate-spin" />
              : <Send size={12} />
            }
            Comment
          </Button>
        </div>
      </div>
    </div>
  )
}
