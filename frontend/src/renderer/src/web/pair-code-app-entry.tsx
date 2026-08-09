// FIX CR-FE2E-003: split out of main.tsx so the E2EE pairing UI (WebConnect →
// web-runtime-client.ts → web-e2ee.ts/TweetNaCl) is only downloaded when this
// module is actually dynamically imported — i.e. only when /auth/config 404s
// (Desktop Pair Code sharing mode). Multi-user browsers (bootstrapWebApp())
// never trigger this import. Logic below is copied verbatim from main.tsx's
// former WebRoot/WebRootBoundary — do not change behavior here without also
// updating specs/frontend/crs/frontend-e2ee/.
import React, { Suspense, useMemo, useState } from 'react'
import { lazyWithRetry as lazy } from '@/lib/lazy-with-retry'
import ReactDOM from 'react-dom/client'
import { useTranslation } from 'react-i18next'
import { RecoverableRenderErrorBoundary } from '../components/error-boundaries/RecoverableRenderErrorBoundary'
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

// Why: lazyWithRetry (not a bare `lazy(() => import(...))`) matches how
// main-web-bootstrap.tsx already lazy-loads the same component — one free
// reload-on-chunk-failure retry before surfacing an error, instead of hand-
// rolling new resilience logic for this entry point.
const WebConnect = lazy(() => import('./WebConnect'))
const App = lazy(() => import('../App'))

function WebRoot(): React.JSX.Element {
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

  if (!hasEnvironment) {
    return (
      <Suspense fallback={<div className="min-h-dvh bg-background" />}>
        <WebConnect
          initialPairingInput={
            startupDecision.kind === 'show-connect' ? startupDecision.initialPairingInput : null
          }
          onConnected={() => setHasEnvironment(true)}
        />
      </Suspense>
    )
  }

  installWebPreloadApi()
  return (
    <Suspense fallback={<div className="min-h-dvh bg-background" />}>
      <App />
    </Suspense>
  )
}

function WebRootBoundary(): React.JSX.Element {
  useTranslation()
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
      <WebRoot />
    </RecoverableRenderErrorBoundary>
  )
}

export function mountPairCodeApp(): void {
  const rootEl = document.getElementById('root')
  if (rootEl) {
    ReactDOM.createRoot(rootEl).render(
      <I18nProvider>
        <WebRootBoundary />
      </I18nProvider>
    )
  }
}
