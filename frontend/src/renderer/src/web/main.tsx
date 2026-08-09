import '../assets/main.css'

import { bootstrapWebApp } from './main-web-bootstrap'

// Check if we are running against the Multi-User Web Server (which has /auth/config)
// If so, use the new bootstrap logic that handles SSO/Local Login and auto-wires the environment.
// If not (e.g. 404), we are in Desktop Pair Code sharing mode, so fallback to the original pair-code UI.
fetch('/auth/config')
  .then((res) => {
    if (res.ok) {
      void bootstrapWebApp()
    } else {
      renderOriginalPairCodeApp()
    }
  })
  .catch(() => {
    // Network error (or cross-origin if misconfigured), fallback to original pair-code mode
    renderOriginalPairCodeApp()
  })

// FIX CR-FE2E-003: only /auth/config 404 (Desktop Pair Code sharing mode)
// reaches this branch — dynamic import keeps TweetNaCl + the E2EE pairing UI
// (WebConnect, web-runtime-client.ts, web-e2ee.ts) out of the bundle every
// multi-user browser downloads. Logic moved verbatim to pair-code-app-entry.tsx.
function renderOriginalPairCodeApp(): void {
  void import('./pair-code-app-entry').then(({ mountPairCodeApp }) => mountPairCodeApp())
}
