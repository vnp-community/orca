// ProjectRequestExecutionSection.tsx — General tab: how a project's requests run once their plan is approved.
// Saves through request.engineSettingsSet, which takes the whole set, so only an administrator can change it.
import { useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Button } from '../ui/button'
import { Label } from '../ui/label'
import { Skeleton } from '../ui/skeleton'
import { SettingsSegmentedControl } from '../settings/SettingsFormControls'
import { useProjectEngineSettings } from '../../hooks/useProjectEngineSettings'
import { useAppStore } from '../../store'
import type { ExecutionModeValue } from '../../../../shared/request-run-all'

const T = 'auto.components.project.ProjectRequestExecutionSection.'

export function ProjectRequestExecutionSection({ projectId }: { projectId: string }) {
  const { status, settings, saving, reload, saveExecutionMode } =
    useProjectEngineSettings(projectId)
  const isAdmin = useAppStore((s) => s.currentUser?.role === 'admin')
  // Why: only the unsaved choice is state; what is shown otherwise is always what the backend last answered.
  const [draft, setDraft] = useState<ExecutionModeValue | null>(null)
  // Why: the client role can differ from the backend's admin check; a refused save turns that project read-only.
  const [refusedFor, setRefusedFor] = useState<string | null>(null)

  if (status === 'unsupported') {
    return null
  }

  const heading = (
    <div className="space-y-1.5">
      <Label>{translate(`${T}title`, 'Request execution')}</Label>
      <p className="text-xs text-muted-foreground">
        {translate(
          `${T}description`,
          'Applies to requests that start running from now on. Requests already running keep their mode.'
        )}
      </p>
    </div>
  )

  if (status === 'loading' && !settings) {
    return (
      <div className="space-y-3" data-testid="project-execution-loading" aria-busy>
        {heading}
        <Skeleton className="h-8 w-80" />
      </div>
    )
  }

  if (!settings) {
    return (
      <div className="space-y-3" data-testid="project-execution-error">
        {heading}
        <p role="alert" className="text-xs text-destructive">
          {status === 'forbidden'
            ? translate(`${T}noAccess`, "You don't have permission to see these settings.")
            : translate(`${T}loadFailed`, "Couldn't load the execution settings.")}
        </p>
        {status !== 'forbidden' && (
          <Button type="button" size="sm" variant="outline" onClick={() => void reload()}>
            {translate(`${T}retry`, 'Retry')}
          </Button>
        )}
      </div>
    )
  }

  const mode = draft ?? settings.executionMode
  const canEdit = isAdmin && refusedFor !== projectId
  const changed = mode !== settings.executionMode

  const save = async (): Promise<void> => {
    const outcome = await saveExecutionMode(mode)
    if (outcome === 'saved' || outcome === 'conflict') {
      setDraft(null)
    }
    if (outcome === 'saved') {
      toast.success(translate(`${T}saved`, 'Request execution updated'))
    } else if (outcome === 'conflict') {
      toast.warning(
        translate(
          `${T}conflict`,
          'Someone else changed these settings. The latest values were loaded.'
        )
      )
    } else if (outcome === 'forbidden') {
      setRefusedFor(projectId)
      toast.error(translate(`${T}adminOnly`, 'Only an administrator can change this.'))
    } else {
      toast.error(translate(`${T}saveFailed`, "Couldn't save the execution settings."))
    }
  }

  return (
    <div className="space-y-3" data-testid="project-execution-section">
      {heading}
      <SettingsSegmentedControl
        value={mode}
        onChange={setDraft}
        ariaLabel={translate(`${T}title`, 'Request execution')}
        options={[
          {
            value: 'step_by_step',
            label: translate(`${T}stepByStep`, 'Ask before each step'),
            disabled: !canEdit || saving
          },
          {
            value: 'continuous',
            label: translate(`${T}continuous`, 'Run all steps after one approval'),
            disabled: !canEdit || saving
          }
        ]}
      />
      {mode === 'continuous' && (
        <p className="text-xs text-muted-foreground" data-testid="project-execution-access-note">
          {translate(
            `${T}accessNote`,
            'Orca will start each step with the access of the person who approves the plan.'
          )}
        </p>
      )}
      {canEdit ? (
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            size="sm"
            disabled={!changed || saving}
            onClick={() => void save()}
            data-testid="project-execution-save"
          >
            {saving ? translate(`${T}saving`, 'Saving…') : translate(`${T}save`, 'Save')}
          </Button>
          {settings.version === 0 && (
            <p className="text-xs text-muted-foreground" data-testid="project-execution-first-save">
              {translate(
                `${T}firstSave`,
                "Saving also stores this project's current engine settings."
              )}
            </p>
          )}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground" data-testid="project-execution-read-only">
          {translate(`${T}adminOnly`, 'Only an administrator can change this.')}
        </p>
      )}
    </div>
  )
}
