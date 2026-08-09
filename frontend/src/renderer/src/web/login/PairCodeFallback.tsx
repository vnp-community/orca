// TASK-FE-005: PairCodeFallback — backward-compatible pairing section embedded
// inside the Login page. Users who already have an Orca pairing code can still
// connect without going through the new SSO / local-password flow.
import { useState } from 'react'
import { parseWebPairingInput } from '../web-pairing'
import {
  createStoredWebRuntimeEnvironment,
  saveStoredWebRuntimeEnvironment
} from '../web-runtime-environment'

export function PairCodeFallback() {
  const [input, setInput] = useState('')
  const [error, setError] = useState<string | null>(null)

  function handleConnect() {
    const offer = parseWebPairingInput(input.trim())
    if (!offer) {
      setError('Invalid pairing URL or code')
      return
    }
    saveStoredWebRuntimeEnvironment(
      createStoredWebRuntimeEnvironment({ name: 'Orca Server', offer })
    )
    // Reload so bootstrapWebApp detects savedEnv and renders the main App.
    window.location.reload()
  }

  return (
    <div className="pair-code-fallback">
      <label htmlFor="pair-code-input">Pairing URL or Code</label>
      <input
        id="pair-code-input"
        type="text"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="Pairing URL or code"
        onKeyDown={(e) => e.key === 'Enter' && handleConnect()}
      />
      {error && (
        <p className="pair-code-fallback__error" role="alert">
          {error}
        </p>
      )}
      <button
        type="button"
        onClick={handleConnect}
        disabled={!input.trim()}
        className="pair-code-fallback__connect"
      >
        Connect
      </button>
    </div>
  )
}
