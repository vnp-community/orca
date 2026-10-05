// ProjectJiraMappingSection.tsx — General tab's Jira project key + site mapping.
// Lets the composer pre-select this project from a Jira issue key prefix.
// Saves through project.update (absent = no change, "" = clear).
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import {
  callRuntimeRpc,
  getActiveRuntimeTarget,
  RuntimeRpcCallError
} from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { useWorkspace } from '../../context/WorkspaceContext'
import { JIRA_PROJECT_KEY_PATTERN } from '../../lib/jira-project-matching'

// Sentinel for "no site" — an optional Select value that isn't a valid site id.
const NO_SITE_VALUE = '__orca_no_jira_site__'

function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

export function ProjectJiraMappingSection({ projectId }: { projectId: string }) {
  const { project, switchProject } = useWorkspace()
  const sites = useAppStore((s) => s.jiraStatus?.sites) ?? []
  const checkJiraConnection = useAppStore((s) => s.checkJiraConnection)
  const [key, setKey] = useState('')
  const [siteId, setSiteId] = useState(NO_SITE_VALUE)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    void checkJiraConnection?.()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load site list once on mount
  }, [])

  useEffect(() => {
    setKey(project?.jiraProjectKey ?? '')
    setSiteId(project?.jiraSiteId || NO_SITE_VALUE)
  }, [project?.jiraProjectKey, project?.jiraSiteId])

  const currentKey = project?.jiraProjectKey ?? ''
  const currentSite = project?.jiraSiteId || NO_SITE_VALUE
  const keyValid = key === '' || JIRA_PROJECT_KEY_PATTERN.test(key)
  const hasChange = key !== currentKey || siteId !== currentSite

  const handleSave = async (): Promise<void> => {
    if (!hasChange || !keyValid) {
      return
    }
    setSaving(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'project.update', {
        id: projectId,
        jiraProjectKey: key,
        jiraSiteId: siteId === NO_SITE_VALUE ? '' : siteId
      })
      toast.success(
        translate('auto.components.project.ProjectJiraMappingSection.saved', 'Jira mapping updated')
      )
      await switchProject(projectId)
    } catch (err) {
      toast.error(
        describeError(
          err,
          translate(
            'auto.components.project.ProjectJiraMappingSection.saveFailed',
            'Failed to update the Jira mapping.'
          )
        )
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label>
          {translate('auto.components.project.ProjectJiraMappingSection.title', 'Jira mapping')}
        </Label>
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.components.project.ProjectJiraMappingSection.description',
            'Map this project to a Jira project key so starting work from a Jira issue selects it automatically.'
          )}
        </p>
      </div>
      <div className="flex flex-wrap items-start gap-2">
        <div className="space-y-1">
          <Label htmlFor="project-jira-key" className="text-xs">
            {translate(
              'auto.components.project.ProjectJiraMappingSection.keyLabel',
              'Jira project key'
            )}
          </Label>
          <Input
            id="project-jira-key"
            className="w-40"
            value={key}
            maxLength={20}
            aria-invalid={!keyValid}
            onChange={(e) => setKey(e.target.value.toUpperCase())}
            data-testid="project-jira-key-input"
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">
            {translate('auto.components.project.ProjectJiraMappingSection.siteLabel', 'Jira site')}
          </Label>
          <Select value={siteId} onValueChange={setSiteId}>
            <SelectTrigger className="w-64" data-testid="project-jira-site-select">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NO_SITE_VALUE}>
                {translate('auto.components.project.ProjectJiraMappingSection.siteNone', 'No site')}
              </SelectItem>
              {sites.map((site) => (
                <SelectItem key={site.id} value={site.id}>
                  {site.displayName || site.siteUrl}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button
          type="button"
          size="sm"
          className="mt-[22px]"
          disabled={!hasChange || !keyValid || saving}
          onClick={() => void handleSave()}
          data-testid="project-jira-mapping-save"
        >
          {saving
            ? translate('auto.components.project.ProjectJiraMappingSection.saving', 'Saving…')
            : translate('auto.components.project.ProjectJiraMappingSection.save', 'Save')}
        </Button>
      </div>
      {!keyValid ? (
        <p className="text-xs text-destructive" data-testid="project-jira-key-invalid">
          {translate(
            'auto.components.project.ProjectJiraMappingSection.keyInvalid',
            'Use 2-20 characters: capital letters, digits or underscores, starting with a letter.'
          )}
        </p>
      ) : null}
      {sites.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.components.project.ProjectJiraMappingSection.noSites',
            'No connected Jira sites. Connect Jira from the Tasks page first.'
          )}
        </p>
      ) : null}
    </div>
  )
}
