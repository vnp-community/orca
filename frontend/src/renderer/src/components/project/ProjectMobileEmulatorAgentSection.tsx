// ProjectMobileEmulatorAgentSection.tsx — General tab's Mobile Emulator
// Agent picker (CR-DS-009 / TASK-EMU-012b). Mirrors
// ProjectDevServerSection.tsx's structure, but:
//   - lists only DevServers with kind 'mobile-emulator' (devServer.list's
//     optional {kind} filter — see DevServerListFilter's doc comment for
//     why it's currently a no-op against the real backend-go channel)
//   - saves through project.update's plain mobileEmulatorAgentId field, NOT
//     project.rebindDevServer (that RPC — and its active-execution guard —
//     is devServerId-specific; TASK-EMU-008 deliberately did not extend it)
//   - allows clearing back to "Not set" since the binding is optional
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

type MobileEmulatorAgentOption = { id: string; name: string; status: string }

// Sentinel for "no Mobile Emulator Agent bound" — mirrors
// MobileEmulatorSettingsPane.tsx's AUTOMATIC_DEVICE_VALUE pattern for an
// optional Select value that isn't itself a valid id.
const NOT_SET_VALUE = '__orca_no_mobile_emulator_agent__'

// Same FORBIDDEN/UNAUTHENTICATED-message pattern as
// ProjectDevServerSection.tsx/MemberManager.tsx/CreateProjectDialog.tsx.
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

export function ProjectMobileEmulatorAgentSection({ projectId }: { projectId: string }) {
  const { project, switchProject } = useWorkspace()
  const [agents, setAgents] = useState<MobileEmulatorAgentOption[]>([])
  const [agentsLoading, setAgentsLoading] = useState(false)
  const [selectedAgentId, setSelectedAgentId] = useState(NOT_SET_VALUE)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setAgentsLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<MobileEmulatorAgentOption[]>(target, 'devServer.list', {
      kind: 'mobile-emulator'
    })
      // Defensive Array.isArray check: a test/mock RPC layer (or a future
      // backend response shape change) resolving with a non-array truthy
      // value would otherwise crash this component's .map render below —
      // ?? [] alone only guards the null/undefined case.
      .then((list) => setAgents(Array.isArray(list) ? list : []))
      .catch(() => setAgents([]))
      .finally(() => setAgentsLoading(false))
  }, [])

  // Why sync from `project.mobileEmulatorAgentId`, not init-once state: see
  // ProjectDevServerSection.tsx's identical comment — switchProject
  // (below, after a successful save) re-fetches the project, and the
  // picker should reflect whatever is actually persisted.
  useEffect(() => {
    setSelectedAgentId(project?.mobileEmulatorAgentId || NOT_SET_VALUE)
  }, [project?.mobileEmulatorAgentId])

  const currentValue = project?.mobileEmulatorAgentId || NOT_SET_VALUE
  const hasChange = selectedAgentId !== currentValue

  const handleSave = async (): Promise<void> => {
    if (!hasChange) {
      return
    }
    setSaving(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'project.update', {
        id: projectId,
        mobileEmulatorAgentId: selectedAgentId === NOT_SET_VALUE ? '' : selectedAgentId
      })
      toast.success('Mobile Emulator Agent updated')
      await switchProject(projectId)
    } catch (err) {
      toast.error(describeError(err, 'Failed to update the Mobile Emulator Agent.'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label>Mobile Emulator Agent</Label>
        <p className="text-xs text-muted-foreground">
          The Mobile Emulator Agent this project&apos;s emulator pane controls — independent from
          the dev server above; optional, and can run on a different machine (e.g. your laptop
          with Android Studio/Xcode) than the dev server running this project&apos;s code.
        </p>
      </div>
      <div className="flex items-end gap-2">
        <Select value={selectedAgentId} onValueChange={setSelectedAgentId}>
          <SelectTrigger className="w-64" data-testid="project-mobile-emulator-agent-select">
            <SelectValue placeholder={agentsLoading ? 'Loading…' : 'Select an agent'} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={NOT_SET_VALUE}>Not set</SelectItem>
            {agents.map((agent) => (
              <SelectItem key={agent.id} value={agent.id}>
                {agent.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          type="button"
          size="sm"
          disabled={!hasChange || saving}
          onClick={() => void handleSave()}
          data-testid="project-mobile-emulator-agent-save"
        >
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </div>
      {!agentsLoading && agents.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          No Mobile Emulator Agents available yet — add one from Settings → Mobile Emulator first.
        </p>
      ) : null}
    </div>
  )
}
