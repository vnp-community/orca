// ServerBootstrapIdleScreen — idle/start screen shown before bootstrap runs (CR-004, TASK-004-C)
import { Server, GitBranch, Key, FolderGit2, Play, AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import type { SshTarget } from '../../../../../shared/ssh-types'

function BootstrapStepPreview({
  icon,
  label
}: {
  icon: React.ReactNode
  label: string
}): React.JSX.Element {
  return (
    <li className="flex items-center gap-2 text-sm text-muted-foreground">
      <span className="flex-shrink-0">{icon}</span>
      {label}
    </li>
  )
}

type BootstrapIdleProps = {
  target: SshTarget
  onStart: () => void
}

export function ServerBootstrapIdleScreen({
  target,
  onStart
}: BootstrapIdleProps): React.JSX.Element {
  // Why: connectionStates is a Map — use .get() not bracket access
  const connectionStates = useAppStore((s) => s.sshConnectionStates)
  const isConnected = connectionStates.get(target.id)?.status === 'connected'

  const repoCount = (target as { repos?: unknown[] }).repos?.length ?? 0

  return (
    <div className="space-y-4">
      {/* Preview of steps bootstrap will perform */}
      <div className="rounded-md border bg-muted/30 p-4">
        <h4 className="mb-3 text-sm font-medium">
          {translate('fleet.bootstrap.whatItDoes', 'Bootstrap will:')}
        </h4>
        <ul className="space-y-2">
          <BootstrapStepPreview
            icon={<Server className="size-4" />}
            label={translate('fleet.bootstrap.stepNode', 'Install Node.js 22+')}
          />
          <BootstrapStepPreview
            icon={<GitBranch className="size-4" />}
            label={translate('fleet.bootstrap.stepGit', 'Install Git 2.35+')}
          />
          <BootstrapStepPreview
            icon={<Key className="size-4" />}
            label={translate('fleet.bootstrap.stepSshKey', 'Setup SSH key for git')}
          />
          <BootstrapStepPreview
            icon={<FolderGit2 className="size-4" />}
            label={
              repoCount > 0
                ? translate(
                    'fleet.bootstrap.stepReposCount',
                    `Clone ${repoCount} repo(s) from fleet config`
                  )
                : translate('fleet.bootstrap.stepRepos', 'Clone project repos')
            }
          />
          <BootstrapStepPreview
            icon={<Play className="size-4" />}
            label={translate(
              'fleet.bootstrap.stepSetup',
              'Run orca.yaml setup scripts'
            )}
          />
        </ul>
      </div>

      {/* Warning banner when server is not connected (no alert.tsx in UI kit) */}
      {!isConnected && (
        <div className="flex items-start gap-3 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm">
          <AlertTriangle className="mt-0.5 size-4 flex-shrink-0 text-yellow-600" />
          <p className="text-yellow-800 dark:text-yellow-300">
            {translate(
              'fleet.bootstrap.connectFirst',
              'Connect to this server before running bootstrap.'
            )}
          </p>
        </div>
      )}

      {/* Start button — disabled when disconnected */}
      <Button onClick={onStart} disabled={!isConnected} className="w-full" size="default">
        {translate('fleet.bootstrap.start', 'Start Bootstrap')}
      </Button>
    </div>
  )
}
