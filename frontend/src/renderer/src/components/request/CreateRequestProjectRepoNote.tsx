import React from 'react'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useRequestProjectRepos } from './use-request-project-repos'

/** Says which repo a Request in the chosen project will run on (the project's first repo). */
export function CreateRequestProjectRepoNote({
  projectId
}: {
  projectId: string
}): React.JSX.Element | null {
  const result = useRequestProjectRepos(projectId)
  const devServers = useAppStore((s) => s.devServers)
  if (result.status === 'idle' || result.status === 'loading' || result.status === 'failed') {
    return null
  }
  const [first] = result.repos
  if (!first) {
    return (
      <p className="text-xs text-destructive" data-testid="create-request-repo-note">
        {translate(
          'auto.components.request.CreateRequestProjectRepoNote.noRepo',
          'This project has no repo yet, so a request in it cannot run. Add a repo to the project first.'
        )}
      </p>
    )
  }
  const host = first.devServerId
    ? (devServers?.find((server) => server.id === first.devServerId)?.name ?? first.devServerId)
    : null
  return (
    <p className="text-xs text-muted-foreground" data-testid="create-request-repo-note">
      {host
        ? translate(
            'auto.components.request.CreateRequestProjectRepoNote.runsOnWithHost',
            'The request runs on repo {{repo}} ({{path}}) on {{host}}.',
            {
              repo: first.displayName,
              path: first.url,
              host
            }
          )
        : translate(
            'auto.components.request.CreateRequestProjectRepoNote.runsOn',
            'The request runs on repo {{repo}} ({{path}}).',
            {
              repo: first.displayName,
              path: first.url
            }
          )}
      {result.repos.length > 1
        ? ` ${translate(
            'auto.components.request.CreateRequestProjectRepoNote.firstOfMany',
            "It is the first of this project's {{count}} repos.",
            { count: result.repos.length }
          )}`
        : ''}
    </p>
  )
}
