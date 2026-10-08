import { useState } from 'react'
import {
  AlertCircle,
  CloudOff,
  Database,
  FileQuestion,
  GitBranch,
  Loader2,
  Lock,
  PlugZap,
  type LucideIcon
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type { ReviewScreen } from '../review-view-state'

export type ReviewScreenActions = {
  onReindex: () => void
  onRebind: () => void
  onRetry: () => void
  onCloseTab: () => void
  onOpenScopePicker: () => void
}

type ScreenCopy = {
  icon: LucideIcon
  title: string
  body: string
  /** Primary action; at most one so the next step is obvious. */
  action?: { label: string; run: (a: ReviewScreenActions) => void }
}

const t = translate

function copyFor(screen: ReviewScreen): ScreenCopy {
  const k = 'auto.components.reviewMap.shell.screen.'
  switch (screen.id) {
    case 'unsupported':
      return {
        icon: PlugZap,
        title: t(`${k}unsupported.title`, 'Review is not available here'),
        body: t(
          `${k}unsupported.body`,
          'This workspace or backend does not provide code review data.'
        ),
        action: { label: t(`${k}closeTab`, 'Close tab'), run: (a) => a.onCloseTab() }
      }
    case 'disabled':
      return {
        icon: PlugZap,
        title: t(`${k}disabled.title`, 'Code review is turned off'),
        body: t(
          `${k}disabled.body`,
          'An administrator has disabled code intelligence for this workspace.'
        ),
        action: { label: t(`${k}closeTab`, 'Close tab'), run: (a) => a.onCloseTab() }
      }
    case 'scope-error': {
      const map = {
        'invalid-base': [
          'The base branch was not found',
          'Pick another base branch to compare against.'
        ],
        'unborn-head': [
          'This branch has no commits yet',
          'Commit something, then review the changes.'
        ],
        'no-merge-base': [
          'No common ancestor with the base',
          'Pick another base branch or a commit range.'
        ],
        error: ['The comparison could not be computed', screen.message ?? 'Try another base.']
      } as const
      const [title, body] = map[screen.reason]
      return {
        icon: GitBranch,
        title: t(`${k}scope.${screen.reason}.title`, title),
        body: t(`${k}scope.${screen.reason}.body`, body),
        action: {
          label: t(`${k}chooseScope`, 'Choose what to compare'),
          run: (a) => a.onOpenScopePicker()
        }
      }
    }
    case 'no-binding':
      return {
        icon: PlugZap,
        title: t(`${k}noBinding.title`, 'This worktree is not connected to a code index'),
        body: t(`${k}noBinding.body`, 'Connect the repository to its dev server, then try again.'),
        action: { label: t(`${k}noBinding.action`, 'Try reconnecting'), run: (a) => a.onRebind() }
      }
    case 'forbidden':
      return {
        icon: Lock,
        title: t(`${k}forbidden.title`, 'You do not have access to this review'),
        body: t(`${k}forbidden.body`, 'Ask a workspace admin for read access to code intelligence.')
      }
    case 'loading-initial':
      return { icon: Loader2, title: t(`${k}loading.title`, 'Loading changes'), body: '' }
    case 'tool-unavailable':
      return {
        icon: Database,
        title: t(`${k}toolUnavailable.title`, 'No code index tool is installed'),
        body: t(
          `${k}toolUnavailable.body`,
          'Install GitNexus or CodeGraph on the machine that holds this repository.'
        ),
        action: { label: t(`${k}retry`, 'Try again'), run: (a) => a.onRetry() }
      }
    case 'repo-not-registered':
      return {
        icon: Database,
        title: t(
          `${k}notRegistered.title`,
          'This repository is not registered with the index tool'
        ),
        body: t(`${k}notRegistered.body`, 'Build the index to register it.'),
        action: { label: t(`${k}buildIndex`, 'Build index'), run: (a) => a.onReindex() }
      }
    case 'path-not-allowed':
      return {
        icon: AlertCircle,
        title: t(`${k}pathNotAllowed.title`, 'A file path was rejected'),
        body: t(`${k}pathNotAllowed.body`, 'The backend refused a path in this change set.'),
        action: { label: t(`${k}retry`, 'Try again'), run: (a) => a.onRetry() }
      }
    case 'index-missing':
      return {
        icon: Database,
        title: t(`${k}indexMissing.title`, 'There is no index for this repository yet'),
        body: t(`${k}indexMissing.body`, 'Review needs an index to map symbols and impact.'),
        action: { label: t(`${k}buildIndex`, 'Build index'), run: (a) => a.onReindex() }
      }
    case 'index-building':
      return {
        icon: Loader2,
        title: t(`${k}indexBuilding.title`, 'Building the index'),
        body:
          screen.percent === null
            ? t(`${k}indexBuilding.unknown`, 'This can take a few minutes.')
            : t(`${k}indexBuilding.percent`, '{{percent}}% done', {
                percent: Math.round(screen.percent)
              })
      }
    case 'offline':
      return {
        icon: CloudOff,
        title: t(`${k}offline.title`, 'Cannot reach the code index'),
        body: t(
          `${k}offline.body`,
          'The connection is down. This view retries when it comes back.'
        ),
        action: { label: t(`${k}retry`, 'Try again'), run: (a) => a.onRetry() }
      }
    case 'error':
      return {
        icon: AlertCircle,
        title: t(`${k}error.title`, 'The review could not be loaded'),
        body: screen.error.message,
        action: { label: t(`${k}retry`, 'Try again'), run: (a) => a.onRetry() }
      }
    case 'no-changes':
      return {
        icon: FileQuestion,
        title:
          screen.reason === 'unborn-head'
            ? t(`${k}noChanges.unborn`, 'This branch has no commits yet')
            : t(`${k}noChanges.title`, 'No changes in this scope'),
        body: t(`${k}noChanges.body`, 'Choose a different base or range to see changes.'),
        action: {
          label: t(`${k}chooseScope`, 'Choose what to compare'),
          run: (a) => a.onOpenScopePicker()
        }
      }
  }
}

type Props = { screen: ReviewScreen; actions: ReviewScreenActions }

export function ReviewViewStateScreen({ screen, actions }: Props): React.JSX.Element {
  const copy = copyFor(screen)
  const Icon = copy.icon
  const [copied, setCopied] = useState(false)
  // Raw error text is copyable for support, but never for path-not-allowed (could echo a path).
  const detail = screen.id === 'error' || screen.id === 'no-binding' ? screen.error.message : null
  const spin = screen.id === 'loading-initial' || screen.id === 'index-building'
  return (
    <div
      role={screen.id === 'loading-initial' || screen.id === 'index-building' ? 'status' : 'alert'}
      data-screen={screen.id}
      className="mx-auto flex max-w-md flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
    >
      <Icon
        className={`size-8 text-muted-foreground ${spin ? 'animate-spin motion-reduce:animate-none' : ''}`}
        aria-hidden
      />
      <h2 className="text-base font-medium">{copy.title}</h2>
      {copy.body && screen.id !== 'error' ? (
        <p className="text-sm text-muted-foreground">{copy.body}</p>
      ) : null}
      {detail ? (
        <div className="w-full space-y-1">
          <pre className="max-h-24 overflow-auto rounded-md bg-muted p-2 text-left text-xs whitespace-pre-wrap">
            {detail}
          </pre>
          <Button
            type="button"
            size="xs"
            variant="ghost"
            onClick={() => {
              void navigator.clipboard?.writeText(detail).then(
                () => setCopied(true),
                () => undefined
              )
            }}
          >
            {copied
              ? translate('auto.components.reviewMap.shell.screen.copied', 'Copied')
              : translate('auto.components.reviewMap.shell.screen.copyDetails', 'Copy details')}
          </Button>
        </div>
      ) : null}
      {copy.action ? (
        <Button type="button" size="sm" onClick={() => copy.action!.run(actions)}>
          {copy.action.label}
        </Button>
      ) : null}
    </div>
  )
}
