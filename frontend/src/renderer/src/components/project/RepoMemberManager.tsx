// RepoMemberManager.tsx — per-repo functional-role (developer/lead/admin)
// grant management.
//
// A second, separate authorization tier from MemberManager.tsx's project
// owner/member access tier: this decides what a project member can do on
// ONE specific repo within the project (a developer might be granted onto
// repo X but have no grant at all on repo Y in the same project) — see
// policy/orca-authz/repo.rego and project-service's requireRepoAccess.
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
import { toast } from 'sonner'
import { Trash2, UserPlus } from 'lucide-react'
import { describeMemberLabel, useTenantMemberDirectory } from '../../hooks/useTenantMemberDirectory'

// Same FORBIDDEN/UNAUTHENTICATED-message pattern as MemberManager.tsx/CreateProjectDialog.tsx.
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

type RepoRole = 'developer' | 'lead' | 'admin'
type RepoMember = { userId: string; role: RepoRole }

export function RepoMemberManager({ repoId }: { repoId: string }) {
  const [members, setMembers] = useState<RepoMember[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const { directory } = useTenantMemberDirectory()

  const [newUserId, setNewUserId] = useState('')
  const [newRole, setNewRole] = useState<RepoRole>('developer')
  const [adding, setAdding] = useState(false)

  // Why exclude existing grants: they already have a functional role here —
  // picking one again would either no-op or (worse) look like it changes
  // their role via the wrong control.
  const addableDirectory = directory.filter((entry) => !members.some((m) => m.userId === entry.id))

  const loadMembers = useCallback(async () => {
    setIsLoading(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<RepoMember[]>(target, 'repo.getMembers', { repoId })
      setMembers(result)
    } catch (err) {
      toast.error(describeError(err, 'Failed to load repo members'))
    } finally {
      setIsLoading(false)
    }
  }, [repoId])

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
      await callRuntimeRpc(target, 'repo.addMember', { repoId, userId, role: newRole })
      setNewUserId('')
      setNewRole('developer')
      toast.success('Repo member added')
      await loadMembers()
    } catch (err) {
      toast.error(describeError(err, 'Failed to add repo member'))
    } finally {
      setAdding(false)
    }
  }

  const updateRole = async (userId: string, role: RepoRole) => {
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'repo.updateMemberRole', { repoId, userId, role })
      setMembers((prev) => prev.map((m) => (m.userId === userId ? { ...m, role } : m)))
      toast.success('Role updated')
    } catch (err) {
      toast.error(describeError(err, 'Failed to update role'))
    }
  }

  const removeMember = async (userId: string) => {
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'repo.removeMember', { repoId, userId })
      setMembers((prev) => prev.filter((m) => m.userId !== userId))
      toast.success('Repo member removed')
    } catch (err) {
      toast.error(describeError(err, 'Failed to remove repo member'))
    }
  }

  return (
    <div className="repo-member-manager" data-testid="repo-member-manager">
      <p className="pb-2 text-xs text-muted-foreground">
        Grants a functional role on this specific repo — separate from project membership above.
      </p>
      <div className="flex items-end gap-2 pb-3" data-testid="add-repo-member-form">
        <div className="flex-1 grid gap-1.5">
          <Select
            value={newUserId}
            onValueChange={setNewUserId}
            data-testid="add-repo-member-user-id"
          >
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
        <Select value={newRole} onValueChange={(v) => setNewRole(v as RepoRole)}>
          <SelectTrigger className="w-32 h-9 text-xs" data-testid="add-repo-member-role">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="developer">Developer</SelectItem>
            <SelectItem value="lead">Lead</SelectItem>
            <SelectItem value="admin">Admin</SelectItem>
          </SelectContent>
        </Select>
        <Button
          type="button"
          size="icon"
          disabled={adding || !newUserId.trim()}
          onClick={addMember}
          data-testid="add-repo-member-submit"
          aria-label="Add repo member"
        >
          <UserPlus size={14} />
        </Button>
      </div>

      {isLoading ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="repo-member-loading">
          Loading repo members...
        </div>
      ) : members.length === 0 ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="repo-member-empty">
          No functional-role grants on this repo yet — the project owner always has full access
          regardless.
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
              <TableRow key={member.userId} data-testid={`repo-member-row-${member.userId}`}>
                <TableCell>
                  <p className="font-medium text-sm">
                    {describeMemberLabel(member.userId, directory)}
                  </p>
                </TableCell>
                <TableCell>
                  <Select
                    value={member.role}
                    onValueChange={(role) => updateRole(member.userId, role as RepoRole)}
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
                    data-testid={`remove-repo-member-${member.userId}`}
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
