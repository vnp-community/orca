// LinkedProjectsManager.tsx — orca_project_source_projects link/unlink table
// (BUG-FE-PW-002 fix, mirrors MemberManager.tsx's shape/pattern)
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
import type { SourceProjectRef, OrcaProjectListItemWithSources } from '../../types/workspace-types'
import { toast } from 'sonner'
import { Link2, Trash2 } from 'lucide-react'

// Same FORBIDDEN/UNAUTHENTICATED-message pattern as MemberManager.tsx/CreateProjectDialog.tsx.
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

export function LinkedProjectsManager({
  orcaProjectId,
  currentUserRole
}: {
  orcaProjectId: string
  // Passed down by ProjectSettings — do not re-derive RBAC here. The backend
  // (requireOwnerOrAdmin in orca-project-sharing-rpc-handler.ts) is the real
  // enforcement point; this only controls whether the Unlink button renders.
  currentUserRole: 'owner' | 'member' | null
}) {
  const [sourceProjects, setSourceProjects] = useState<SourceProjectRef[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [linking, setLinking] = useState(false)

  // Project of the current user — legacy, per-user, multi-host model, already
  // in the client store (repos.ts slice). No RPC needed to list these. Read
  // via getState() (not the useAppStore(selector) hook form) to match the
  // established pattern in components/project/* (MemberManager.tsx, etc.) —
  // this component already re-renders on load()/link()/unlink() state
  // changes, so this stays current without a store subscription.
  const myProjects = useAppStore.getState().projects ?? []
  const canUnlink = currentUserRole === 'owner'

  const load = useCallback(async () => {
    setIsLoading(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // orcaProjects.list() returns every OrcaProject the caller is a member
      // of — there is no per-id fetch RPC, so filter client-side.
      const all = await callRuntimeRpc<OrcaProjectListItemWithSources[]>(
        target,
        'orcaProjects.list',
        null
      )
      const mine = all.find((item) => item.orcaProject.id === orcaProjectId)
      setSourceProjects(mine?.sourceProjects ?? [])
    } catch {
      toast.error('Failed to load linked projects')
    } finally {
      setIsLoading(false)
    }
  }, [orcaProjectId])

  useEffect(() => {
    load()
  }, [load])

  const linkProject = async () => {
    if (!selectedProjectId) {
      return
    }
    setLinking(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'orcaProjects.linkSourceProject', {
        orcaProjectId,
        projectId: selectedProjectId
      })
      setSelectedProjectId('')
      toast.success('Project linked')
      await load()
    } catch (err) {
      toast.error(describeError(err, 'Failed to link project'))
    } finally {
      setLinking(false)
    }
  }

  const unlinkProject = async (projectId: string) => {
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'orcaProjects.unlinkSourceProject', { orcaProjectId, projectId })
      setSourceProjects((prev) => prev.filter((s) => s.projectId !== projectId))
      toast.success('Project unlinked')
    } catch (err) {
      toast.error(describeError(err, 'Failed to unlink project'))
    }
  }

  // Exclude already-linked projects from the picker — linkSourceProject is
  // idempotent server-side, but there is no reason to offer a redundant pick.
  const linkedIds = new Set(sourceProjects.map((s) => s.projectId))
  const linkableProjects = myProjects.filter((p) => !linkedIds.has(p.id))

  return (
    <div className="linked-projects-manager" data-testid="linked-projects-manager">
      <div className="flex items-end gap-2 pb-3" data-testid="link-project-form">
        <div className="flex-1 grid gap-1.5">
          <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
            <SelectTrigger data-testid="link-project-select">
              <SelectValue
                placeholder={
                  linkableProjects.length === 0 ? 'No projects to link' : 'Choose a Project'
                }
              />
            </SelectTrigger>
            <SelectContent>
              {linkableProjects.map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.displayName}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button
          type="button"
          size="icon"
          disabled={linking || !selectedProjectId}
          onClick={linkProject}
          data-testid="link-project-submit"
          aria-label="Link project"
        >
          <Link2 size={14} />
        </Button>
      </div>

      {isLoading ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="linked-loading">
          Loading…
        </div>
      ) : sourceProjects.length === 0 ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="linked-empty">
          No projects linked yet.
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Project</TableHead>
              <TableHead>Owner</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {sourceProjects.map((s) => {
              const label = myProjects.find((p) => p.id === s.projectId)?.displayName ?? s.projectId
              return (
                <TableRow key={s.projectId} data-testid={`linked-row-${s.projectId}`}>
                  <TableCell>
                    <p className="font-medium text-sm">{label}</p>
                  </TableCell>
                  <TableCell>
                    <p className="text-xs text-muted-foreground">{s.ownerUserId}</p>
                  </TableCell>
                  <TableCell>
                    {canUnlink ? (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7"
                        onClick={() => unlinkProject(s.projectId)}
                        data-testid={`unlink-project-${s.projectId}`}
                      >
                        <Trash2 size={12} />
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
