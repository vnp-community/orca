// MemberManager.tsx — Project member CRUD table (TDD-FE-12, TASK-FE-004)
import { useState, useEffect, useCallback } from 'react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../ui/table'
import { Button } from '../ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/select'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'

type ProjectRole = 'developer' | 'lead' | 'admin'

type ProjectMember = {
  userId:      string
  displayName: string
  email:       string
  role:        ProjectRole
  joinedAt:    Date
}

export function MemberManager({ projectId }: { projectId: string }) {
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const loadMembers = useCallback(async () => {
    setIsLoading(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<ProjectMember[]>(
        target,
        'projects.listMembers',
        { projectId }
      )
      setMembers(result)
    } catch {
      toast.error('Failed to load members')
    } finally {
      setIsLoading(false)
    }
  }, [projectId])

  useEffect(() => { loadMembers() }, [loadMembers])

  const updateRole = async (userId: string, role: ProjectRole) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'projects.updateMemberRole', { projectId, userId, role })
    setMembers(prev => prev.map(m => m.userId === userId ? { ...m, role } : m))
    toast.success('Role updated')
  }

  const removeMember = async (userId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'projects.removeMember', { projectId, userId })
    setMembers(prev => prev.filter(m => m.userId !== userId))
    toast.success('Member removed')
  }

  if (isLoading) {
    return (
      <div className="p-4 text-sm text-muted-foreground" data-testid="member-loading">
        Loading members...
      </div>
    )
  }

  if (members.length === 0) {
    return (
      <div className="p-4 text-sm text-muted-foreground" data-testid="member-empty">
        No members found.
      </div>
    )
  }

  return (
    <div className="member-manager" data-testid="member-manager">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Member</TableHead>
            <TableHead>Role</TableHead>
            <TableHead className="w-10" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {members.map(member => (
            <TableRow key={member.userId} data-testid={`member-row-${member.userId}`}>
              <TableCell>
                <div>
                  <p className="font-medium text-sm">{member.displayName}</p>
                  <p className="text-xs text-muted-foreground">{member.email}</p>
                </div>
              </TableCell>
              <TableCell>
                <Select
                  value={member.role}
                  onValueChange={role => updateRole(member.userId, role as ProjectRole)}
                >
                  <SelectTrigger className="w-32 h-7 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="developer">Developer</SelectItem>
                    <SelectItem value="lead">Lead</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                  </SelectContent>
                </Select>
              </TableCell>
              <TableCell>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => removeMember(member.userId)}
                  data-testid={`remove-member-${member.userId}`}
                >
                  <Trash2 size={12} />
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
