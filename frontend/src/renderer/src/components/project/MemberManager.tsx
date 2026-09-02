// MemberManager.tsx — Project member CRUD table (TDD-FE-12, TASK-FE-004)
//
// Scoped to the project-level *access* tier only (owner/member — who's in
// the project at all). The per-repo functional-role tier (admin/developer/
// lead — what someone can do on a specific repo within the project) is a
// separate, repo-scoped concept with its own UI, not this component.
import { useState, useEffect, useCallback } from 'react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { Button } from '../ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import {
  callRuntimeRpc,
  getActiveRuntimeTarget,
  RuntimeRpcCallError
} from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import type { ProjectMember } from '../../types/workspace-types'
import { toast } from 'sonner'
import { Trash2, UserPlus } from 'lucide-react'
import { describeMemberLabel, useTenantMemberDirectory } from '../../hooks/useTenantMemberDirectory'

// Same FORBIDDEN/UNAUTHENTICATED-message pattern as CreateProjectDialog.tsx/TeamAdmin.tsx.
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

export function MemberManager({ projectId }: { projectId: string }) {
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const { directory } = useTenantMemberDirectory()

  const [newUserId, setNewUserId] = useState('')
  const [newRole, setNewRole] = useState<ProjectMember['role']>('member')
  const [adding, setAdding] = useState(false)

  // Why exclude existing members: they're already added — picking one
  // again would either no-op or (worse) look like it changes their role
  // via the wrong control.
  const addableDirectory = directory.filter((entry) => !members.some((m) => m.userId === entry.id))

  const loadMembers = useCallback(async () => {
    setIsLoading(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<ProjectMember[]>(target, 'project.getMembers', {
        projectId
      })
      setMembers(result)
    } catch {
      toast.error('Failed to load members')
    } finally {
      setIsLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    loadMembers()
  }, [loadMembers])

  const addMember = async () => {
    const userId = newUserId.trim()
    if (!userId) {
      return
    }
    setAdding(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'project.addMember', { projectId, userId, role: newRole })
      setNewUserId('')
      setNewRole('member')
      toast.success('Member added')
      await loadMembers()
    } catch (err) {
      toast.error(describeError(err, 'Failed to add member'))
    } finally {
      setAdding(false)
    }
  }

  const updateRole = async (userId: string, role: ProjectMember['role']) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'project.updateMemberRole', { projectId, userId, role })
    setMembers((prev) => prev.map((m) => (m.userId === userId ? { ...m, role } : m)))
    toast.success('Role updated')
  }

  const removeMember = async (userId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'project.removeMember', { projectId, userId })
    setMembers((prev) => prev.filter((m) => m.userId !== userId))
    toast.success('Member removed')
  }

  return (
    <div className="member-manager" data-testid="member-manager">
      <div className="flex items-end gap-2 pb-3" data-testid="add-member-form">
        <div className="flex-1 grid gap-1.5">
          <Select value={newUserId} onValueChange={setNewUserId} data-testid="add-member-user-id">
            <SelectTrigger className="h-9 text-xs">
              <SelectValue placeholder="Select a person…" />
            </SelectTrigger>
            <SelectContent>
              {addableDirectory.length === 0 ? (
                <div className="px-2 py-1.5 text-xs text-muted-foreground">
                  No other tenant members to add.
                </div>
              ) : (
                addableDirectory.map((entry) => (
                  <SelectItem key={entry.id} value={entry.id}>
                    {entry.name} ({entry.email})
                  </SelectItem>
                ))
              )}
            </SelectContent>
          </Select>
        </div>
        <Select value={newRole} onValueChange={(v) => setNewRole(v as ProjectMember['role'])}>
          <SelectTrigger className="w-28 h-9 text-xs" data-testid="add-member-role">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="member">Member</SelectItem>
            <SelectItem value="owner">Owner</SelectItem>
          </SelectContent>
        </Select>
        <Button
          type="button"
          size="icon"
          disabled={adding || !newUserId.trim()}
          onClick={addMember}
          data-testid="add-member-submit"
          aria-label="Add member"
        >
          <UserPlus size={14} />
        </Button>
      </div>

      {isLoading ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="member-loading">
          Loading members...
        </div>
      ) : members.length === 0 ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="member-empty">
          No members found.
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Member</TableHead>
              <TableHead>Role</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.map((member) => (
              <TableRow key={member.userId} data-testid={`member-row-${member.userId}`}>
                <TableCell>
                  <p className="font-medium text-sm">
                    {describeMemberLabel(member.userId, directory)}
                  </p>
                </TableCell>
                <TableCell>
                  <Select
                    value={member.role}
                    onValueChange={(role) =>
                      updateRole(member.userId, role as ProjectMember['role'])
                    }
                  >
                    <SelectTrigger className="w-28 h-7 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="member">Member</SelectItem>
                      <SelectItem value="owner">Owner</SelectItem>
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
      )}
    </div>
  )
}
