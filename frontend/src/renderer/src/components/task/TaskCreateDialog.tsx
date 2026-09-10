import { useState } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { toast } from 'sonner'

export function TaskCreateDialog({
  projectId,
  onCreated,
  onCancel
}: {
  projectId: string
  onCreated: () => void
  onCancel: () => void
}) {
  const [title, setTitle] = useState('')
  const [isCreating, setIsCreating] = useState(false)

  const create = async () => {
    if (!title.trim()) {
      return
    }
    setIsCreating(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      // LƯU Ý: backend hôm nay CHỈ decode {title, parentId} (channels.go:278-291's
      // createArgs) — projectId gửi ở đây bị bỏ qua âm thầm cho tới khi 1 thay đổi
      // backend-go riêng thêm field ProjectID (xem FE-TASK-001's "Xác nhận đã đọc
      // code thật"). Vẫn gửi projectId ngay từ bây giờ để không phải sửa lại call
      // site khi backend fix xong.
      await callRuntimeRpc(target, 'task.create', { title: title.trim(), projectId })
      toast.success(`Task "${title.trim()}" created`)
      onCreated()
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(`Failed to create task: ${message}`)
    } finally {
      setIsCreating(false)
    }
  }

  return (
    <div
      className="task-create-dialog border rounded p-3 bg-background shadow-sm"
      data-testid="task-create-dialog"
    >
      <Input
        autoFocus
        placeholder="Task title..."
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && create()}
        data-testid="task-create-title-input"
        className="mb-2"
      />
      <div className="flex gap-2 justify-end">
        <Button size="sm" variant="ghost" onClick={onCancel} data-testid="task-create-cancel">
          Cancel
        </Button>
        <Button
          size="sm"
          disabled={!title.trim() || isCreating}
          onClick={create}
          data-testid="task-create-submit"
        >
          {isCreating ? 'Creating...' : 'Create'}
        </Button>
      </div>
    </div>
  )
}
