// src/renderer/src/components/code-review/commit-message-generator.tsx
// BL-CR-04: AI-assisted commit message textarea with generate button
// Used in CodeReviewPanel staging flow

import { useState } from 'react'
import { Sparkles, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { toast } from 'sonner'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { useWorkspace } from '../../context/WorkspaceContext'
import { Tracers } from '../../../../shared/trace/tracers'

type CommitMessageGeneratorProps = {
  value: string
  onChange: (value: string) => void
  onCommit: (push: boolean) => Promise<void>
  isCommitting: boolean
}

export function CommitMessageGenerator({
  value,
  onChange,
  onCommit,
  isCommitting,
}: CommitMessageGeneratorProps) {
  const [isGenerating, setIsGenerating] = useState(false)
  const { project, worktreePath } = useWorkspace()

  const generateMessage = async () => {
    if (!project) {return}
    setIsGenerating(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // Why: entry: 'code-review-panel' distinguishes this call site from
    // useGit.aiCommitMessage()'s entry: 'commit-form' (TASK-FE-005.2) — both
    // share the same tracer.
    const span = Tracers.codeReviewAiCommitFlow.start({ projectId: project.id, entry: 'code-review-panel' })
    try {
      const message = await callRuntimeRpc<string>(target, 'git.generateCommitMessage', {
        projectId: project.id,
        worktreePath: worktreePath ?? project.rootPath,
        traceId: span.id,
      })
      onChange(message)
      span.ok({ messageChars: message.length })
    } catch (err: any) {
      if (err?.code === 'GIT_NO_STAGED_CHANGES' || err?.message?.includes('no staged')) {
        toast.error('Stage some files first before generating a commit message')
      } else {
        toast.error('Failed to generate commit message')
      }
      span.fail(err, { projectId: project.id })
    } finally {
      setIsGenerating(false)
    }
  }

  const canSubmit = value.trim().length > 0 && !isCommitting && !isGenerating

  return (
    <div className="commit-message-generator space-y-2">
      {/* Label row with AI button */}
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">Commit message</span>
        <Button
          variant="ghost"
          size="sm"
          className="h-6 gap-1 text-xs px-2"
          onClick={generateMessage}
          disabled={isGenerating || isCommitting}
          title="Generate commit message with AI"
        >
          {isGenerating
            ? <Loader2 size={12} className="animate-spin" />
            : <Sparkles size={12} />
          }
          AI
        </Button>
      </div>

      {/* Textarea */}
      <Textarea
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder="feat(scope): describe your changes"
        maxLength={500}
        rows={3}
        className="text-sm resize-none font-mono"
        disabled={isCommitting}
      />

      {/* Character count */}
      <div className="flex items-center justify-between">
        <span className="text-[10px] text-muted-foreground">{value.length}/500</span>

        {/* Action buttons */}
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            onClick={() => onCommit(true)}
            disabled={!canSubmit}
          >
            Commit & Push
          </Button>
          <Button
            size="sm"
            className="h-7 text-xs"
            onClick={() => onCommit(false)}
            disabled={!canSubmit}
          >
            {isCommitting ? <Loader2 size={12} className="animate-spin mr-1" /> : null}
            Commit
          </Button>
        </div>
      </div>
    </div>
  )
}
