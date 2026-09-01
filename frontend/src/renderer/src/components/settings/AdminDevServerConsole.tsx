// AdminDevServerConsole.tsx — CR-DS-006/007/008 admin console.
// Settings pane (admin-only, gated by the caller in Settings.tsx) covering
// the three admin surfaces this feature needs: approve/reject dev-server
// agents + assign them to groups, manage group↔department grants, and
// resolve pending access requests.
import { useCallback, useEffect, useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import type {
  DevServer,
  DevServerGroup,
  DevServerAccessRequest,
  DevServerGroupGrant
} from '../../../../shared/dev-server-types'
import type { TenantDepartment } from '../../../../shared/tenant-user-profile-types'

function useDevServers(): {
  servers: DevServer[]
  loading: boolean
  reload: () => void
} {
  const [servers, setServers] = useState<DevServer[]>([])
  const [loading, setLoading] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    setLoading(true)
    window.api.devServer
      .list()
      .then(setServers)
      .catch(() => toast.error('Failed to load dev servers'))
      .finally(() => setLoading(false))
  }, [reloadToken])

  return { servers, loading, reload: () => setReloadToken((n) => n + 1) }
}

function useDevServerGroups(): {
  groups: DevServerGroup[]
  loading: boolean
  reload: () => void
} {
  const [groups, setGroups] = useState<DevServerGroup[]>([])
  const [loading, setLoading] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    setLoading(true)
    window.api.devServerGroup
      .list()
      .then(setGroups)
      .catch(() => toast.error('Failed to load dev server groups'))
      .finally(() => setLoading(false))
  }, [reloadToken])

  return { groups, loading, reload: () => setReloadToken((n) => n + 1) }
}

function ApprovalsTab(): React.JSX.Element {
  const { servers, loading, reload } = useDevServers()
  const { groups } = useDevServerGroups()
  const [pendingGroupChoice, setPendingGroupChoice] = useState<Record<string, string>>({})
  const [busyId, setBusyId] = useState<string | null>(null)

  const runAction = useCallback(
    (id: string, action: () => Promise<unknown>) => {
      setBusyId(id)
      action()
        .then(() => reload())
        .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
        .finally(() => setBusyId(null))
    },
    [reload]
  )

  if (loading && servers.length === 0) {
    return <p className="text-sm text-muted-foreground">Loading…</p>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Group</TableHead>
          <TableHead className="text-right">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {servers.map((server) => {
          const status = server.approvalStatus ?? 'pending_approval'
          const isBusy = busyId === server.id
          return (
            <TableRow key={server.id}>
              <TableCell>{server.name}</TableCell>
              <TableCell>
                <Badge
                  variant={
                    status === 'approved'
                      ? 'default'
                      : status === 'rejected'
                        ? 'destructive'
                        : 'secondary'
                  }
                >
                  {status}
                </Badge>
              </TableCell>
              <TableCell>
                {status === 'approved' ? (
                  <Select
                    value={pendingGroupChoice[server.id] ?? server.groupId ?? ''}
                    onValueChange={(value) =>
                      setPendingGroupChoice((prev) => ({ ...prev, [server.id]: value }))
                    }
                  >
                    <SelectTrigger className="h-8 w-[180px]">
                      <SelectValue placeholder="Ungrouped" />
                    </SelectTrigger>
                    <SelectContent>
                      {groups.map((group) => (
                        <SelectItem key={group.id} value={group.id}>
                          {group.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <span className="text-sm text-muted-foreground">—</span>
                )}
              </TableCell>
              <TableCell className="flex justify-end gap-2">
                {status === 'pending_approval' ? (
                  <>
                    <Button
                      size="sm"
                      disabled={isBusy}
                      onClick={() =>
                        runAction(server.id, () => window.api.devServer.approve(server.id))
                      }
                    >
                      Approve
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={isBusy}
                      onClick={() =>
                        runAction(server.id, () => window.api.devServer.reject(server.id))
                      }
                    >
                      Reject
                    </Button>
                  </>
                ) : null}
                {status === 'approved' &&
                pendingGroupChoice[server.id] &&
                pendingGroupChoice[server.id] !== server.groupId ? (
                  <Button
                    size="sm"
                    disabled={isBusy}
                    onClick={() =>
                      runAction(server.id, () =>
                        window.api.devServer.assignGroup(server.id, pendingGroupChoice[server.id])
                      )
                    }
                  >
                    Save group
                  </Button>
                ) : null}
              </TableCell>
            </TableRow>
          )
        })}
        {servers.length === 0 ? (
          <TableRow>
            <TableCell colSpan={4} className="text-center text-sm text-muted-foreground">
              No dev servers connected yet.
            </TableCell>
          </TableRow>
        ) : null}
      </TableBody>
    </Table>
  )
}

function GroupsAndGrantsTab(): React.JSX.Element {
  const { groups, loading, reload } = useDevServerGroups()
  const [departments, setDepartments] = useState<TenantDepartment[]>([])
  const [newGroupName, setNewGroupName] = useState('')
  const [creating, setCreating] = useState(false)
  const [grantChoice, setGrantChoice] = useState<Record<string, string>>({})
  const [grantsByGroup, setGrantsByGroup] = useState<Record<string, DevServerGroupGrant[]>>({})

  useEffect(() => {
    window.api.tenantProfile
      .listDepartments()
      .then(setDepartments)
      .catch(() => {})
  }, [])

  const loadGrants = useCallback((groupId: string) => {
    window.api.devServerGroup
      .listGrants(groupId)
      .then((grants) => setGrantsByGroup((prev) => ({ ...prev, [groupId]: grants })))
      .catch(() => toast.error('Failed to load grants'))
  }, [])

  useEffect(() => {
    groups.forEach((group) => loadGrants(group.id))
  }, [groups, loadGrants])

  const handleCreateGroup = useCallback(() => {
    if (!newGroupName.trim() || creating) {
      return
    }
    setCreating(true)
    window.api.devServerGroup
      .create({ name: newGroupName.trim() })
      .then(() => {
        setNewGroupName('')
        reload()
      })
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
      .finally(() => setCreating(false))
  }, [newGroupName, creating, reload])

  const handleGrant = useCallback(
    (groupId: string) => {
      const departmentId = grantChoice[groupId]
      if (!departmentId) {
        return
      }
      window.api.devServerGroup
        .grant({ devServerGroupId: groupId, granteeKind: 'department', granteeId: departmentId })
        .then(() => loadGrants(groupId))
        .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
    },
    [grantChoice, loadGrants]
  )

  const handleRevoke = useCallback(
    (groupId: string, grantId: string) => {
      window.api.devServerGroup
        .revoke(grantId)
        .then(() => loadGrants(groupId))
        .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
    },
    [loadGrants]
  )

  const departmentName = useCallback(
    (id: string) => departments.find((d) => d.id === id)?.name ?? id,
    [departments]
  )

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-end gap-2">
        <Input
          value={newGroupName}
          onChange={(e) => setNewGroupName(e.target.value)}
          placeholder="New group name"
          className="max-w-xs"
        />
        <Button disabled={!newGroupName.trim() || creating} onClick={handleCreateGroup}>
          Create group
        </Button>
      </div>

      {loading && groups.length === 0 ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : (
        <div className="flex flex-col gap-4">
          {groups.map((group) => (
            <div key={group.id} className="rounded-lg border border-border p-4">
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-sm font-semibold">{group.name}</h3>
              </div>
              <div className="flex flex-wrap gap-2">
                {(grantsByGroup[group.id] ?? []).map((grant) => (
                  <Badge key={grant.id} variant="secondary" className="gap-1">
                    {grant.granteeKind}: {departmentName(grant.granteeId)}
                    <button
                      type="button"
                      className="ml-1 text-muted-foreground hover:text-foreground"
                      onClick={() => handleRevoke(group.id, grant.id)}
                      aria-label={`Revoke grant ${grant.id}`}
                    >
                      ×
                    </button>
                  </Badge>
                ))}
              </div>
              <div className="mt-3 flex items-end gap-2">
                <Select
                  value={grantChoice[group.id] ?? ''}
                  onValueChange={(value) =>
                    setGrantChoice((prev) => ({ ...prev, [group.id]: value }))
                  }
                >
                  <SelectTrigger className="h-8 w-[220px]">
                    <SelectValue placeholder="Grant a department access" />
                  </SelectTrigger>
                  <SelectContent>
                    {departments.map((dept) => (
                      <SelectItem key={dept.id} value={dept.id}>
                        {dept.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button
                  size="sm"
                  disabled={!grantChoice[group.id]}
                  onClick={() => handleGrant(group.id)}
                >
                  Grant
                </Button>
              </div>
            </div>
          ))}
          {groups.length === 0 ? (
            <p className="text-sm text-muted-foreground">No groups yet — create one above.</p>
          ) : null}
        </div>
      )}
      {/* Why: team-based grants aren't wired up yet — tenant-service has no
          "list teams for a user" RPC, so ListDevServersForUser's team_ids is
          always empty server-side too (documented gap, BE-SOL-003). Only
          department grants are exposed here until that follow-up lands. */}
    </div>
  )
}

function AccessRequestsTab(): React.JSX.Element {
  const [requests, setRequests] = useState<DevServerAccessRequest[]>([])
  const { groups } = useDevServerGroups()
  const [loading, setLoading] = useState(false)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    setLoading(true)
    window.api.devServer
      .listPendingAccessRequests()
      .then(setRequests)
      .catch(() => toast.error('Failed to load access requests'))
      .finally(() => setLoading(false))
  }, [reloadToken])

  const groupName = useCallback(
    (groupId: string) => groups.find((g) => g.id === groupId)?.name ?? groupId,
    [groups]
  )

  const resolve = useCallback((requestId: string, approve: boolean) => {
    setBusyId(requestId)
    window.api.devServer
      .resolveAccessRequest({ requestId, approve })
      .then(() => setReloadToken((n) => n + 1))
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
      .finally(() => setBusyId(null))
  }, [])

  if (loading && requests.length === 0) {
    return <p className="text-sm text-muted-foreground">Loading…</p>
  }

  if (requests.length === 0) {
    return <p className="text-sm text-muted-foreground">No pending access requests.</p>
  }

  return (
    <div className="flex flex-col gap-3">
      {requests.map((req) => (
        <div
          key={req.id}
          className="flex items-center justify-between rounded-lg border border-border p-4"
        >
          <div>
            <p className="text-sm font-medium">{groupName(req.devServerGroupId)}</p>
            {req.message ? (
              <p className="mt-1 text-sm text-muted-foreground">{req.message}</p>
            ) : null}
          </div>
          <div className="flex gap-2">
            <Button size="sm" disabled={busyId === req.id} onClick={() => resolve(req.id, true)}>
              Approve
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busyId === req.id}
              onClick={() => resolve(req.id, false)}
            >
              Reject
            </Button>
          </div>
        </div>
      ))}
    </div>
  )
}

export function AdminDevServerConsole(): React.JSX.Element {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <ShieldCheck className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
        <h2 className="text-base font-semibold">
          {translate('auto.components.settings.AdminDevServerConsole.title', 'Admin console')}
        </h2>
      </div>
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.components.settings.AdminDevServerConsole.description',
          'Approve connected dev server agents, assign them to groups, grant department access, and resolve access requests.'
        )}
      </p>
      <Tabs defaultValue="approvals">
        <TabsList>
          <TabsTrigger value="approvals">Approvals</TabsTrigger>
          <TabsTrigger value="groups">Groups &amp; access</TabsTrigger>
          <TabsTrigger value="requests">Access requests</TabsTrigger>
        </TabsList>
        <TabsContent value="approvals">
          <ApprovalsTab />
        </TabsContent>
        <TabsContent value="groups">
          <GroupsAndGrantsTab />
        </TabsContent>
        <TabsContent value="requests">
          <AccessRequestsTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
