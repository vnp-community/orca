// CreateProjectDialog.tsx — "Create New Project" form for ProjectSwitcher (F38 Workspace, TDD-FE-12)
import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget, RuntimeRpcCallError } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/select'

type OrcaProjectListItem = { id: string; name: string; devServerId: string }
type DevServerOption = { id: string; name: string; status: string }

type ProjectVisibility = 'private' | 'team' | 'department' | 'company'

type CreateProjectDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (project: OrcaProjectListItem) => void
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

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [devServerId, setDevServerId] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [visibility, setVisibility] = useState<ProjectVisibility>('private')

  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {return}
    setDevServersLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<DevServerOption[]>(target, 'devServer.list', null)
      .then(list => setDevServers(list ?? []))
      .catch(() => setDevServers([]))
      .finally(() => setDevServersLoading(false))
  }, [open])

  function resetForm() {
    setName('')
    setDescription('')
    setDevServerId('')
    setRepoPath('')
    setVisibility('private')
    setError(null)
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim() || !devServerId || !repoPath.trim()) {return}
    setSubmitting(true)
    setError(null)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const project = await callRuntimeRpc<OrcaProjectListItem>(target, 'project.create', {
        name: name.trim(),
        description: description.trim() || undefined,
        devServerId,
        repoPath: repoPath.trim(),
        visibility,
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

  return (
    <Dialog open={open} onOpenChange={next => { onOpenChange(next); if (!next) {resetForm()} }}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create New Project</DialogTitle>
            <DialogDescription>
              Register an existing repo on a dev server as a new OrcaProject.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-3 py-4">
            <div className="grid gap-1.5">
              <Label htmlFor="cp-name">Name</Label>
              <Input id="cp-name" value={name} onChange={e => setName(e.target.value)} required autoFocus />
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="cp-description">Description (optional)</Label>
              <Input id="cp-description" value={description} onChange={e => setDescription(e.target.value)} />
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="cp-dev-server">Dev Server</Label>
              <Select value={devServerId} onValueChange={setDevServerId}>
                <SelectTrigger id="cp-dev-server" data-testid="cp-dev-server-trigger">
                  <SelectValue placeholder={devServersLoading ? 'Loading…' : 'Select a dev server'} />
                </SelectTrigger>
                <SelectContent>
                  {devServers.map(ds => (
                    <SelectItem key={ds.id} value={ds.id}>{ds.name}</SelectItem>
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
                onChange={e => setRepoPath(e.target.value)}
                required
              />
              <p className="text-xs text-muted-foreground">Absolute path on the dev server&apos;s filesystem.</p>
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="cp-visibility">Visibility</Label>
              <Select value={visibility} onValueChange={v => setVisibility(v as ProjectVisibility)}>
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
            <Button type="submit" disabled={submitting || !name.trim() || !devServerId || !repoPath.trim()}>
              {submitting ? 'Creating…' : 'Create Project'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
