import { useState } from 'react'
import { useAddDevServer } from '../../hooks/useAddDevServer'
import { useDevServers } from '../../store/slices/dev-servers'
import { DevServerStatusBadge } from '../dev-server/DevServerStatusBadge'
import type { DevServerConnectionType } from '../../../../shared/dev-server-types'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  /** Called when user clicks Continue with ≥1 connected server */
  onNext: () => void
  /** Called when user clicks Skip */
  onSkip: () => void
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Onboarding wizard step for adding a remote dev server.
 * Supports relay-ssh and relay-websocket connection types.
 *
 * Shows existing connected servers so users who already have a server
 * registered can skip the add form and continue.
 */
export function DevServerStep({ onNext, onSkip }: Props) {
  const { state, testResult, testConnection, addAndConnect, reset } = useAddDevServer()
  const devServers = useDevServers()
  const connectedServers = devServers.filter((ds) => ds.status === 'connected')

  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [connectionType, setConnectionType] = useState<DevServerConnectionType>('relay-websocket')

  const handleTestConnection = async () => {
    await testConnection({
      name: name || 'Dev Server',
      connectionType,
      wsUrl: connectionType === 'relay-websocket' ? host : undefined,
      sshTargetId: connectionType === 'relay-ssh' ? host : undefined,
    })
  }

  const handleAddServer = async () => {
    try {
      await addAndConnect({
        name: name || 'Dev Server',
        connectionType,
        wsUrl: connectionType === 'relay-websocket' ? host : undefined,
        sshTargetId: connectionType === 'relay-ssh' ? host : undefined,
      })
    } catch {
      // error state is handled by useAddDevServer
    }
  }

  return (
    <div className="dev-server-step" data-testid="dev-server-step">
      {/* Existing connected servers */}
      {connectedServers.length > 0 && (
        <div className="dev-server-step__existing">
          <p className="dev-server-step__section-label">Connected dev servers</p>
          <ul className="dev-server-step__server-list">
            {connectedServers.map((ds) => (
              <li key={ds.id} className="dev-server-step__server-row">
                <DevServerStatusBadge status={ds.status} platform={ds.platform} />
                <span className="dev-server-step__server-name">{ds.name}</span>
              </li>
            ))}
          </ul>
          <button
            type="button"
            id="dev-server-continue-btn"
            className="dev-server-step__continue-btn"
            onClick={onNext}
          >
            Continue
          </button>
        </div>
      )}

      {/* Add new server form */}
      <div className="dev-server-step__form">
        <p className="dev-server-step__section-label">
          {connectedServers.length > 0 ? 'Or add another dev server' : 'Add a dev server'}
        </p>

        <label htmlFor="dev-server-connection-type" className="dev-server-step__label">
          Connection type
        </label>
        <select
          id="dev-server-connection-type"
          className="dev-server-step__select"
          value={connectionType}
          onChange={(e) => {
            setConnectionType(e.target.value as DevServerConnectionType)
            reset()
          }}
        >
          <option value="relay-websocket">WebSocket relay (recommended)</option>
          <option value="relay-ssh">SSH relay</option>
        </select>

        <label htmlFor="dev-server-name" className="dev-server-step__label">
          Server name
        </label>
        <input
          id="dev-server-name"
          type="text"
          className="dev-server-step__input"
          placeholder="e.g. My MacBook Pro"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />

        <label htmlFor="dev-server-host" className="dev-server-step__label">
          {connectionType === 'relay-websocket' ? 'WebSocket URL' : 'SSH target ID'}
        </label>
        <input
          id="dev-server-host"
          type="text"
          className="dev-server-step__input"
          placeholder={
            connectionType === 'relay-websocket' ? 'ws://devserver.local:6799' : 'ssh-target-id'
          }
          value={host}
          onChange={(e) => {
            setHost(e.target.value)
            reset()
          }}
        />

        {/* Test connection */}
        <button
          type="button"
          id="test-connection-btn"
          className="dev-server-step__test-btn"
          disabled={!host || state === 'testing' || state === 'connecting'}
          onClick={() => void handleTestConnection()}
        >
          {state === 'testing' ? 'Testing…' : 'Test connection'}
        </button>

        {/* Test result feedback */}
        {testResult && (
          <p
            className={`dev-server-step__test-result dev-server-step__test-result--${testResult.ok ? 'ok' : 'error'}`}
          >
            {testResult.ok
              ? `✓ Connected — ${testResult.platform} · Node ${testResult.nodeVersion}`
              : `✗ ${testResult.error}${testResult.ok === false && testResult.hint ? ` (${testResult.hint})` : ''}`}
          </p>
        )}

        {/* Add button */}
        <button
          type="button"
          id="add-server-btn"
          className="dev-server-step__add-btn"
          disabled={!testResult?.ok || state === 'connecting' || state === 'testing'}
          onClick={() => void handleAddServer()}
        >
          {state === 'connecting' ? 'Connecting…' : 'Add dev server'}
        </button>

        {state === 'error' && !testResult && (
          <p className="dev-server-step__error">Failed to connect. Please check your settings.</p>
        )}
      </div>

      {/* Skip */}
      <button
        type="button"
        id="dev-server-skip-btn"
        className="dev-server-step__skip-btn"
        onClick={onSkip}
      >
        Skip for now
      </button>
    </div>
  )
}
