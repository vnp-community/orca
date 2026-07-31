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
  createSessionWebRuntimeEnvironment,
  createStoredWebRuntimeEnvironment,
  clearStoredWebRuntimeEnvironment,
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
import { useLogout } from '../hooks/useLogout'
import { initBrowserTrace } from '../../../shared/trace/browser'
import { TracePanel } from '../components/trace/TracePanel'
import { useAppStore } from '../store'

const WebConnect = lazy(() => import('./WebConnect'))
const App = lazy(() => import('../App'))
import { WorkspaceProvider } from '../context/WorkspaceContext'
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

/**
 * Listen for `orca:auth-failed` events emitted by WebSessionClient / WebRuntimeClient
 * when the WebSocket closes with code 4401 (session cookie missing/expired).
 *
 * On auth failure: clear all browser-side state and redirect to /login so the
 * user can sign in again with a fresh session — no manual intervention required.
 *
 * Guards: only runs once (redirected flag), only for session-auth environments
 * (E2EE-paired environments should reconnect, not logout).
 */
function installAuthFailedRedirect(): void {
  let redirected = false
  window.addEventListener('orca:auth-failed', () => {
    if (redirected) return
    const env = readStoredWebRuntimeEnvironment()
    if (env?.id !== 'session-auth') return
    redirected = true
    console.warn('[Orca] Auth failed — clearing session and redirecting to /login')
    try { localStorage.clear() } catch { /* sandboxed iframe */ }
    try { sessionStorage.clear() } catch { /* sandboxed iframe */ }
    document.cookie.split(';').forEach((c) => {
      const name = c.split('=')[0].trim()
      if (!name) return
      const exp = 'expires=Thu, 01 Jan 1970 00:00:00 GMT'
      document.cookie = `${name}=; ${exp}; path=/`
      document.cookie = `${name}=; ${exp}; path=/; domain=${location.hostname}`
      document.cookie = `${name}=; ${exp}; path=/; domain=.${location.hostname}`
    })
    clearStoredWebRuntimeEnvironment()
    window.location.href = '/login'
  })
}

// Why: banner wrapper reads from ConnectionStatusProvider context so it stays
// in sync with the connection poll without prop-drilling through App.
// onLogout is forwarded so the banner's Logout button can clear stale sessions
// (e.g., after container restart) without requiring the user to find the avatar menu.
function WebConnectionBannerWrapper(): React.JSX.Element | null {
  const status = useConnectionStatus()
  const retry = useConnectionRetry()
  const logout = useLogout()
  return <ConnectionStatusBanner status={status} onRetry={retry} onLogout={logout} />
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
    // Why: installWebPreloadApi reads activeEnvironment from localStorage via
    // readStoredWebRuntimeEnvironment(). Without a stored environment, all RPC
    // calls fail with "No active runtime environment" because requireActiveEnvironment()
    // throws. Create a stable 'session-auth' environment (no E2EE — cookie auth)
    // before installing the API so RPC calls route through WebSocketRpcClient.
    // Guard: only create if no existing environment — don't overwrite a paired env.
    // See TASK-PC-002 / TASK-PC-003 in specs/backend/bugs/paircode-v1/.
    if (readStoredWebRuntimeEnvironment() === null) {
      saveStoredWebRuntimeEnvironment(createSessionWebRuntimeEnvironment(window.location))
    }
    installWebPreloadApi()
    return (
      <ConnectionStatusProvider client={client}>
        <WebConnectionBannerWrapper />
        <Suspense fallback={<div className="min-h-dvh bg-background" />}>
        <WorkspaceProvider>
          <App />
        </WorkspaceProvider>
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

  // Install early so auth-failed events from any WebSocket client trigger redirect.
  installAuthFailedRedirect()

  // Initialize browser trace sink — must happen before ReactDOM.createRoot
  // so trace events from initial renders are captured.
  initBrowserTrace((event) => {
    useAppStore.getState().addTraceEvent(event)
  })

  // Ctrl+Shift+T → toggle TracePanel
  document.addEventListener('keydown', (e) => {
    if (e.ctrlKey && e.shiftKey && e.key === 'T') {
      e.preventDefault()
      const state = useAppStore.getState()
      state.setTracePanelOpen(!state.tracePanelOpen)
    }
  })

  ReactDOM.createRoot(rootEl).render(
    <React.StrictMode>
      <I18nProvider>
        <WebRootBoundary client={client} />
        <TracePanel />
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
