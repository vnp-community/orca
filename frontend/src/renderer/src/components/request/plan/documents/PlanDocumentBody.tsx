/**
 * PlanDocumentBody — CR-REQ-045
 *
 * One document of the change on screen: read when its tab is opened, shown rendered or as the source text the agent wrote.
 * Nothing here injects HTML: Rendered goes through RequestMarkdownContent, Source is plain text in a <pre>.
 *
 * @module components/request/plan/documents/PlanDocumentBody
 */

import React from 'react'
import { AlertTriangle, FileWarning, History } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { McpCopyButton } from '../../../settings/mcp/McpCopyButton'
import { usePlanBundleFile } from '../../../../hooks/usePlanBundleFile'
import { RequestMarkdownContent } from '../../solution/RequestMarkdownContent'
import { formatBytes, type DocumentTab } from './plan-document-model'

export type DocumentMode = 'rendered' | 'source'

type Props = {
  requestId: string
  tab: DocumentTab
  /** The old version on screen; undefined reads the current one. */
  revision?: number
  /** Says which current version is on screen (its commit or digest), so a rewritten file is read again. */
  currentKey: string
  /** False once the text of this version was pruned: only the file index is left. */
  textKept: boolean
  branch: string
  mode: DocumentMode
  onModeChange: (mode: DocumentMode) => void
}

function Notice({
  icon: Icon,
  testId,
  children
}: {
  icon: typeof FileWarning
  testId: string
  children: React.ReactNode
}): React.JSX.Element {
  return (
    <div
      role="status"
      data-testid={testId}
      className="flex items-start gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground"
    >
      <Icon className="mt-0.5 size-4 shrink-0" aria-hidden />
      <p className="min-w-0 break-words">{children}</p>
    </div>
  )
}

export function PlanDocumentBody({
  requestId,
  tab,
  revision,
  currentKey,
  textKept,
  branch,
  mode,
  onModeChange
}: Props): React.JSX.Element {
  // Why: the index already says what cannot be read, so those files are never requested.
  const skipRead = tab.omitted || tab.change === 'removed' || !textKept
  const file = usePlanBundleFile(requestId, skipRead ? null : tab.path, revision, currentKey)

  if (!textKept) {
    return (
      <Notice icon={History} testId="plan-document-pruned">
        {translate(
          'auto.components.request.plan.documents.pruned',
          'The text of this version is no longer kept.'
        )}
      </Notice>
    )
  }
  if (tab.change === 'removed') {
    return (
      <Notice icon={FileWarning} testId="plan-document-removed">
        {translate(
          'auto.components.request.plan.documents.removed',
          'This file was removed in this version.'
        )}
      </Notice>
    )
  }
  if (tab.omitted || file.status === 'omitted') {
    return (
      <Notice icon={FileWarning} testId="plan-document-omitted">
        {translate(
          'auto.components.request.plan.documents.omitted',
          'This file is too large to display ({{size}}). It is on the proposal branch {{branch}}.',
          { size: formatBytes(tab.size), branch: branch === '' ? '-' : branch }
        )}
      </Notice>
    )
  }
  if (file.status === 'pruned') {
    return (
      <Notice icon={History} testId="plan-document-pruned">
        {translate(
          'auto.components.request.plan.documents.pruned',
          'The text of this version is no longer kept.'
        )}
      </Notice>
    )
  }
  if (file.status === 'notFound') {
    return (
      <Notice icon={FileWarning} testId="plan-document-not-found">
        {translate(
          'auto.components.request.plan.documents.notFound',
          'This file is not in this version.'
        )}
      </Notice>
    )
  }
  if (file.status === 'error') {
    return (
      <div
        role="alert"
        data-testid="plan-document-error"
        className="flex flex-wrap items-center gap-2 text-sm text-destructive"
      >
        <AlertTriangle className="size-4 shrink-0" aria-hidden />
        <span className="min-w-0 flex-1">{file.message}</span>
        <Button size="sm" variant="outline" onClick={file.retry}>
          {translate('auto.components.request.plan.documents.tryAgain', 'Try again')}
        </Button>
      </div>
    )
  }
  if (file.status !== 'ready') {
    return (
      <div className="flex flex-col gap-2" aria-busy="true" data-testid="plan-document-loading">
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-4 w-2/3" />
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2" data-testid="plan-document-body">
      <div className="flex items-center gap-2">
        <ToggleGroup
          type="single"
          size="sm"
          variant="outline"
          value={mode}
          onValueChange={(next) => {
            if (next === 'rendered' || next === 'source') {
              onModeChange(next)
            }
          }}
          aria-label={translate(
            'auto.components.request.plan.documents.modeLabel',
            'Document view'
          )}
        >
          <ToggleGroupItem value="rendered">
            {translate('auto.components.request.plan.documents.modeRendered', 'Rendered')}
          </ToggleGroupItem>
          <ToggleGroupItem value="source">
            {translate('auto.components.request.plan.documents.modeSource', 'Source')}
          </ToggleGroupItem>
        </ToggleGroup>
        {mode === 'source' && (
          <McpCopyButton
            text={file.content}
            ariaLabel={translate(
              'auto.components.request.plan.documents.copySource',
              'Copy document source'
            )}
            quiet
          />
        )}
      </div>
      {mode === 'source' ? (
        <pre
          tabIndex={0}
          data-testid="plan-document-source"
          className="max-h-[60vh] overflow-auto scrollbar-sleek whitespace-pre-wrap break-words rounded-md border border-border bg-muted/40 p-3 font-mono text-xs"
        >
          {file.content}
        </pre>
      ) : file.content.trim() === '' ? (
        <p className="text-sm text-muted-foreground" data-testid="plan-document-empty">
          {translate('auto.components.request.plan.documents.emptyFile', 'This file is empty.')}
        </p>
      ) : (
        <RequestMarkdownContent gfm content={file.content} title={tab.label} />
      )}
    </div>
  )
}
