// CreateProjectDialog.tsx — "Create New Project" form for ProjectSwitcher (F38 Workspace, TDD-FE-12)
import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { toast } from 'sonner'
import {
  callRuntimeRpc,
  getActiveRuntimeTarget,
  RuntimeRpcCallError
} from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import type { OrcaProject } from '../../types/workspace-types'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '../ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'

type DevServerOption = { id: string; name: string; status: string }

type ProjectVisibility = 'private' | 'team' | 'department' | 'company'

// BUG-FE-PW-001: two ways to seed a new OrcaProject — either register a fresh
// repo path on a dev server (existing behavior, creates a brand-new Go-native
// Repo via repo.add), or link one of the caller's own pre-existing OrcaProjects
// into this new one for sharing (orcaProjects.linkSourceProject). The link
// picker MUST list real OrcaProjects (project.list, real project.projects
// UUIDs) — NOT the client-only "Project Host Setup" projection (ids like
// `github:owner/repo`, never a project.projects row): sending one of those as
// linkSourceProject's projectId makes the backend's UUID-column lookup throw
// (PROJECT_MEMBERSHIP_LOOKUP_FAILED) instead of cleanly denying — confirmed
// live on b15.openledger.vn. See docs/guides/authorization/
// asset-hierarchy-and-permission-model.md "Project vs OrcaProject".
type DialogMode = 'new-repo' | 'link'

type CreateProjectDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (project: OrcaProject) => void
}

// requireAdmin()/assertAccess() throw a plain Error with no structured .code —
// message starts with 'FORBIDDEN' or is 'UNAUTHENTICATED' (same pattern as TeamAdmin.tsx).
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to create a project.'
  }
  return message || fallback
}

export function CreateProjectDialog({ open, onOpenChange, onCreated }: CreateProjectDialogProps) {
  const [devServers, setDevServers] = useState<DevServerOption[]>([])
  const [devServersLoading, setDevServersLoading] = useState(false)
  const [myOrcaProjects, setMyOrcaProjects] = useState<OrcaProject[]>([])

  const [mode, setMode] = useState<DialogMode>('new-repo')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [devServerId, setDevServerId] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [visibility, setVisibility] = useState<ProjectVisibility>('private')
  const [selectedProjectId, setSelectedProjectId] = useState('')

  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // BUG-FE-PW-001: existing sidebar repo data, used to warn when
  // repoPath+devServerId already exists there — already lives in the client
  // store, no new RPC needed. Read via getState() (not the
  // useAppStore(selector) hook form) to match this file's existing pattern —
  // the component already re-renders on every field keystroke, so this stays
  // current without a store subscription.
  const existingRepos = useAppStore.getState().repos ?? []

  useEffect(() => {
    if (!open) {
      return
    }
    setDevServersLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<DevServerOption[]>(target, 'devServer.list', null)
      .then((list) => setDevServers(list ?? []))
      .catch(() => setDevServers([]))
      .finally(() => setDevServersLoading(false))
    // Real OrcaProjects the caller belongs to — link-picker candidates (see
    // DialogMode's doc comment for why this must be project.list, not the
    // client-only Project Host Setup projection).
    callRuntimeRpc<OrcaProject[]>(target, 'project.list', null)
      .then((list) => setMyOrcaProjects(list ?? []))
      .catch(() => setMyOrcaProjects([]))
  }, [open])

  function resetForm() {
    setMode('new-repo')
    setName('')
    setDescription('')
    setDevServerId('')
    setRepoPath('')
    setVisibility('private')
    setSelectedProjectId('')
    setError(null)
  }

  function findDuplicateRepo(path: string, targetDevServerId: string) {
    const normalizedPath = path.trim().replace(/\/+$/, '')
    if (!normalizedPath || !targetDevServerId) {
      return undefined
    }
    return existingRepos.find(
      (r) =>
        r.path.replace(/\/+$/, '') === normalizedPath &&
        r.executionHostId === `devServer:${targetDevServerId}`
    )
  }

  const duplicateRepo = mode === 'new-repo' ? findDuplicateRepo(repoPath, devServerId) : undefined

  // OrcaProjects not yet linked into *some other* OrcaProject aren't
  // distinguishable here (that state lives per-container, not on the
  // project itself) — the picker simply lists every OrcaProject the caller
  // belongs to; linking twice into the same container is a harmless
  // idempotent no-op server-side.
  const linkableProjects = myOrcaProjects

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()

    if (mode === 'link') {
      if (!name.trim() || !selectedProjectId) {
        return
      }
      setSubmitting(true)
      setError(null)
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        const project = await callRuntimeRpc<OrcaProject>(target, 'project.create', {
          name: name.trim(),
          description: description.trim() || undefined,
          visibility
        })
        // BUG-FE-PW-002 fix — links an EXISTING Project into the OrcaProject
        // just created. Deliberately does NOT call repo.add/rebindDevServer:
        // a linked Project keeps its own (possibly multi-host) repos as-is.
        await callRuntimeRpc(target, 'orcaProjects.linkSourceProject', {
          orcaProjectId: project.id,
          projectId: selectedProjectId
        })
        onCreated(project)
        onOpenChange(false)
        resetForm()
      } catch (err) {
        setError(describeError(err, 'Failed to create project.'))
      } finally {
        setSubmitting(false)
      }
      return
    }

    if (!name.trim() || !devServerId || !repoPath.trim()) {
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // Why devServerId/repoPath aren't sent here: CreateProjectRequest
      // (project.proto) has neither field — a repo path only makes sense as
      // a follow-up AddRepo call against the new project's id. Both used to
      // be sent here anyway and were silently dropped by the wscompat
      // handler — this project's dev server/repo were never actually set.
      const project = await callRuntimeRpc<OrcaProject>(target, 'project.create', {
        name: name.trim(),
        description: description.trim() || undefined,
        visibility
      })

      // Phase 10 (project.repos.dev_server_id): the repo now carries its own
      // dev-server binding, set directly here — no more separate
      // project.rebindDevServer step before it. The project row already
      // exists past this point — a follow-up failure is "couldn't fully set
      // it up", not "creation failed", so it surfaces as a toast (same
      // reasoning as ProjectSwitcher's onCreated catch) rather than blocking
      // onCreated/closing the dialog.
      await callRuntimeRpc(target, 'repo.add', {
        projectId: project.id,
        url: repoPath.trim(),
        displayName: name.trim(),
        devServerId
      }).catch(() => {
        toast.error(
          'Project created, but the repo could not be added. Add it from Project Settings.'
        )
      })

      onCreated(project)
      onOpenChange(false)
      resetForm()
    } catch (err) {
      setError(describeError(err, 'Failed to create project.'))
    } finally {
      setSubmitting(false)
    }
  }

  const submitDisabled =
    submitting ||
    !name.trim() ||
    (mode === 'new-repo' ? !devServerId || !repoPath.trim() : !selectedProjectId)

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          resetForm()
        }
      }}
    >
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create New Project</DialogTitle>
            <DialogDescription>
              Register an existing repo on a dev server as a new OrcaProject, or link a Project you
              already have.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-3 py-4">
            <div className="grid gap-1.5">
              <Label htmlFor="cp-name">Name</Label>
              <Input
                id="cp-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                autoFocus
              />
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="cp-description">Description (optional)</Label>
              <Input
                id="cp-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>

            <Tabs value={mode} onValueChange={(v) => setMode(v as DialogMode)}>
              <TabsList>
                <TabsTrigger value="new-repo" data-testid="cp-mode-new-repo">
                  New Repo
                </TabsTrigger>
                <TabsTrigger value="link" data-testid="cp-mode-link">
                  Link Existing Project
                </TabsTrigger>
              </TabsList>

              <TabsContent value="new-repo" className="grid gap-3 pt-3">
                <div className="grid gap-1.5">
                  <Label htmlFor="cp-dev-server">Dev Server</Label>
                  <Select value={devServerId} onValueChange={setDevServerId}>
                    <SelectTrigger id="cp-dev-server" data-testid="cp-dev-server-trigger">
                      <SelectValue
                        placeholder={devServersLoading ? 'Loading…' : 'Select a dev server'}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {devServers.map((ds) => (
                        <SelectItem key={ds.id} value={ds.id}>
                          {ds.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {!devServersLoading && devServers.length === 0 ? (
                    <p className="text-xs text-muted-foreground">
                      No dev server paired yet — add one first from the sidebar.
                    </p>
                  ) : null}
                </div>

                <div className="grid gap-1.5">
                  <Label htmlFor="cp-repo-path">Repo Path</Label>
                  <Input
                    id="cp-repo-path"
                    placeholder="/home/user/projects/my-repo"
                    value={repoPath}
                    onChange={(e) => setRepoPath(e.target.value)}
                    required={mode === 'new-repo'}
                  />
                  <p className="text-xs text-muted-foreground">
                    Absolute path on the dev server&apos;s filesystem.
                  </p>
                  {duplicateRepo ? (
                    <p className="text-xs text-amber-600" data-testid="cp-duplicate-repo-warning">
                      This repo is already in your sidebar ({duplicateRepo.displayName}). Creating a
                      new Project here will NOT link to that data — they will stay independent.{' '}
                      {myOrcaProjects.length > 0 ? (
                        <button type="button" className="underline" onClick={() => setMode('link')}>
                          Link an existing Project instead?
                        </button>
                      ) : null}
                    </p>
                  ) : null}
                </div>
              </TabsContent>

              <TabsContent value="link" className="grid gap-1.5 pt-3">
                <Label htmlFor="cp-link-project">Your Project</Label>
                <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
                  <SelectTrigger id="cp-link-project" data-testid="cp-link-project-select">
                    <SelectValue
                      placeholder={
                        linkableProjects.length === 0 ? 'No projects to link' : 'Choose a Project'
                      }
                    />
                  </SelectTrigger>
                  <SelectContent>
                    {linkableProjects.map((p) => (
                      <SelectItem key={p.id} value={p.id}>
                        {p.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  Share a Project you already have (possibly spanning multiple dev servers) with the
                  other members of this OrcaProject.
                </p>
              </TabsContent>
            </Tabs>

            <div className="grid gap-1.5">
              <Label htmlFor="cp-visibility">Visibility</Label>
              <Select
                value={visibility}
                onValueChange={(v) => setVisibility(v as ProjectVisibility)}
              >
                <SelectTrigger id="cp-visibility">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="private">Private</SelectItem>
                  <SelectItem value="team">Team</SelectItem>
                  <SelectItem value="department">Department</SelectItem>
                  <SelectItem value="company">Company</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {error ? <p className="text-sm text-destructive">{error}</p> : null}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitDisabled}>
              {submitting ? 'Creating…' : 'Create Project'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
