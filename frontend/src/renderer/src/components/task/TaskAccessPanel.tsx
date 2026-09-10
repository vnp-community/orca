import { useState } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useTaskPermission } from '../../hooks/useTaskPermission'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '../ui/select'
import type { TaskGrantLevel } from '../../../../shared/task-types'

export function TaskAccessPanel({ taskId }: { taskId: string }) {
  const currentUserId = useAppStore((s) => s.currentUser?.id)
  const { level, isSupported } = useTaskPermission(taskId, currentUserId)
  const [subjectId, setSubjectId] = useState('')
  const [grantLevel, setGrantLevel] = useState<TaskGrantLevel>('user')
  const [applyTree, setApplyTree] = useState(false)
  const [granting, setGranting] = useState(false)

  if (!isSupported) {
    return (
      <div className="text-xs text-muted-foreground p-3" data-testid="task-access-unsupported">
        Access control chưa khả dụng — RPC `task.grant`/`task.resolvePermission` chưa được wire ở
        backend hiện tại. Theo dõi: BUG-TASKV1-003.
      </div>
    )
  }

  const submitGrant = async () => {
    if (!subjectId.trim()) {
      return
    }
    setGranting(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'task.grant', {
        taskId,
        subjectId: subjectId.trim(),
        level: `GRANT_LEVEL_${grantLevel.toUpperCase()}`,
        applyTree
      })
      toast.success(`Đã cấp quyền ${grantLevel} cho ${subjectId}`)
      setSubjectId('')
    } catch (err) {
      toast.error(`Không thể cấp quyền: ${(err as Error).message}`)
    } finally {
      setGranting(false)
    }
  }

  return (
    <div className="space-y-4 p-3" data-testid="task-access-panel">
      <div className="text-xs">
        <span className="font-medium">Quyền của bạn trên task này: </span>
        <span data-testid="own-permission-badge">{level ?? 'đang tải…'}</span>
      </div>
      <div className="space-y-2 border-t pt-3">
        <p className="text-xs font-semibold">Cấp quyền cho user/team khác</p>
        <p className="text-xs text-muted-foreground">
          Lưu ý: chưa có cách xem lại danh sách đã cấp sau khi submit (backend chưa có RPC
          list-grants công khai — BUG-TASKV1-003).
        </p>
        <Input
          value={subjectId}
          onChange={(e) => setSubjectId(e.target.value)}
          placeholder="User ID hoặc Team ID"
        />
        <Select value={grantLevel} onValueChange={(v) => setGrantLevel(v as TaskGrantLevel)}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(['admin', 'user', 'team', 'company'] as const).map((l) => (
              <SelectItem key={l} value={l}>
                {l}
              </SelectItem>
            ))}
            {/* 'owner' cố tình không cho chọn ở đây — cấp owner nên là hành động riêng */}
          </SelectContent>
        </Select>
        <label className="flex items-center gap-2 text-xs">
          <input
            type="checkbox"
            checked={applyTree}
            onChange={(e) => setApplyTree(e.target.checked)}
          />
          Áp dụng cho toàn bộ subtask (apply_tree)
        </label>
        <Button size="sm" disabled={granting || !subjectId.trim()} onClick={submitGrant}>
          {granting ? 'Đang cấp…' : 'Cấp quyền'}
        </Button>
      </div>
    </div>
  )
}
