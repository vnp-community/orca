import React from 'react'
import type { ConnectionStatus } from './ConnectionStatusProvider'

export interface ConnectionStatusBannerProps {
  status: ConnectionStatus
  onRetry: () => void
}

// Why: banner is web-only — always null when connected so no DOM overhead
export function ConnectionStatusBanner({
  status,
  onRetry
}: ConnectionStatusBannerProps): React.JSX.Element | null {
  if (status === 'connected') return null

  const isConnecting = status === 'connecting'
  const backgroundColor = isConnecting ? '#f59e0b' : '#ef4444'

  return (
    <div
      role="alert"
      aria-live="polite"
      style={{
        position: 'fixed',
        bottom: 16,
        right: 16,
        zIndex: 9999,
        background: backgroundColor,
        color: 'white',
        borderRadius: 8,
        padding: '10px 16px',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
        fontSize: 14
      }}
    >
      {isConnecting ? (
        <>
          <span role="status" aria-busy="true" className="animate-spin">
            ↻
          </span>
          <span>Connecting to Orca backend...</span>
        </>
      ) : (
        <>
          <span>⚠ Connection lost</span>
          <button
            onClick={onRetry}
            style={{
              background: 'rgba(255,255,255,0.2)',
              border: '1px solid rgba(255,255,255,0.4)',
              color: 'white',
              padding: '4px 10px',
              borderRadius: 4,
              cursor: 'pointer',
              fontSize: 12
            }}
          >
            Retry
          </button>
        </>
      )}
    </div>
  )
}
