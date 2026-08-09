import { useState } from 'react'
import { useGit } from '../../../hooks/useGit'
import { Button } from '../../ui/button'
import { Textarea } from '../../ui/textarea'
import { Bot, Loader2 } from 'lucide-react'

export function CommitForm() {
  const { commit, push, aiCommitMessage, stagedFiles, isPushing, isCommitting } = useGit()
  const [message, setMessage]   = useState('')
  const [aiLoading, setAiLoad]  = useState(false)

  const canCommit = message.trim().length > 0 && stagedFiles.length > 0

  const handleCommit = async () => {
    if (!canCommit) {return}
    await commit(message.trim())
    setMessage('')
  }

  const handleCommitAndPush = async () => {
    if (!canCommit) {return}
    await commit(message.trim())
    setMessage('')
    await push('HEAD')
  }

  const fillAIMessage = async () => {
    setAiLoad(true)
    try {
      const msg = await aiCommitMessage()
      setMessage(msg)
    } finally {
      setAiLoad(false)
    }
  }

  return (
    <div className="commit-form p-2 space-y-2 border-t" data-testid="commit-form">
      <div className="flex items-center gap-1">
        <Textarea
          value={message}
          onChange={e => setMessage(e.target.value)}
          placeholder="Commit message..."
          rows={2}
          className="resize-none text-sm flex-1"
          data-testid="commit-message-input"
        />
        <Button
          size="icon"
          variant="ghost"
          onClick={fillAIMessage}
          disabled={aiLoading}
          title="Generate commit message with AI"
          data-testid="ai-commit-btn"
        >
          {aiLoading ? <Loader2 size={14} className="animate-spin" /> : <Bot size={14} />}
        </Button>
      </div>

      <div className="flex gap-2">
        <Button
          size="sm"
          onClick={handleCommit}
          disabled={!canCommit || isCommitting}
          className="flex-1"
          data-testid="commit-btn"
        >
          {isCommitting ? <Loader2 size={12} className="animate-spin mr-1" /> : null}
          Commit
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={handleCommitAndPush}
          disabled={!canCommit || isCommitting || isPushing}
          className="flex-1"
          data-testid="commit-push-btn"
        >
          Commit & Push
        </Button>
      </div>
    </div>
  )
}
