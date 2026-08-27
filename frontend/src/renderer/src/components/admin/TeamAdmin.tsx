// TeamAdmin.tsx — Admin view for Team management (list teams, view/add/remove members)
import { useCallback, useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget, RuntimeRpcCallError } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Skeleton } from '../ui/skeleton'

type Team = {
  id:        string
  name:      string
  createdAt: string
  updatedAt: string
}

type TeamMember = {
  teamId:   string
  userId:   string
  role:     string
  priority: number
  addedAt:  string
}

// requireAdmin() on the backend throws a plain Error (no structured .code) —
// message is 'UNAUTHENTICATED' or starts with 'FORBIDDEN' — so detect it by
// message, not by RuntimeRpcCallError.code (which is always 'runtime_error' here).
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'Admin access required for this action.'
  }
  return message || fallback
}

export function TeamAdmin() {
  const [teams, setTeams]             = useState<Team[]>([])
  const [teamsLoading, setTeamsLoading] = useState(true)
  const [teamsError, setTeamsError]   = useState<string | null>(null)

  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null)
  const [members, setMembers]               = useState<TeamMember[]>([])
  const [membersLoading, setMembersLoading] = useState(false)
  const [membersError, setMembersError]     = useState<string | null>(null)

  const [newTeamName, setNewTeamName]     = useState('')
  const [creatingTeam, setCreatingTeam]   = useState(false)
  const [createTeamError, setCreateTeamError] = useState<string | null>(null)

  const [newMemberUserId, setNewMemberUserId]   = useState('')
  const [newMemberRole, setNewMemberRole]       = useState('')
  const [newMemberPriority, setNewMemberPriority] = useState('0')
  const [addingMember, setAddingMember]         = useState(false)
  const [addMemberError, setAddMemberError]     = useState<string | null>(null)

  const loadTeams = useCallback(() => {
    setTeamsLoading(true)
    setTeamsError(null)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    return callRuntimeRpc<Team[]>(target, 'team.list', {})
      .then(t => setTeams(t))
      .catch(err => {
        console.error('[TeamAdmin] failed to load teams:', err)
        setTeamsError(describeError(err, 'Failed to load teams'))
      })
      .finally(() => setTeamsLoading(false))
  }, [])

  useEffect(() => {
    loadTeams()
  }, [loadTeams])

  const loadMembers = useCallback((teamId: string) => {
    setMembersLoading(true)
    setMembersError(null)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<TeamMember[]>(target, 'team.listMembers', { teamId })
      .then(m => setMembers(m))
      .catch(err => {
        console.error('[TeamAdmin] failed to load members:', err)
        setMembers([])
        setMembersError(describeError(err, 'Failed to load members'))
      })
      .finally(() => setMembersLoading(false))
  }, [])

  const handleSelectTeam = (teamId: string) => {
    if (selectedTeamId === teamId) {
      setSelectedTeamId(null)
      setMembers([])
      setMembersError(null)
      return
    }
    setSelectedTeamId(teamId)
    loadMembers(teamId)
  }

  const handleCreateTeam = async (e: FormEvent) => {
    e.preventDefault()
    const name = newTeamName.trim()
    if (!name) {return}
    setCreatingTeam(true)
    setCreateTeamError(null)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      const team = await callRuntimeRpc<Team>(target, 'team.create', { name })
      setTeams(prev => [...prev, team])
      setNewTeamName('')
    } catch (err) {
      setCreateTeamError(describeError(err, 'Failed to create team'))
    } finally {
      setCreatingTeam(false)
    }
  }

  const handleAddMember = async (e: FormEvent) => {
    e.preventDefault()
    if (!selectedTeamId) {return}
    const userId = newMemberUserId.trim()
    const role = newMemberRole.trim()
    if (!userId || !role) {return}
    const priority = Number(newMemberPriority) || 0
    setAddingMember(true)
    setAddMemberError(null)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      await callRuntimeRpc(target, 'team.addMember', { teamId: selectedTeamId, userId, role, priority })
      setNewMemberUserId('')
      setNewMemberRole('')
      setNewMemberPriority('0')
      loadMembers(selectedTeamId)
    } catch (err) {
      setAddMemberError(describeError(err, 'Failed to add member'))
    } finally {
      setAddingMember(false)
    }
  }

  const handleRemoveMember = async (userId: string) => {
    if (!selectedTeamId) {return}
    setMembersError(null)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      await callRuntimeRpc(target, 'team.removeMember', { teamId: selectedTeamId, userId })
      loadMembers(selectedTeamId)
    } catch (err) {
      setMembersError(describeError(err, 'Failed to remove member'))
    }
  }

  if (teamsLoading) {
    return (
      <div className="team-admin p-4 space-y-2" data-testid="team-admin-loading">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
      </div>
    )
  }

  const selectedTeam = teams.find(t => t.id === selectedTeamId)

  return (
    <div className="team-admin space-y-6 p-4" data-testid="team-admin">
      <h2 className="text-xl font-semibold">Teams</h2>

      {teamsError && (
        <p className="text-sm text-destructive" data-testid="team-list-error">{teamsError}</p>
      )}

      {/* Create team form */}
      <form onSubmit={handleCreateTeam} className="flex items-end gap-2" data-testid="create-team-form">
        <div className="space-y-1">
          <Label htmlFor="new-team-name">New Team</Label>
          <Input
            id="new-team-name"
            value={newTeamName}
            onChange={e => setNewTeamName(e.target.value)}
            placeholder="Team name"
            data-testid="new-team-name-input"
          />
        </div>
        <Button type="submit" disabled={creatingTeam || !newTeamName.trim()} data-testid="create-team-btn">
          {creatingTeam ? 'Creating…' : 'Create Team'}
        </Button>
      </form>
      {createTeamError && (
        <p className="text-sm text-destructive" data-testid="create-team-error">{createTeamError}</p>
      )}

      {/* Team list */}
      {teams.length === 0 && !teamsError ? (
        <p className="text-sm text-muted-foreground" data-testid="team-list-empty">No teams yet.</p>
      ) : (
        <ul className="space-y-2" data-testid="team-list">
          {teams.map(team => (
            <li
              key={team.id}
              className="flex items-center justify-between rounded-md border border-border/50 px-3 py-2"
              data-testid={`team-row-${team.id}`}
            >
              <span className="text-sm font-medium">{team.name}</span>
              <Button
                variant={selectedTeamId === team.id ? 'default' : 'outline'}
                size="sm"
                onClick={() => handleSelectTeam(team.id)}
                data-testid={`view-members-btn-${team.id}`}
              >
                {selectedTeamId === team.id ? 'Hide Members' : 'View Members'}
              </Button>
            </li>
          ))}
        </ul>
      )}

      {/* Members view */}
      {selectedTeamId && (
        <div className="space-y-4 rounded-md border border-border/50 p-4" data-testid="team-members-panel">
          <h3 className="text-base font-medium">
            Members — {selectedTeam?.name}
          </h3>

          {membersError && (
            <p className="text-sm text-destructive" data-testid="team-members-error">{membersError}</p>
          )}

          {membersLoading ? (
            <div className="space-y-2" data-testid="team-members-loading">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-3/4" />
            </div>
          ) : members.length === 0 && !membersError ? (
            <p className="text-sm text-muted-foreground" data-testid="team-members-empty">No members yet.</p>
          ) : (
            <ul className="space-y-1" data-testid="team-members-list">
              {members.map(m => (
                <li
                  key={m.userId}
                  className="flex items-center justify-between gap-2 text-sm"
                  data-testid={`member-row-${m.userId}`}
                >
                  <span className="flex items-center gap-2">
                    {m.userId}
                    <Badge variant="outline">{m.role}</Badge>
                    <span className="text-muted-foreground">priority {m.priority}</span>
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleRemoveMember(m.userId)}
                    data-testid={`remove-member-btn-${m.userId}`}
                  >
                    Remove
                  </Button>
                </li>
              ))}
            </ul>
          )}

          {/* Add member form */}
          <form onSubmit={handleAddMember} className="flex flex-wrap items-end gap-2" data-testid="add-member-form">
            <div className="space-y-1">
              <Label htmlFor="new-member-user-id">User ID</Label>
              <Input
                id="new-member-user-id"
                value={newMemberUserId}
                onChange={e => setNewMemberUserId(e.target.value)}
                placeholder="userId"
                data-testid="new-member-user-id-input"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="new-member-role">Role</Label>
              <Input
                id="new-member-role"
                value={newMemberRole}
                onChange={e => setNewMemberRole(e.target.value)}
                placeholder="role"
                data-testid="new-member-role-input"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="new-member-priority">Priority</Label>
              <Input
                id="new-member-priority"
                type="number"
                value={newMemberPriority}
                onChange={e => setNewMemberPriority(e.target.value)}
                data-testid="new-member-priority-input"
              />
            </div>
            <Button
              type="submit"
              size="sm"
              disabled={addingMember || !newMemberUserId.trim() || !newMemberRole.trim()}
              data-testid="add-member-btn"
            >
              {addingMember ? 'Adding…' : 'Add Member'}
            </Button>
          </form>
          {addMemberError && (
            <p className="text-sm text-destructive" data-testid="add-member-error">{addMemberError}</p>
          )}
        </div>
      )}
    </div>
  )
}
