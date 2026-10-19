/**
 * RequestAiSetupBanner — TASK-FE-E2E-005
 *
 * Replaces a raw "classifier error" / generic failure with the cause and the fix when the
 * dev server's AI cannot run. Shown for classification, Solution and Plan runs alike.
 *
 * @module components/request/RequestAiSetupBanner
 */

import React from 'react'
import { AlertCircle } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { useOpenDevServerSettings } from './use-open-dev-server-settings'
import type { AiSetupCode } from './request-ai-setup'

const T = 'auto.components.request.aiSetup.'
// Kept literal in every language so the command can be shown as code.
const LOGIN_COMMAND = 'claude auth login'

type Copy = { titleKey: string; titleFallback: string; bodyKey: string; bodyFallback: string }

const NOT_CONFIGURED: Copy = {
  titleKey: `${T}title.notConfigured`,
  titleFallback: 'AI is not set up on the dev server',
  bodyKey: `${T}body.notConfigured`,
  bodyFallback:
    'The dev server is not signed in to Claude Code. Run claude auth login on the dev server (or set an API key for its agent), then try again.'
}

const COPY: Record<AiSetupCode, Copy> = {
  REQUEST_AI_NOT_CONFIGURED: NOT_CONFIGURED,
  REQUEST_AI_NO_PROVIDER: NOT_CONFIGURED,
  REQUEST_AI_CREDENTIAL_UNAVAILABLE: {
    titleKey: `${T}title.credentialUnavailable`,
    titleFallback: 'The AI credential could not be read',
    bodyKey: `${T}body.credentialUnavailable`,
    bodyFallback: 'Check the AI credential configured for the dev server, then try again.'
  },
  REQUEST_AI_SESSION_EXPIRED: {
    titleKey: `${T}title.sessionExpired`,
    titleFallback: 'The Claude Code session on the dev server has expired',
    bodyKey: `${T}body.sessionExpired`,
    bodyFallback: 'Sign in again with claude auth login on the dev server, then try again.'
  },
  REQUEST_AI_QUOTA_EXHAUSTED: {
    titleKey: `${T}title.quotaExhausted`,
    titleFallback: 'The AI quota of the dev server is used up',
    bodyKey: `${T}body.quotaExhausted`,
    bodyFallback:
      'The account behind the Claude Code login on the dev server has no quota left. Wait for it to reset or switch accounts.'
  },
  REQUEST_AI_AGENT_UNAVAILABLE: {
    titleKey: `${T}title.agentUnavailable`,
    titleFallback: 'No dev server is available to run the AI',
    bodyKey: `${T}body.agentUnavailable`,
    bodyFallback: 'Connect a dev server for this project, or update its agent, then try again.'
  },
  REQUEST_AI_BUSY: {
    titleKey: `${T}title.busy`,
    titleFallback: 'The AI is busy',
    bodyKey: `${T}body.busy`,
    bodyFallback:
      'The dev server is already running as many AI jobs as it allows. Try again in a moment.'
  }
}

function withCommandAsCode(text: string): React.ReactNode {
  const at = text.indexOf(LOGIN_COMMAND)
  if (at < 0) {
    return text
  }
  return (
    <>
      {text.slice(0, at)}
      <code className="rounded bg-muted px-1 font-mono text-[11px] text-foreground">
        {LOGIN_COMMAND}
      </code>
      {text.slice(at + LOGIN_COMMAND.length)}
    </>
  )
}

type Props = {
  code: AiSetupCode
  onRetry?: () => void
  retryDisabled?: boolean
}

export function RequestAiSetupBanner({
  code,
  onRetry,
  retryDisabled = false
}: Props): React.JSX.Element {
  const isAdmin = useAppStore((s) => s.currentUser?.role === 'admin')
  const openDevServerSettings = useOpenDevServerSettings()
  const copy = COPY[code]
  return (
    <div
      role="alert"
      data-testid="request-ai-setup-banner"
      data-ai-code={code}
      className="flex flex-col gap-1.5 rounded-md border border-destructive/30 px-3 py-2"
    >
      <p className="flex items-center gap-2 text-sm text-destructive">
        <AlertCircle className="size-4 shrink-0" aria-hidden />
        {translate(copy.titleKey, copy.titleFallback)}
      </p>
      <p className="text-xs text-muted-foreground">
        {withCommandAsCode(translate(copy.bodyKey, copy.bodyFallback))}
      </p>
      <p className="text-xs text-muted-foreground" data-testid="request-ai-setup-code">
        {translate(`${T}code`, 'Error code: {{code}}', { code })}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        {onRetry ? (
          <Button size="xs" variant="outline" disabled={retryDisabled} onClick={onRetry}>
            {translate(`${T}retry`, 'Try again')}
          </Button>
        ) : null}
        {isAdmin ? (
          <Button size="xs" variant="outline" onClick={openDevServerSettings}>
            {translate(`${T}openSettings`, 'Open dev server settings')}
          </Button>
        ) : (
          <span className="text-xs text-muted-foreground" data-testid="request-ai-setup-ask-admin">
            {translate(`${T}askAdmin`, 'Ask your administrator to fix this.')}
          </span>
        )}
      </div>
    </div>
  )
}
