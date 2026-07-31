import type { PublicKnownRuntimeEnvironment } from '../../../shared/runtime-environments'
import type { WebPairingOffer } from './web-pairing'
import { createBrowserUuid } from '@/lib/browser-uuid'
import { translate } from '@/i18n/i18n'

export type StoredWebRuntimeEnvironment = Omit<PublicKnownRuntimeEnvironment, 'endpoints'> & {
  endpoints: {
    id: string
    kind: 'websocket'
    label: string
    endpoint: string
    deviceToken: string
    publicKeyB64: string
  }[]
}

const ENVIRONMENT_STORAGE_KEY = 'orca.web.runtimeEnvironment.v1'

export function readStoredWebRuntimeEnvironment(): StoredWebRuntimeEnvironment | null {
  const raw = window.localStorage.getItem(ENVIRONMENT_STORAGE_KEY)
  if (!raw) {
    return null
  }
  try {
    const parsed = JSON.parse(raw) as StoredWebRuntimeEnvironment
    if (!parsed.id || !parsed.name || parsed.endpoints.length === 0) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function saveStoredWebRuntimeEnvironment(environment: StoredWebRuntimeEnvironment): void {
  window.localStorage.setItem(ENVIRONMENT_STORAGE_KEY, JSON.stringify(environment))
}

export function clearStoredWebRuntimeEnvironment(): void {
  window.localStorage.removeItem(ENVIRONMENT_STORAGE_KEY)
}

export function createStoredWebRuntimeEnvironment(args: {
  name: string
  offer: WebPairingOffer
}): StoredWebRuntimeEnvironment {
  const id = `web-${createBrowserUuid()}`
  const now = Date.now()
  return {
    id,
    name: args.name.trim() || 'Orca Server',
    createdAt: now,
    updatedAt: now,
    lastUsedAt: null,
    runtimeId: null,
    preferredEndpointId: `ws-${id}`,
    endpoints: [
      {
        id: `ws-${id}`,
        kind: 'websocket',
        label: translate('auto.web.web.runtime.environment.07f788de83', 'WebSocket'),
        endpoint: args.offer.endpoint,
        deviceToken: args.offer.deviceToken,
        publicKeyB64: args.offer.publicKeyB64
      }
    ]
  }
}

export function redactStoredWebRuntimeEnvironment(
  environment: StoredWebRuntimeEnvironment
): PublicKnownRuntimeEnvironment {
  return {
    ...environment,
    endpoints: environment.endpoints.map(
      ({ deviceToken: _token, publicKeyB64: _key, ...rest }) => ({
        ...rest
      })
    )
  }
}

export function getPreferredWebPairingOffer(
  environment: StoredWebRuntimeEnvironment
): WebPairingOffer {
  const endpoint =
    environment.endpoints.find((entry) => entry.id === environment.preferredEndpointId) ??
    environment.endpoints[0]
  if (!endpoint) {
    throw new Error('No runtime endpoint is stored for this web client.')
  }
  return {
    v: 2,
    endpoint: endpoint.endpoint,
    deviceToken: endpoint.deviceToken,
    publicKeyB64: endpoint.publicKeyB64
  }
}

export function updateStoredEnvironmentRuntimeId(
  environment: StoredWebRuntimeEnvironment,
  runtimeId: string | null
): StoredWebRuntimeEnvironment {
  const next = {
    ...environment,
    runtimeId,
    updatedAt: Date.now(),
    lastUsedAt: Date.now()
  }
  saveStoredWebRuntimeEnvironment(next)
  return next
}

export function isMixedContentWebSocket(endpoint: string): boolean {
  return window.location.protocol === 'https:' && endpoint.startsWith('ws://')
}

/**
 * Create a StoredWebRuntimeEnvironment for session-based auth (no Pair Code / E2EE).
 *
 * Used when ORCA_MULTI_USER=1 and the user has logged in via /auth/local
 * (email + password). WsSessionRouter routes WebSocket connections via
 * session cookie — no E2EE Pair Code required.
 *
 * The generated environment:
 *   - Points to ws(s)://same-host/ws (session-authenticated WS endpoint)
 *   - Has stable id 'session-auth' (no random uuid — consistent across reloads)
 *   - deviceToken = '' and publicKeyB64 = '' (no E2EE — cookie is the auth)
 *
 * @param location - window.location (or equivalent for testability)
 */
export function createSessionWebRuntimeEnvironment(
  location: Pick<Location, 'protocol' | 'host'>
): StoredWebRuntimeEnvironment {
  const wsProtocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsEndpoint = `${wsProtocol}//${location.host}/ws`
  const now = Date.now()
  const envId = 'session-auth'

  return {
    id: envId,
    name: 'Orca Session',
    createdAt: now,
    updatedAt: now,
    lastUsedAt: null,
    runtimeId: null,
    preferredEndpointId: `ws-${envId}`,
    endpoints: [
      {
        id: `ws-${envId}`,
        kind: 'websocket',
        label: 'Session WebSocket',
        endpoint: wsEndpoint,
        // Why: No E2EE keys — WsSessionRouter validates session cookie instead
        // of Curve25519 device token. getClientForEnvironment checks empty keys
        // and routes to WebSocketRpcClient (plain cookie-auth WS).
        deviceToken: '',
        publicKeyB64: ''
      }
    ]
  }
}
