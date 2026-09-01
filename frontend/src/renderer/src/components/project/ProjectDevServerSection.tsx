// ProjectDevServerSection.tsx — General tab's dev-server rebind form.
// project.rebindDevServer already existed server-side (with its
// active-execution guard) and was only ever called from CreateProjectDialog
// at creation time — a project created without picking one (or one whose
// dev server later needs to change) had no way to fix that afterward; the
// dialog's own error toast even said "Set it from Project Settings," a
// promise this component finally keeps.
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '../ui/button'
import { Label } from '../ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import {
  callRuntimeRpc,
  getActiveRuntimeTarget,
  RuntimeRpcCallError
} from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { useWorkspace } from '../../context/WorkspaceContext'

type DevServerOption = { id: string; name: string; status: string }

// Same FORBIDDEN/UNAUTHENTICATED-message pattern as MemberManager.tsx/CreateProjectDialog.tsx.
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

export function ProjectDevServerSection({ projectId }: { projectId: string }) {
  const { project, switchProject } = useWorkspace()
  const [devServers, setDevServers] = useState<DevServerOption[]>([])
  const [devServersLoading, setDevServersLoading] = useState(false)
  const [selectedDevServerId, setSelectedDevServerId] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setDevServersLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<DevServerOption[]>(target, 'devServer.list', null)
      .then((list) => setDevServers(list ?? []))
      .catch(() => setDevServers([]))
      .finally(() => setDevServersLoading(false))
  }, [])

  // Why sync from `project.devServerId` instead of initializing state once:
  // switchProject (called after a successful save, below) re-fetches the
  // project, and the picker should reflect whatever is actually persisted —
  // including on first mount, and after another tab/session changes it.
  useEffect(() => {
    setSelectedDevServerId(project?.devServerId ?? '')
  }, [project?.devServerId])

  const hasChange =
    selectedDevServerId !== '' && selectedDevServerId !== (project?.devServerId ?? '')

  const handleSave = async (): Promise<void> => {
    if (!hasChange) {
      return
    }
    setSaving(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'project.rebindDevServer', {
        projectId,
        newDevServerId: selectedDevServerId
      })
      toast.success('Dev server updated')
      await switchProject(projectId)
    } catch (err) {
      toast.error(describeError(err, 'Failed to update the dev server.'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label>Dev server</Label>
        <p className="text-xs text-muted-foreground">
          The dev server this project&apos;s repos and worktrees run on. Changing it is blocked
          while a workflow or task is actively running against this project.
        </p>
      </div>
      <div className="flex items-end gap-2">
        <Select value={selectedDevServerId} onValueChange={setSelectedDevServerId}>
          <SelectTrigger className="w-64" data-testid="project-dev-server-select">
            <SelectValue placeholder={devServersLoading ? 'Loading…' : 'Select a dev server'} />
          </SelectTrigger>
          <SelectContent>
            {devServers.map((ds) => (
              <SelectItem key={ds.id} value={ds.id}>
                {ds.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          type="button"
          size="sm"
          disabled={!hasChange || saving}
          onClick={() => void handleSave()}
          data-testid="project-dev-server-save"
        >
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </div>
      {!devServersLoading && devServers.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          No dev servers available yet — add one from Settings first.
        </p>
      ) : null}
    </div>
  )
}
