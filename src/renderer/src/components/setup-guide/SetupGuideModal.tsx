import { useCallback, useEffect, useMemo, useState, type JSX } from 'react'
import { EyeOff } from 'lucide-react'
import {
  FEATURE_WALL_SETUP_STEP_IDS,
  getFirstIncompleteFeatureWallSetupStepId,
  getFeatureWallSetupSteps
} from '../../../../shared/feature-wall-setup-steps'
import type { FeatureWallSetupStepId } from '../../../../shared/feature-wall-setup-steps'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { FeatureWallSetupChecklist } from '../feature-wall/FeatureWallSetupChecklist'
import { SetupGuideProgressRing } from './SetupGuideProgressRing'
import { useSetupGuideProgress } from './use-setup-guide-progress'
import { useSetupGuideOpenCloseTelemetry } from './use-setup-guide-telemetry'
import { translate } from '@/i18n/i18n'
import { useConnectedDevServers } from '@/store/slices/dev-servers'
import { useServerChecklist } from '@/store/slices/onboarding-checklist'
import type { DevServer } from '../../../../shared/dev-server-types'
import type { PerServerChecklistState } from '../../../../shared/dev-server-types'

export default function SetupGuideModal(): JSX.Element | null {
  const activeModal = useAppStore((s) => s.activeModal)
  const modalData = useAppStore((s) => s.modalData)
  const closeModal = useAppStore((s) => s.closeModal)
  const setSetupGuideSidebarDismissed = useAppStore((s) => s.setSetupGuideSidebarDismissed)
  const isOpen = activeModal === 'setup-guide'
  const setupSteps = useMemo(() => getFeatureWallSetupSteps(), [])
  const [userSelectedStep, setUserSelectedStep] = useState(false)
  const [orchestrationSkillInstalled, setOrchestrationSkillInstalled] = useState(false)
  const [browserUseSkillInstalled, setBrowserUseSkillInstalled] = useState(false)
  const progress = useSetupGuideProgress(
    isOpen,
    orchestrationSkillInstalled,
    browserUseSkillInstalled
  )
  const [activeStepId, setActiveStepId] = useState<FeatureWallSetupStepId>(() =>
    getFirstIncompleteFeatureWallSetupStepId(progress.stepDone)
  )
  const requestedStepId = isFeatureWallSetupStepId(modalData.setupStepId)
    ? modalData.setupStepId
    : null
  const telemetrySource =
    typeof modalData.setupGuideSource === 'string'
      ? modalData.setupGuideSource
      : typeof modalData.telemetrySource === 'string'
        ? modalData.telemetrySource
        : 'unknown'
  const activeStep = setupSteps.find((step) => step.id === activeStepId) ?? setupSteps[0] ?? null

  useSetupGuideOpenCloseTelemetry({
    isOpen,
    source: telemetrySource,
    progress,
    activeStepId: activeStep?.id ?? null
  })

  useEffect(() => {
    if (!isOpen) {
      setUserSelectedStep(false)
      return
    }
    if (requestedStepId === null) {
      return
    }
    setUserSelectedStep(false)
    setActiveStepId(requestedStepId)
  }, [isOpen, requestedStepId])

  useEffect(() => {
    if (!isOpen || userSelectedStep || requestedStepId !== null) {
      return
    }
    setActiveStepId(getFirstIncompleteFeatureWallSetupStepId(progress.stepDone))
  }, [isOpen, progress.stepDone, requestedStepId, userSelectedStep])

  useEffect(() => {
    if (
      !isOpen ||
      userSelectedStep ||
      requestedStepId === null ||
      activeStep?.id !== requestedStepId ||
      !progress.stepDone[activeStep.id]
    ) {
      return
    }
    const nextUnfinishedCoreStepId = getFirstIncompleteFeatureWallSetupStepId(progress.stepDone)
    if (nextUnfinishedCoreStepId !== activeStep.id) {
      setActiveStepId(nextUnfinishedCoreStepId)
    }
  }, [activeStep, isOpen, progress.stepDone, requestedStepId, userSelectedStep])

  const handleSelectStep = (id: FeatureWallSetupStepId): void => {
    setUserSelectedStep(true)
    setActiveStepId(id)
  }

  const handleOpenChange = (open: boolean): void => {
    if (!open) {
      closeModal()
    }
  }

  const handleHideFromSidebar = useCallback((): void => {
    setSetupGuideSidebarDismissed(true)
  }, [setSetupGuideSidebarDismissed])

  if (!isOpen) {
    return null
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogContent
        className="grid h-[min(780px,calc(100vh-2rem))] w-[min(1080px,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)] gap-0 p-0 sm:max-w-none"
        tabIndex={-1}
      >
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label={translate(
                'auto.components.setup.guide.SetupGuideModal.f3b5ffb2a6',
                'Hide checklist from sidebar'
              )}
              onClick={handleHideFromSidebar}
              className="absolute right-10 top-3.5 text-muted-foreground"
            >
              <EyeOff className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top" sideOffset={4}>
            {translate(
              'auto.components.setup.guide.SetupGuideModal.28cf59fcb4',
              'This will hide the checklist from the sidebar'
            )}
          </TooltipContent>
        </Tooltip>
        <DialogHeader className="gap-1 border-b border-border px-7 py-4">
          <div className="flex items-center gap-2">
            <DialogTitle className="text-lg">
              {translate(
                'auto.components.setup.guide.SetupGuideModal.48a9e5ef2d',
                'Getting started'
              )}
            </DialogTitle>
            <SetupGuideProgressRing
              done={progress.coreDoneCount}
              total={progress.coreTotal}
              className="text-green-600 dark:text-green-300"
              sizeClassName="size-5"
            />
          </div>
          <DialogDescription className="text-sm text-muted-foreground">
            {translate(
              'auto.components.setup.guide.SetupGuideModal.3598a3ca0c',
              'Finish the core workflows that make Orca useful for parallel agent work.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-hidden px-7 py-6">
          <FeatureWallSetupChecklist
            activeStep={activeStep}
            progress={progress}
            onSelectStep={handleSelectStep}
            onOrchestrationSkillInstalledChange={setOrchestrationSkillInstalled}
            onBrowserUseSkillInstalledChange={setBrowserUseSkillInstalled}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}

function isFeatureWallSetupStepId(value: unknown): value is FeatureWallSetupStepId {
  return (
    typeof value === 'string' &&
    FEATURE_WALL_SETUP_STEP_IDS.includes(value as FeatureWallSetupStepId)
  )
}

// ─── Server checklist section ──────────────────────────────────────────────────

const SERVER_CHECKLIST_ITEMS: Array<{ key: keyof PerServerChecklistState; label: string }> = [
  { key: 'addedRepo', label: 'Add a repository' },
  { key: 'ranFirstAgent', label: 'Run first agent' },
  { key: 'reviewedDiff', label: 'Review a diff' },
  { key: 'openedPr', label: 'Open a pull request' },
]

function ServerChecklistSection({
  devServer,
  checklist,
}: {
  devServer: DevServer
  checklist: PerServerChecklistState
}): JSX.Element {
  return (
    <div className="mt-4 rounded-lg border border-border p-4" data-devserver-id={devServer.id}>
      <p className="mb-2 text-sm font-semibold">{devServer.name}</p>
      <ul className="space-y-1">
        {SERVER_CHECKLIST_ITEMS.map(({ key, label }) => (
          <li key={key} className="flex items-center gap-2 text-sm">
            <span
              className={`size-4 rounded-full border-2 ${
                checklist[key] ? 'border-green-500 bg-green-500' : 'border-muted-foreground'
              }`}
              aria-hidden
            />
            <span className={checklist[key] ? 'line-through text-muted-foreground' : ''}>
              {label}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function ServerChecklistSectionWrapper({ devServer }: { devServer: DevServer }): JSX.Element {
  const checklist = useServerChecklist(devServer.id)
  return <ServerChecklistSection devServer={devServer} checklist={checklist} />
}

// ─── Overall progress bar ──────────────────────────────────────────────────────

export function OverallProgressBar({
  completedCount,
  totalCount,
}: {
  completedCount: number
  totalCount: number
}): JSX.Element {
  const percent = totalCount === 0 ? 0 : Math.round((completedCount / totalCount) * 100)
  return (
    <div className="mt-4 space-y-1">
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>Overall progress</span>
        <span>
          {completedCount}/{totalCount} ({percent}%)
        </span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <div
          className="h-full bg-green-500 transition-all"
          style={{ width: `${percent}%` }}
          role="progressbar"
          aria-valuenow={percent}
          aria-valuemin={0}
          aria-valuemax={100}
        />
      </div>
    </div>
  )
}

// ─── Per-server panel ─────────────────────────────────────────────────────────

/** Renders per-server checklist sections + overall progress bar.
 *  Exported so it can be embedded anywhere in the setup guide flow. */
export function PerServerChecklistPanel(): JSX.Element {
  const connectedServers = useConnectedDevServers()
  const checklistState = useAppStore((s) => s.checklistState)

  if (connectedServers.length === 0) {
    return (
      <p className="mt-4 text-sm text-muted-foreground" id="connect-dev-server">
        Connect a dev server to see per-server checklist.
      </p>
    )
  }

  const GLOBAL_ITEMS = 3 // choseAgent, addedRepo, ranFirstAgent
  const PER_SERVER_ITEMS = SERVER_CHECKLIST_ITEMS.length
  const totalCount = GLOBAL_ITEMS + PER_SERVER_ITEMS * connectedServers.length

  let completedCount = 0
  if (checklistState.choseAgent) completedCount++
  if (checklistState.addedRepo) completedCount++
  if (checklistState.ranFirstAgent) completedCount++

  for (const ds of connectedServers) {
    const cl = checklistState.perServer[ds.id] ?? {}
    for (const { key } of SERVER_CHECKLIST_ITEMS) {
      if (cl[key]) completedCount++
    }
  }

  return (
    <>
      {connectedServers.map((ds) => (
        <ServerChecklistSectionWrapper key={ds.id} devServer={ds} />
      ))}
      <OverallProgressBar completedCount={completedCount} totalCount={totalCount} />
    </>
  )
}
