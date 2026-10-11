import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'

/**
 * Blocking notice: the folder is already a repo on that dev server. Adding it again would put two
 * entries on one path in the sidebar, so the only action is to open the project that holds it.
 */
export function AddRepoDuplicateNotice({
  projectName,
  onOpen,
  testId = 'add-project-duplicate',
  openTestId = 'add-project-open-existing'
}: {
  projectName: string
  onOpen?: () => void
  testId?: string
  openTestId?: string
}): React.JSX.Element {
  return (
    <div role="alert" data-testid={testId} className="space-y-1.5 text-xs text-destructive">
      <p>
        {projectName
          ? translate(
              'auto.components.sidebar.AddRepoDuplicateNotice.message',
              'This folder is already added in project "{{project}}" on this dev server, so it was not added again.',
              { project: projectName }
            )
          : translate(
              'auto.components.sidebar.AddRepoDuplicateNotice.messageUnknownProject',
              'This folder is already added on this dev server, so it was not added again.'
            )}
      </p>
      {onOpen ? (
        <Button type="button" size="xs" variant="outline" data-testid={openTestId} onClick={onOpen}>
          {translate('auto.components.sidebar.AddRepoDuplicateNotice.open', 'Open that project')}
        </Button>
      ) : null}
    </div>
  )
}
