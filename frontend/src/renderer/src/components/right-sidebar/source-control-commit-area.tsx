/* eslint-disable max-lines -- Why: straight verbatim extraction of the
   CommitArea component out of SourceControl.tsx's pre-existing, already
   grandfathered max-lines disable (TASK-BIGFILE-021, BUG-FE-BIGFILE-004).
   The component's composer + split-button + 4 recovery-notice branches were
   already over budget before the move; no lines were added beyond the new
   import/export scaffolding. Needs its own internal split, tracked in
   SOLUTION-FE-BIGFILE-004, not deferred silently. */
// `CommitArea`: the commit-message composer + primary/dropdown action split
// button + failure/recovery notices at the bottom of the Source Control
// panel. Extracted verbatim from SourceControl.tsx (TASK-BIGFILE-021).
import React, { useMemo } from 'react'
import {
  ChevronDown,
  CloudUpload,
  Check,
  Loader2,
  RefreshCw,
  Sparkles,
  Square,
  ArrowDownUp,
  ArrowUp,
  GitPullRequestArrow,
  Plus
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import type { PrimaryAction, RemoteOpKind } from './source-control-primary-action'
import type { DropdownActionKind, DropdownEntry } from './source-control-dropdown-items'
import { isCommitMessageFieldDisabled } from './source-control-commit-eligibility'
import { getCommitMessageTextareaRows } from './source-control-commit-message-rows'
import { hasExpandedCommitFailureDetails, summarizeCommitFailure } from './commit-failure-summary'
import {
  isPullPolicyRemoteActionError,
  PullPolicyRemoteActionNotice
} from './source-control-pull-policy-error-notice'
import { translate } from '@/i18n/i18n'
import {
  getSourceControlRecoveryFailureKindLabel,
  type SourceControlPushRecovery
} from './source-control-push-recovery'
import { SourceControlRecoveryNotice } from './source-control-recovery-notice'
import type {
  SourceControlActionRecipe,
  SourceControlLaunchActionId
} from '../../../../shared/source-control-ai-actions'
import type { SourceControlAiWriteTarget } from '../../../../shared/source-control-ai-recipe-save'

export type CreatePrIntentTone = 'muted' | 'destructive'
export type CreatePrIntentNotice = {
  message: string
  tone: CreatePrIntentTone
  action?: 'settings'
}

// Why: directional signifiers ahead of each primary action label. Commit
// (✓) is affirmative; Push (↑) points in the direction data flows; Sync
// (↕) is bidirectional; Publish gets a cloud-up to distinguish the
// first-time publish from a subsequent push. Pull is intentionally
// icon-less — the down-arrow read as a download/save affordance and was
// removed. Keeping the mapping outside the render function avoids
// reallocating it on every render.
const PRIMARY_ICONS: Partial<
  Record<
    PrimaryAction['kind'],
    React.ComponentType<{ className?: string; 'aria-hidden'?: boolean | 'true' | 'false' }>
  >
> = {
  commit: Check,
  stage: Plus,
  push: ArrowUp,
  sync: ArrowDownUp,
  publish: CloudUpload,
  create_pr_intent: GitPullRequestArrow,
  create_pr: GitPullRequestArrow
}

type CommitAreaProps = {
  worktreeId: string | null
  groupId: string | null
  connectionId?: string | null
  repoId?: string | null
  launchPlatform?: NodeJS.Platform
  commitMessage: string
  commitError: string | null
  commitFailureRecoveryPrompt: string | null
  pushRecovery: SourceControlPushRecovery | null
  remoteActionError: string | null
  createPrIntentNotice?: CreatePrIntentNotice | null
  isCommitting: boolean
  isFixingCommitFailureWithAI: boolean
  isFixingPushFailureWithAI: boolean
  isCreatingPr?: boolean
  isCreatePrIntentInFlight?: boolean
  showComposer?: boolean
  sourceControlAiActionsVisible: boolean
  aiEnabled: boolean
  aiAgentConfigured: boolean
  isGenerating: boolean
  generateError: string | null
  stagedCount: number
  hasPartiallyStagedChanges: boolean
  hasUnresolvedConflicts: boolean
  isRemoteOperationActive: boolean
  inFlightRemoteOpKind: RemoteOpKind | null
  primaryAction: PrimaryAction
  dropdownItems: DropdownEntry[]
  fixCommitFailureRecipe?: SourceControlActionRecipe
  fixPushFailureRecipe?: SourceControlActionRecipe
  onCommitMessageChange: (message: string) => void
  onGenerate: () => void
  onCancelGenerate: () => void
  onSaveLaunchActionDefault?: (
    target: SourceControlAiWriteTarget,
    actionId: SourceControlLaunchActionId,
    recipe: SourceControlActionRecipe
  ) => void | Promise<void>
  onOpenSourceControlAiSettings?: () => void
  onFixCommitFailureWithAI: (promptOverride?: string) => Promise<boolean> | boolean
  onFixPushFailureWithAI: (promptOverride?: string) => Promise<boolean> | boolean
  onPrimaryAction: () => void
  onDropdownAction: (kind: DropdownActionKind) => void
}

export function CommitArea({
  worktreeId,
  groupId,
  connectionId,
  repoId,
  launchPlatform,
  commitMessage,
  commitError,
  commitFailureRecoveryPrompt,
  pushRecovery,
  remoteActionError,
  createPrIntentNotice,
  isCommitting,
  isFixingCommitFailureWithAI,
  isFixingPushFailureWithAI,
  isCreatingPr = false,
  isCreatePrIntentInFlight = false,
  showComposer = true,
  sourceControlAiActionsVisible,
  aiEnabled,
  aiAgentConfigured,
  isGenerating,
  generateError,
  stagedCount,
  hasPartiallyStagedChanges,
  hasUnresolvedConflicts,
  isRemoteOperationActive,
  inFlightRemoteOpKind,
  primaryAction,
  dropdownItems,
  fixCommitFailureRecipe,
  fixPushFailureRecipe,
  onCommitMessageChange,
  onGenerate,
  onCancelGenerate,
  onSaveLaunchActionDefault,
  onOpenSourceControlAiSettings,
  onFixCommitFailureWithAI,
  onFixPushFailureWithAI,
  onPrimaryAction,
  onDropdownAction
}: CommitAreaProps): React.JSX.Element {
  // Why: cap at 12 rows so a pasted multi-page commit message doesn't push
  // the Commit button off-screen. The textarea keeps `resize-none` (matching
  // the existing style) — the browser scrolls internally past 12 rows.
  const rows = getCommitMessageTextareaRows(commitMessage)
  // Why: only spin the primary when its label matches what's actually
  // running. The commit-area resolver overrides the primary kind to mirror
  // the in-flight op (e.g. user picks Sync from the dropdown → primary
  // becomes "Sync"), so the equality check spins the button for any primary-
  // eligible remote op the user triggered. Background ops the primary
  // doesn't show (Fetch) leave primaryAction.kind unchanged and the
  // mismatch keeps the spinner off — the disabled state alone is enough
  // signal there. Commit still spins on isCommitting because that path
  // doesn't go through inFlightRemoteOpKind.
  const primaryHostsRemoteOperation =
    primaryAction.kind === inFlightRemoteOpKind ||
    (primaryAction.kind === 'push' && inFlightRemoteOpKind === 'force_push')
  const showSpinner =
    primaryAction.kind === 'create_pr' || primaryAction.kind === 'create_pr_intent'
      ? isCreatingPr
      : primaryAction.kind === 'commit'
        ? isCommitting
        : isRemoteOperationActive && primaryHostsRemoteOperation
  // Why: when the primary doesn't host the in-flight op (e.g. Fetch, or any
  // dropdown action that mismatches the primary's natural label) the click
  // would otherwise be silent — the toast only fires on failure and a
  // no-op fetch leaves status counts unchanged. Spinning the chevron gives
  // the user immediate feedback that the action they picked is running,
  // while still leaving the menu reachable to read the disabled-row
  // tooltips.
  const showChevronSpinner =
    (isCommitting || isCreatingPr || isRemoteOperationActive) && !showSpinner
  const commitFailureSummary = useMemo(
    () => (commitError ? summarizeCommitFailure(commitError) : null),
    [commitError]
  )
  const commitFailureKindLabel = useMemo(
    () =>
      commitFailureSummary ? getSourceControlRecoveryFailureKindLabel(commitFailureSummary) : null,
    [commitFailureSummary]
  )
  const hasCommitFailureDetails = useMemo(
    () =>
      commitError && commitFailureSummary
        ? hasExpandedCommitFailureDetails(commitError, commitFailureSummary)
        : false,
    [commitError, commitFailureSummary]
  )
  // Why: most primary-kind labels are anchored by a directional icon so
  // the affirmative Commit (✓) reads distinctly from the remote-state
  // labels sharing this slot — Push (↑), Sync (↕), Publish (☁︎↑). Pull is
  // intentionally icon-less because the down-arrow read as a
  // download/save affordance. The icon is decorative; the label and
  // title attribute carry the meaning for assistive tech.
  const PrimaryIcon = PRIMARY_ICONS[primaryAction.kind]

  const hasMessage = commitMessage.trim().length > 0
  const isCommitMessageDisabled = isCommitMessageFieldDisabled({
    stagedCount,
    hasPartiallyStagedChanges,
    hasMessage,
    hasUnresolvedConflicts,
    isCommitting,
    isRemoteOperationActive,
    isPullRequestOperationActive: isCreatingPr
  })
  const describedBy = [
    commitError ? 'commit-area-error' : null,
    pushRecovery ? 'commit-area-push-error' : null,
    remoteActionError ? 'commit-area-remote-error' : null,
    createPrIntentNotice ? 'commit-area-create-pr-intent' : null,
    generateError ? 'commit-area-generate-error' : null
  ]
    .filter(Boolean)
    .join(' ')

  // Why: only render Generate when it has a runnable path; otherwise the
  // composer should stay focused on the normal Commit action.
  // Why: Create PR intent owns message generation and surfaces status via the
  // inline notice; a second composer spinner stacks on the primary spinner.
  const showGenerate =
    showComposer && aiEnabled && !isCreatePrIntentInFlight && (aiAgentConfigured || isGenerating)
  let generateDisabledReason: string | undefined
  if (isGenerating) {
    generateDisabledReason = 'Generating commit message…'
  } else if (isCommitting) {
    generateDisabledReason = 'Commit in progress…'
  } else if (!aiAgentConfigured) {
    generateDisabledReason = 'Pick an agent in Settings -> Git -> Source Control AI.'
  } else if (stagedCount === 0) {
    generateDisabledReason = 'Stage at least one file to generate a message.'
  } else if (hasMessage) {
    generateDisabledReason = 'Clear the message to regenerate.'
  }
  const isGenerateDisabled =
    !aiAgentConfigured ||
    isGenerating ||
    isCommitting ||
    stagedCount === 0 ||
    hasMessage ||
    hasUnresolvedConflicts
  const moreCommitAndRemoteActionsLabel = translate(
    'auto.components.right.sidebar.SourceControl.cc199ccc5f',
    'More commit and remote actions'
  )
  const moreActionsLabel = translate(
    'auto.components.right.sidebar.SourceControl.4d6e1fd7f3',
    'More actions'
  )
  const dropdownMenuContent = (
    <DropdownMenuContent align="end" className="min-w-[14rem]">
      {dropdownItems.map((entry, index) =>
        entry.kind === 'separator' ? (
          <DropdownMenuSeparator key={`sep-${index}`} />
        ) : (
          <Tooltip key={entry.kind}>
            <TooltipTrigger asChild>
              <div className="block">
                <DropdownMenuItem
                  disabled={entry.disabled}
                  title={entry.title}
                  variant={entry.variant}
                  className="w-full"
                  onSelect={(event) => {
                    if (entry.disabled) {
                      event.preventDefault()
                      return
                    }
                    onDropdownAction(entry.kind)
                  }}
                >
                  <span className="flex min-w-0 flex-col">
                    <span>{entry.label}</span>
                    {entry.hint ? (
                      <span className="truncate text-[10px] text-muted-foreground">
                        {entry.hint}
                      </span>
                    ) : null}
                  </span>
                </DropdownMenuItem>
              </div>
            </TooltipTrigger>
            <TooltipContent side="left" sideOffset={8} className="max-w-72">
              {entry.title}
            </TooltipContent>
          </Tooltip>
        )
      )}
    </DropdownMenuContent>
  )

  return (
    <div className="px-3 pb-2">
      {showComposer ? (
        <div className="relative">
          <textarea
            rows={rows}
            value={commitMessage}
            disabled={isCommitMessageDisabled}
            onChange={(e) => onCommitMessageChange(e.target.value)}
            placeholder={translate(
              'auto.components.right.sidebar.SourceControl.0d0a8359d3',
              'Message'
            )}
            aria-label={translate(
              'auto.components.right.sidebar.SourceControl.b94112eb9e',
              'Commit message'
            )}
            aria-describedby={describedBy || undefined}
            // Why: reserve right padding so typed text does not slide under the
            // absolute-positioned Generate icon in the top-right corner.
            // Why: match Input surface tokens and pin disabled:border-input so
            // Chromium's UA disabled styles don't wash out the field outline.
            className={`mt-0.5 min-h-14 w-full resize-none appearance-none rounded-md border border-input bg-background shadow-xs px-2 py-1.5 text-xs text-foreground outline-none placeholder:text-muted-foreground/70 focus-visible:border-ring focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:border-input disabled:bg-background disabled:text-foreground disabled:shadow-xs dark:bg-input/30 dark:disabled:bg-input/30 ${
              showGenerate ? 'pr-8' : ''
            }`}
          />
          {showGenerate &&
            (isGenerating ? (
              // Why: while generating the icon doubles as the cancel affordance.
              // Default state shows the spinning RefreshCw; on hover/focus we
              // swap to a Square ("stop") with a destructive tint so the user
              // sees that clicking will abort the run. Group/group-hover toggles
              // keep this stateless on the React side.
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    onClick={() => onCancelGenerate()}
                    title={translate(
                      'auto.components.right.sidebar.SourceControl.527e130b6f',
                      'Stop generating'
                    )}
                    aria-label={translate(
                      'auto.components.right.sidebar.SourceControl.ddc1fbd690',
                      'Stop generating commit message'
                    )}
                    className="group absolute right-1.5 top-1.5 inline-flex size-5 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive focus-visible:bg-destructive/10 focus-visible:text-destructive focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-destructive/40"
                  >
                    <RefreshCw className="size-3.5 animate-spin group-hover:hidden group-focus-visible:hidden" />
                    <Square className="hidden size-3.5 fill-current group-hover:block group-focus-visible:block" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="left" sideOffset={6}>
                  {translate(
                    'auto.components.right.sidebar.SourceControl.37a81f29ad',
                    'Generating commit message. Click to stop.'
                  )}
                </TooltipContent>
              </Tooltip>
            ) : (
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    aria-disabled={isGenerateDisabled}
                    onClick={(event) => {
                      if (isGenerateDisabled) {
                        event.preventDefault()
                        return
                      }
                      onGenerate()
                    }}
                    title={
                      generateDisabledReason ??
                      translate(
                        'auto.components.right.sidebar.SourceControl.b16b8f0e4b',
                        'ai commit msg'
                      )
                    }
                    aria-label={translate(
                      'auto.components.right.sidebar.SourceControl.461575b9bc',
                      'Generate commit message with AI'
                    )}
                    className={cn(
                      'absolute right-1.5 top-1.5 inline-flex size-5 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring',
                      isGenerateDisabled &&
                        'cursor-not-allowed opacity-40 hover:bg-transparent hover:text-muted-foreground'
                    )}
                  >
                    <Sparkles className="size-3.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="left" sideOffset={6}>
                  {generateDisabledReason ??
                    translate(
                      'auto.components.right.sidebar.SourceControl.b16b8f0e4b',
                      'ai commit msg'
                    )}
                </TooltipContent>
              </Tooltip>
            ))}
        </div>
      ) : null}
      {/* Why: the current manual action + chevron sit together as a visual
          split button so the edit → commit → push loop stays in a single
          vertical band. The chevron exposes the full action surface without
          forcing morphing labels to carry every possible intent. */}
      <div
        className={cn(showComposer ? 'mt-1 flex items-stretch gap-1' : 'flex items-stretch gap-1')}
      >
        <div className="flex flex-1 items-stretch">
          {/* Why: match the hosted-review action buttons in Checks
              (size="xs", px-3 text-[11px]) so the sidebar has a consistent
              action-button shape across Source Control and Checks. */}
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="flex flex-1">
                <Button
                  type="button"
                  variant="outline"
                  size="xs"
                  disabled={primaryAction.disabled}
                  onClick={() => onPrimaryAction()}
                  className="w-full rounded-r-none px-3 text-[11px]"
                  title={primaryAction.title}
                >
                  {showSpinner ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : PrimaryIcon ? (
                    <PrimaryIcon className="size-3.5" aria-hidden="true" />
                  ) : null}
                  {primaryAction.label}
                </Button>
              </span>
            </TooltipTrigger>
            <TooltipContent side="top" sideOffset={6} className="max-w-72">
              {primaryAction.title}
            </TooltipContent>
          </Tooltip>
          <DropdownMenu>
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="inline-flex shrink-0">
                  <DropdownMenuTrigger asChild>
                    <Button
                      type="button"
                      variant="outline"
                      size="xs"
                      className={cn(
                        'rounded-l-none border-l border-border px-1.5 shrink-0',
                        // Why: mirror the primary's disabled dimming so the split
                        // button reads as one unit when Commit is unavailable. The
                        // chevron itself stays clickable — its dropdown exposes
                        // independently-gated remote actions (push / fetch / pull)
                        // that are still valid when the primary is disabled.
                        primaryAction.disabled && 'opacity-50'
                      )}
                      aria-label={moreCommitAndRemoteActionsLabel}
                      title={moreActionsLabel}
                    >
                      {showChevronSpinner ? (
                        <Loader2 className="size-3.5 animate-spin" />
                      ) : (
                        <ChevronDown className="size-3.5" />
                      )}
                    </Button>
                  </DropdownMenuTrigger>
                </span>
              </TooltipTrigger>
              <TooltipContent side="top" sideOffset={6}>
                {moreCommitAndRemoteActionsLabel}
              </TooltipContent>
            </Tooltip>
            {dropdownMenuContent}
          </DropdownMenu>
        </div>
      </div>
      {commitError && commitFailureSummary ? (
        <SourceControlRecoveryNotice
          id="commit-area-error"
          recoveryKind="commit"
          title={translate(
            'auto.components.right.sidebar.SourceControl.011f9713fc',
            'Commit blocked'
          )}
          detailsTitle={translate(
            'auto.components.right.sidebar.SourceControl.a9bf7c171a',
            'Commit Failed'
          )}
          summary={commitFailureSummary}
          detailText={commitError}
          hasDetails={hasCommitFailureDetails}
          kindLabel={commitFailureKindLabel}
          prompt={commitFailureRecoveryPrompt}
          worktreeId={worktreeId}
          groupId={groupId}
          connectionId={connectionId}
          repoId={repoId}
          launchPlatform={launchPlatform}
          sourceControlAiActionsVisible={sourceControlAiActionsVisible}
          isLaunching={isFixingCommitFailureWithAI}
          recipe={fixCommitFailureRecipe}
          onSaveLaunchActionDefault={onSaveLaunchActionDefault}
          onOpenSourceControlAiSettings={onOpenSourceControlAiSettings}
          onFixWithAI={onFixCommitFailureWithAI}
        />
      ) : null}
      {pushRecovery ? (
        <SourceControlRecoveryNotice
          id="commit-area-push-error"
          recoveryKind="push"
          title={translate(
            'auto.components.right.sidebar.SourceControl.pushRecovery.011f9713fc',
            'Push blocked'
          )}
          detailsTitle={translate(
            'auto.components.right.sidebar.SourceControl.pushRecovery.a9bf7c171a',
            'Push Failed'
          )}
          summary={pushRecovery.summary}
          detailText={pushRecovery.detailText}
          hasDetails={pushRecovery.hasDetails}
          kindLabel={pushRecovery.kindLabel}
          prompt={pushRecovery.prompt}
          worktreeId={worktreeId}
          groupId={groupId}
          connectionId={connectionId}
          repoId={repoId}
          launchPlatform={launchPlatform}
          sourceControlAiActionsVisible={sourceControlAiActionsVisible}
          isLaunching={isFixingPushFailureWithAI}
          recipe={fixPushFailureRecipe}
          onSaveLaunchActionDefault={onSaveLaunchActionDefault}
          onOpenSourceControlAiSettings={onOpenSourceControlAiSettings}
          onFixWithAI={onFixPushFailureWithAI}
        />
      ) : null}
      {remoteActionError && isPullPolicyRemoteActionError(remoteActionError) ? (
        <PullPolicyRemoteActionNotice id="commit-area-remote-error" />
      ) : remoteActionError ? (
        <p
          id="commit-area-remote-error"
          role="alert"
          aria-live="polite"
          className="mt-1 text-[11px] text-destructive"
        >
          {remoteActionError}
        </p>
      ) : null}
      {createPrIntentNotice && (
        <div
          id="commit-area-create-pr-intent"
          role={createPrIntentNotice.tone === 'destructive' ? 'alert' : 'status'}
          aria-live="polite"
          className={cn(
            'mt-1 flex min-w-0 items-center gap-1.5 text-[11px]',
            createPrIntentNotice.tone === 'destructive'
              ? 'text-destructive'
              : 'text-muted-foreground'
          )}
        >
          {/* Why: Create Review blockers carry recovery steps; truncating them hides
          the action the user needs in the default narrow sidebar. */}
          <span className="min-w-0 flex-1 break-words leading-4 [overflow-wrap:anywhere]">
            {createPrIntentNotice.message}
          </span>
          {createPrIntentNotice.action === 'settings' && onOpenSourceControlAiSettings ? (
            <button
              type="button"
              className="shrink-0 font-medium text-foreground underline decoration-border underline-offset-2 hover:decoration-foreground"
              onClick={() => onOpenSourceControlAiSettings()}
            >
              {translate(
                'auto.components.right.sidebar.SourceControl.473f18758e',
                'Source Control AI settings'
              )}
            </button>
          ) : null}
        </div>
      )}
      {generateError && (
        <p
          id="commit-area-generate-error"
          role="alert"
          aria-live="polite"
          className="mt-1 text-[11px] text-destructive"
        >
          {generateError}
        </p>
      )}
    </div>
  )
}
