// Why: extracted bootstrap function makes the web entry point unit-testable
// without side-effecting module-level imports in main.tsx.
import React from 'react'
import ReactDOM from 'react-dom/client'
import { lazyWithRetry as lazy } from '@/lib/lazy-with-retry'
import { Suspense, useMemo, useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clearPairingInputFromAddressBar,
  decideWebPairingStartup,
  readPairingInputFromLocation
} from './web-pairing'
import {
  createStoredWebRuntimeEnvironment,
  readStoredWebRuntimeEnvironment,
  saveStoredWebRuntimeEnvironment
} from './web-runtime-environment'
import { installWebPreloadApi } from './web-preload-api'
import { I18nProvider } from '../i18n/I18nProvider'
import { translate } from '../i18n/i18n'
import { RecoverableRenderErrorBoundary } from '../components/error-boundaries/RecoverableRenderErrorBoundary'
import { ConnectionStatusProvider } from './ConnectionStatusProvider'
import { ConnectionStatusBanner } from './ConnectionStatusBanner'
import {
  useConnectionStatus,
  useConnectionRetry
} from './ConnectionStatusProvider'
import { WebSocketRpcClient } from '../../../platform/adapters/web/rpc-client'
import type { IRpcClient } from '../../../platform/rpc-client-interface'
import { fetchCurrentUser, fetchAuthConfig } from '../auth/auth-api-client'
import type { AuthUser, SsoProvider } from '../auth/auth-types'

const WebConnect = lazy(() => import('./WebConnect'))
const App = lazy(() => import('../App'))
const LoginPage = lazy(() => import('./login/LoginPage').then((m) => ({ default: m.LoginPage })))

export interface BootstrapOptions {
  rootElementId?: string
  maxRetries?: number
  retryDelayMs?: number
  wsUrl?: string
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function showErrorUi(rootEl: HTMLElement): void {
  rootEl.innerHTML = `
    <div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:system-ui;color:#ef4444">
      <div style="text-align:center">
        <h2>Cannot connect to Orca backend</h2>
        <p>Make sure the Orca server is running at the expected address.</p>
        <button onclick="location.reload()"
                style="padding:8px 16px;background:#3b82f6;color:white;border:none;border-radius:6px;cursor:pointer">
          Retry
        </button>
      </div>
    </div>
  `
}

// Why: banner wrapper reads from ConnectionStatusProvider context so it stays
// in sync with the connection poll without prop-drilling through App.
function WebConnectionBannerWrapper(): React.JSX.Element | null {
  const status = useConnectionStatus()
  const retry = useConnectionRetry()
  return <ConnectionStatusBanner status={status} onRetry={retry} />
}

// Why: WebRoot encapsulates the pairing/app decision so it can be tested
// independently of the ReactDOM.createRoot lifecycle.
interface WebRootProps {
  client: IRpcClient
}

// CR-LOGIN-001 (TASK-FE-007): auth session check result fed in by WebRootBoundary
interface WebRootAuthContext {
  sessionUser: AuthUser | null
  availableProviders: SsoProvider[]
}

function WebRoot({
  client,
  sessionUser,
  availableProviders
}: WebRootProps & WebRootAuthContext): React.JSX.Element {
  const initialPairingInput = useMemo(() => readPairingInputFromLocation(window.location), [])
  const startupDecision = useMemo(() => {
    const decision = decideWebPairingStartup({
      initialPairingInput,
      hasStoredEnvironment: readStoredWebRuntimeEnvironment() !== null
    })
    if (
      decision.kind === 'auto-save-runtime-offer' ||
      (decision.kind === 'show-connect' && decision.initialPairingInput !== null)
    ) {
      clearPairingInputFromAddressBar()
    }
    return decision
  }, [initialPairingInput])

  const [hasEnvironment, setHasEnvironment] = useState(() => {
    if (startupDecision.kind === 'auto-save-runtime-offer') {
      saveStoredWebRuntimeEnvironment(
        createStoredWebRuntimeEnvironment({ name: 'Orca Server', offer: startupDecision.offer })
      )
      return true
    }
    return startupDecision.kind === 'use-stored-environment'
  })

  // CR-LOGIN-001: if the user is already authenticated via session cookie,
  // skip the WebConnect / pairing flow entirely and render the App directly.
  if (sessionUser !== null) {
    installWebPreloadApi()
    return (
      <ConnectionStatusProvider client={client}>
        <WebConnectionBannerWrapper />
        <Suspense fallback={<div className="min-h-dvh bg-background" />}>
          <App />
        </Suspense>
      </ConnectionStatusProvider>
    )
  }

  if (!hasEnvironment) {
    // Show Login page first; PairCodeFallback inside it handles the pairing path.
    // After successful SSO/local login the page reloads and sessionUser will be set.
    return (
      <Suspense fallback={<div className="min-h-dvh bg-background" />}>
        <LoginPage
          availableProviders={availableProviders}
          onLoginSuccess={() => { window.location.href = '/' }}
        />
      </Suspense>
    )
  }

  installWebPreloadApi()
  return (
    <ConnectionStatusProvider client={client}>
      <WebConnectionBannerWrapper />
      <Suspense fallback={<div className="min-h-dvh bg-background" />}>
        <App />
      </Suspense>
    </ConnectionStatusProvider>
  )
}

function WebRootBoundary({ client }: WebRootProps): React.JSX.Element {
  useTranslation()
  // CR-LOGIN-001 (TASK-FE-007): resolve auth session before first render so
  // WebRoot can decide whether to show LoginPage or go straight to App.
  const [sessionUser, setSessionUser] = useState<AuthUser | null>(null)
  const [availableProviders, setAvailableProviders] = useState<SsoProvider[]>([])
  const [authResolved, setAuthResolved] = useState(false)

  useEffect(() => {
    Promise.all([
      fetchCurrentUser().catch(() => null),
      fetchAuthConfig().catch(() => ({ providers: [], localEnabled: true }))
    ]).then(([user, config]) => {
      setSessionUser(user)
      setAvailableProviders(config.providers as SsoProvider[])
      setAuthResolved(true)
    })
  }, [])

  if (!authResolved) {
    // Minimal blank splash while auth check is in flight
    return <div className="min-h-dvh bg-background" />
  }

  return (
    <RecoverableRenderErrorBoundary
      boundaryId="web.root"
      surface="web-root"
      title={translate('app.recoverableError.webTitle', 'Orca web hit a renderer error.')}
      description={translate(
        'app.recoverableError.webDescription',
        'Retry the web client or reconnect to the paired runtime.'
      )}
    >
      <WebRoot
        client={client}
        sessionUser={sessionUser}
        availableProviders={availableProviders}
      />
    </RecoverableRenderErrorBoundary>
  )
}

/**
 * Testable bootstrap function for web mode.
 * Extracted from main.tsx so the startup sequence can be exercised in Vitest.
 */
export async function bootstrapWebApp(options: BootstrapOptions = {}): Promise<void> {
  const {
    rootElementId = 'root',
    maxRetries = 3,
    retryDelayMs = 2000,
    wsUrl
  } = options

  const rootEl = document.getElementById(rootElementId)
  if (!rootEl) {
    console.error(`[Orca Web] Root element #${rootElementId} not found`)
    return
  }

  // Why: create a lightweight RPC client for connection status tracking;
  // the full pairing-based WebRuntimeClient is initialised inside WebRoot.
  const client = new WebSocketRpcClient(wsUrl)

  let connected = false
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      await client.connect()
      connected = true
      break
    } catch {
      if (attempt < maxRetries) {
        await sleep(retryDelayMs)
      }
    }
  }

  if (!connected) {
    showErrorUi(rootEl)
    return
  }

  ReactDOM.createRoot(rootEl).render(
    <React.StrictMode>
      <I18nProvider>
        <WebRootBoundary client={client} />
      </I18nProvider>
    </React.StrictMode>
  )

  // Register Service Worker for Web Push notifications (non-fatal — web mode only)
  if ('serviceWorker' in navigator) {
    try {
      await navigator.serviceWorker.register('/service-worker.js')
      console.log('[Web] Service Worker registered for push notifications')
    } catch {
      console.warn('[Web] Service Worker registration failed (non-fatal)')
    }
  }
}
