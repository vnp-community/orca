import { useState } from 'react'
import { useTaskComments } from '../../hooks/useTaskComments'
import { Input } from '../ui/input'
import { Button } from '../ui/button'

export function TaskComments({ taskId }: { taskId: string }) {
  const { comments, addComment, isSupported } = useTaskComments(taskId)
  const [text, setText] = useState('')

  if (!isSupported) {
    return (
      <div className="text-xs text-muted-foreground p-3" data-testid="task-comments-unsupported">
        Comments chưa khả dụng trên backend hiện tại (chỉ hỗ trợ ở Node deploy target). Theo dõi:
        BUG-TASKV1-001 (task_comments table chưa có RPC ở backend-go).
      </div>
    )
  }

  const handleSend = (): void => {
    if (!text.trim()) {
      return
    }
    void addComment(text)
    setText('')
  }

  return (
    <div className="task-comments space-y-2 p-3" data-testid="task-comments">
      <div className="space-y-1 max-h-64 overflow-y-auto">
        {comments.map((c) => (
          <div key={c.id} className="text-xs">
            <span className="font-medium">{c.userId}</span>
            <p className="text-muted-foreground">{c.content}</p>
          </div>
        ))}
        {comments.length === 0 && (
          <p className="text-xs text-muted-foreground">Chưa có comment nào.</p>
        )}
      </div>
      <div className="flex gap-2">
        <Input
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Viết comment..."
          data-testid="task-comment-input"
        />
        <Button
          size="sm"
          disabled={!text.trim()}
          onClick={handleSend}
          data-testid="task-comment-send"
        >
          Gửi
        </Button>
      </div>
    </div>
  )
}
