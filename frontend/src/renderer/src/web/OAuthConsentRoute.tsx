import { Suspense } from 'react'
import { lazyWithRetry as lazy } from '@/lib/lazy-with-retry'

const McpConsentPage = lazy(() =>
  import('../components/mcp/consent/McpConsentPage').then((m) => ({ default: m.McpConsentPage }))
)

// Why: the consent page must not mount App (terminals/worktrees) — only this minimal shell.
export function OAuthConsentRoute(): React.JSX.Element {
  return (
    <Suspense fallback={<div className="min-h-dvh bg-background" />}>
      <McpConsentPage />
    </Suspense>
  )
}
