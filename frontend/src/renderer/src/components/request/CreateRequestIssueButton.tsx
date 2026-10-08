/**
 * CreateRequestIssueButton — CR-REQ-019-06
 *
 * "Create request" next to "Start workspace" on Jira / GitHub issues. Unlike
 * Start workspace it sends the issue into the analysis flow instead of opening
 * a worktree. Hidden for pull requests and when the runtime lacks the flow.
 *
 * @module components/request/CreateRequestIssueButton
 */

import React, { useCallback, useMemo, useState } from 'react'
import { Inbox } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { translate } from '@/i18n/i18n'
import { matchProjectForJiraIssue } from '@/lib/jira-project-matching'
import { useAppStore } from '@/store'
import { CreateRequestDialog } from './CreateRequestDialog'
import { githubItemToCreateParams, jiraIssueToCreateParams } from './create-request-from-issue'
import type { GitHubRepoIdentity } from './create-request-from-issue'
import type { JiraIssue } from '../../../../shared/jira-types'
import type { GitHubWorkItem } from '../../../../shared/types'
import type { OrcaProject } from '../../types/workspace-types'

const T = 'auto.components.request.CreateRequestIssueButton.'

type Props =
  | { issue: JiraIssue; item?: undefined; repoIdentity?: undefined; compact?: boolean }
  | { issue?: undefined; item: GitHubWorkItem; repoIdentity: GitHubRepoIdentity | null; compact?: boolean }

export function CreateRequestIssueButton(props: Props): React.JSX.Element | null {
  const supported = useAppStore((s) => s.requestFlowSupport === 'supported')
  const [open, setOpen] = useState(false)
  const { issue, item, repoIdentity, compact } = props

  const initial = useMemo(() => {
    if (issue) {return jiraIssueToCreateParams(issue, '')}
    if (item && repoIdentity) {return githubItemToCreateParams(item, repoIdentity, '')}
    return null
  }, [issue, item, repoIdentity])

  const pickProjectId = useCallback(
    (projects: OrcaProject[]) => (issue ? (matchProjectForJiraIssue(projects, issue.key, issue.siteId)?.id ?? null) : null),
    [issue]
  )

  if (!supported || !initial) {return null}

  const label = translate(`${T}label`, 'Create request')
  const button = (
    <Button
      type="button"
      variant="outline"
      size={compact ? 'icon-xs' : 'sm'}
      aria-label={label}
      onClick={(event) => {
        event.stopPropagation()
        setOpen(true)
      }}
    >
      <Inbox className="size-3.5" aria-hidden />
      {!compact && label}
    </Button>
  )

  return (
    <>
      <Tooltip>
        <TooltipTrigger asChild>{button}</TooltipTrigger>
        <TooltipContent side="bottom" sideOffset={6}>
          {translate(
            `${T}tooltip`,
            'Send this issue into the analysis flow. Unlike Start workspace, no worktree is opened right away.'
          )}
        </TooltipContent>
      </Tooltip>
      {open && <CreateRequestDialog open onOpenChange={setOpen} initial={initial} pickProjectId={pickProjectId} />}
    </>
  )
}
