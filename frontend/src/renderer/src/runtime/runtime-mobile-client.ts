// Why: mirrors runtime-git-client.ts's local-vs-environment routing. Each
// wrapper here defaults to the local Electron IPC bridge (window.api.mobile.*)
// and only crosses into callRuntimeRpc('mobile.*', ...) for a non-local
// target — see desktop/src/main/runtime/rpc/methods/mobile.ts's file header
// for why the RPC side needs OrcaRuntimeRpcServer's live singleton.
import QRCode from 'qrcode'
import type { RuntimeAccessGrant } from '../../../shared/runtime-access-grants'
import type { WindowsMobileFirewallStatus } from '../../../shared/windows-mobile-firewall'
import { callRuntimeRpc, type RuntimeClientTarget } from './runtime-rpc-client'

export type RuntimeNetworkInterface = { name: string; address: string }

export type RuntimePairedDevice = {
  deviceId: string
  name: string
  pairedAt: number
  lastSeenAt: number
}

export type RuntimeMobilePairingOffer =
  | { available: false }
  | {
      available: true
      qrDataUrl: string
      pairingUrl: string
      endpoint: string
      deviceId: string
    }

export type RuntimePairingUrlOffer =
  | { available: false }
  | {
      available: true
      pairingUrl: string
      webClientUrl: string | null
      endpoint: string
      deviceId: string
    }

const LOCAL_TARGET: RuntimeClientTarget = { kind: 'local' }

export async function listRuntimeNetworkInterfaces(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<{ interfaces: RuntimeNetworkInterface[] }> {
  if (target.kind === 'local') {
    return window.api.mobile.listNetworkInterfaces()
  }
  return callRuntimeRpc<{ interfaces: RuntimeNetworkInterface[] }>(
    target,
    'mobile.listNetworkInterfaces'
  )
}

export async function getRuntimeMobilePairingQR(
  args?: { address?: string; rotate?: boolean },
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<RuntimeMobilePairingOffer> {
  if (target.kind === 'local') {
    return window.api.mobile.getPairingQR(args)
  }
  const offer = await callRuntimeRpc<
    | { available: false }
    | { available: true; pairingUrl: string; endpoint: string; deviceId: string }
  >(target, 'mobile.getPairingQR', args)
  if (!offer.available) {
    return offer
  }
  // Why: QR image rendering is desktop-renderer-only convenience on the
  // ipcMain path (see ipc/mobile.ts); the RPC channel only returns the raw
  // pairing URL, so a remote target renders its own code client-side from
  // the exact same 'qrcode' package with matching options.
  const qrDataUrl = await QRCode.toDataURL(offer.pairingUrl, {
    errorCorrectionLevel: 'M',
    margin: 2,
    width: 256
  })
  return { ...offer, qrDataUrl }
}

export async function getRuntimeMobilePairingUrl(
  args?: { address?: string; rotate?: boolean },
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<RuntimePairingUrlOffer> {
  if (target.kind === 'local') {
    return window.api.mobile.getRuntimePairingUrl(args)
  }
  return callRuntimeRpc<RuntimePairingUrlOffer>(target, 'mobile.getRuntimePairingUrl', args)
}

export async function listRuntimePairedDevices(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<{ devices: RuntimePairedDevice[] }> {
  if (target.kind === 'local') {
    return window.api.mobile.listDevices()
  }
  return callRuntimeRpc<{ devices: RuntimePairedDevice[] }>(target, 'mobile.listDevices')
}

export async function listRuntimeAccessGrants(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<{ grants: RuntimeAccessGrant[] }> {
  if (target.kind === 'local') {
    return window.api.mobile.listRuntimeAccessGrants()
  }
  return callRuntimeRpc<{ grants: RuntimeAccessGrant[] }>(target, 'mobile.listRuntimeAccessGrants')
}

export async function revokeRuntimeMobileDevice(
  deviceId: string,
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<{ revoked: boolean }> {
  if (target.kind === 'local') {
    return window.api.mobile.revokeDevice({ deviceId })
  }
  return callRuntimeRpc<{ revoked: boolean }>(target, 'mobile.revokeDevice', { deviceId })
}

export async function revokeRuntimeMobileAccess(
  deviceId: string,
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<{ revoked: boolean }> {
  if (target.kind === 'local') {
    return window.api.mobile.revokeRuntimeAccess({ deviceId })
  }
  return callRuntimeRpc<{ revoked: boolean }>(target, 'mobile.revokeRuntimeAccess', { deviceId })
}

export async function isRuntimeMobileWebSocketReady(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<{ ready: boolean; endpoint: string | null }> {
  if (target.kind === 'local') {
    return window.api.mobile.isWebSocketReady()
  }
  return callRuntimeRpc<{ ready: boolean; endpoint: string | null }>(
    target,
    'mobile.isWebSocketReady'
  )
}

export async function getRuntimeMobileWindowsFirewallStatus(
  args?: { address?: string },
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<WindowsMobileFirewallStatus> {
  if (target.kind === 'local') {
    return window.api.mobile.getWindowsFirewallStatus(args)
  }
  return callRuntimeRpc<WindowsMobileFirewallStatus>(
    target,
    'mobile.getWindowsFirewallStatus',
    args
  )
}
