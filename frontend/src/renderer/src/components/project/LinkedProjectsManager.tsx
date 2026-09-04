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
import type {
  SourceProjectRef,
  OrcaProjectListItemWithSources,
  OrcaProject
} from '../../types/workspace-types'
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
  // Real OrcaProjects the caller belongs to (excluding this container) —
  // link-picker candidates, and the first-choice source for resolving a
  // linked project's display name. MUST be real project-service rows (real
  // UUIDs from orcaProjects.list), NOT the client-only "Project Host Setup"
  // projection (useAppStore's `projects`, built by
  // projectHostSetupProjectionFromRepos — ids like `github:owner/repo` or
  // `repo:<repoId>`, never a project.projects UUID): sending one of those as
  // linkSourceProject's projectId makes the backend's UUID-column lookup
  // throw (PROJECT_MEMBERSHIP_LOOKUP_FAILED) instead of cleanly denying —
  // confirmed live on b15.openledger.vn linking "vnp-asm".
  const [linkableProjects, setLinkableProjects] = useState<OrcaProject[]>([])
  // Real display names for linked projects that aren't in linkableProjects
  // (e.g. linked by another OrcaProject member the caller doesn't also
  // belong to) — resolved via orcaProjects.getProjectData, keyed by
  // projectId. Without this, those rows fall back to a raw UUID.
  const [resolvedNames, setResolvedNames] = useState<Record<string, string>>({})

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
      const sources = mine?.sourceProjects ?? []
      setSourceProjects(sources)

      const myOrcaProjects = all
        .map((item) => item.orcaProject)
        .filter((p) => p.id !== orcaProjectId)
      setLinkableProjects(myOrcaProjects)

      // A linked project isn't necessarily one the caller also belongs to
      // (it may have been linked by a different OrcaProject member) —
      // resolve those via getProjectData rather than showing a raw UUID.
      // Best effort: a resolve failure (e.g. since-revoked access) just
      // leaves that row falling back to the UUID, same as before this fix.
      const knownIds = new Set(myOrcaProjects.map((p) => p.id))
      const unresolved = sources.filter((s) => !knownIds.has(s.projectId))
      if (unresolved.length > 0) {
        const results = await Promise.allSettled(
          unresolved.map((s) =>
            callRuntimeRpc<{ project: { name: string } }>(target, 'orcaProjects.getProjectData', {
              orcaProjectId,
              projectId: s.projectId
            })
          )
        )
        setResolvedNames((prev) => {
          const next = { ...prev }
          results.forEach((result, i) => {
            // Guard against a malformed/empty response, not just a
            // rejection — either way, that row just keeps falling back to
            // the raw UUID rather than crashing the whole load.
            const name = result.status === 'fulfilled' ? result.value?.project?.name : undefined
            if (name) {
              next[unresolved[i].projectId] = name
            }
          })
          return next
        })
      }
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
  const pickableProjects = linkableProjects.filter((p) => !linkedIds.has(p.id))

  return (
    <div className="linked-projects-manager" data-testid="linked-projects-manager">
      <div className="flex items-end gap-2 pb-3" data-testid="link-project-form">
        <div className="flex-1 grid gap-1.5">
          <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
            <SelectTrigger data-testid="link-project-select">
              <SelectValue
                placeholder={
                  pickableProjects.length === 0 ? 'No projects to link' : 'Choose a Project'
                }
              />
            </SelectTrigger>
            <SelectContent>
              {pickableProjects.map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.name}
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
              const label =
                linkableProjects.find((p) => p.id === s.projectId)?.name ??
                resolvedNames[s.projectId] ??
                s.projectId
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
