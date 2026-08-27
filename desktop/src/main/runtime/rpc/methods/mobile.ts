// Why: RPC-channel counterpart to the desktop-only 'mobile:*' ipcMain
// handlers (see ipc/mobile.ts / registerMobileHandlers). listNetworkInterfaces
// has zero dependency on OrcaRuntimeRpcServer's pairing/device-registry state
// (deviceRegistry, e2ee keypair, wsTransport); the other 8 need that state.
// OrcaRuntimeRpcServer never threads into RpcContext (RpcDispatcher only
// injects runtime/devServerManager/signal/userId — see rpc/dispatcher.ts and
// rpc/core.ts), so these read the live instance lazily via
// getActiveRuntimeRpcServer() (set by registerMobileHandlers at real
// bootstrap time in main/index.ts) instead — the same singleton-accessor
// pattern getActiveStarNagService()/getActiveOnboardingStore() use.
import { app } from 'electron'
import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import {
  getActiveRuntimeRpcServer,
  getNetworkInterfaces,
  type NetworkInterface
} from '../../../ipc/mobile'
import type { RuntimeAccessGrant } from '../../../../shared/runtime-access-grants'
import type { DeviceEntry } from '../../device-registry'
import {
  getWebSocketPort,
  inspectWindowsMobileFirewall
} from '../../windows-mobile-firewall'

function requireRuntimeRpcServer(): NonNullable<ReturnType<typeof getActiveRuntimeRpcServer>> {
  const rpcServer = getActiveRuntimeRpcServer()
  if (!rpcServer) {
    throw new Error('runtime_rpc_server_unavailable')
  }
  return rpcServer
}

function toRuntimeAccessGrant(device: DeviceEntry): RuntimeAccessGrant {
  return {
    deviceId: device.deviceId,
    name: device.name,
    createdAt: device.pairedAt,
    lastSeenAt: device.lastSeenAt > 0 ? device.lastSeenAt : null
  }
}

const PairingOfferParams = z
  .object({
    address: z.string().optional(),
    rotate: z.boolean().optional()
  })
  .nullable()
  .optional()

const DeviceIdParams = z.object({
  deviceId: z.string().min(1)
})

const FirewallStatusParams = z
  .object({
    address: z.string().optional()
  })
  .nullable()
  .optional()

export const MOBILE_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'mobile.listNetworkInterfaces',
    params: null,
    handler: (): { interfaces: NetworkInterface[] } => ({
      interfaces: getNetworkInterfaces()
    })
  }),
  defineMethod({
    name: 'mobile.getPairingQR',
    params: PairingOfferParams,
    handler: (params) => {
      const rpcServer = requireRuntimeRpcServer()
      const ip = params?.address ?? getDefaultPairingAddress()
      if (!ip) {
        return { available: false as const }
      }
      const offer = rpcServer.createPairingOffer({
        address: ip,
        rotate: params?.rotate,
        name: `Mobile ${new Date().toLocaleDateString()}`,
        scope: 'mobile'
      })
      if (!offer.available) {
        return { available: false as const }
      }
      // Why: QR image generation is desktop-renderer-only (canvas/dataURL
      // convenience for the pairing dialog); the RPC channel returns the raw
      // pairing URL so any caller (mobile app, CLI) can render its own code.
      return {
        available: true as const,
        pairingUrl: offer.pairingUrl,
        endpoint: offer.endpoint,
        deviceId: offer.deviceId
      }
    }
  }),
  defineMethod({
    name: 'mobile.getRuntimePairingUrl',
    params: PairingOfferParams,
    handler: (params) => {
      const rpcServer = requireRuntimeRpcServer()
      const ip = params?.address ?? getDefaultPairingAddress()
      if (!ip) {
        return { available: false as const }
      }
      const offer = rpcServer.createPairingOffer({
        address: ip,
        rotate: params?.rotate,
        name: `Runtime ${new Date().toLocaleDateString()}`,
        scope: 'runtime'
      })
      if (!offer.available) {
        return { available: false as const }
      }
      return {
        available: true as const,
        pairingUrl: offer.pairingUrl,
        webClientUrl: offer.webClientUrl,
        endpoint: offer.endpoint,
        deviceId: offer.deviceId
      }
    }
  }),
  defineMethod({
    name: 'mobile.listDevices',
    params: null,
    handler: () => {
      const registry = requireRuntimeRpcServer().getDeviceRegistry()
      if (!registry) {
        return { devices: [] }
      }
      // Why: devices with lastSeenAt === 0 were created during QR generation
      // but never actually scanned/connected. Showing them as "paired" is
      // misleading, so we filter them out.
      return {
        devices: registry
          .listDevices()
          .filter((d) => d.scope === 'mobile' && d.lastSeenAt > 0)
          .map((d) => ({
            deviceId: d.deviceId,
            name: d.name,
            pairedAt: d.pairedAt,
            lastSeenAt: d.lastSeenAt
          }))
      }
    }
  }),
  defineMethod({
    name: 'mobile.listRuntimeAccessGrants',
    params: null,
    handler: () => {
      const registry = requireRuntimeRpcServer().getDeviceRegistry()
      if (!registry) {
        return { grants: [] }
      }
      return {
        grants: registry
          .listDevices()
          .filter((d) => d.scope === 'runtime')
          .sort((a, b) => b.pairedAt - a.pairedAt)
          .map(toRuntimeAccessGrant)
      }
    }
  }),
  defineMethod({
    name: 'mobile.revokeDevice',
    params: DeviceIdParams,
    handler: (params) => {
      const rpcServer = requireRuntimeRpcServer()
      if (!rpcServer.getDeviceRegistry()) {
        return { revoked: false }
      }
      return { revoked: rpcServer.revokeMobileDevice(params.deviceId) }
    }
  }),
  defineMethod({
    name: 'mobile.revokeRuntimeAccess',
    params: DeviceIdParams,
    handler: (params) => {
      const rpcServer = requireRuntimeRpcServer()
      if (!rpcServer.getDeviceRegistry()) {
        return { revoked: false }
      }
      return { revoked: rpcServer.revokeRuntimeAccess(params.deviceId) }
    }
  }),
  defineMethod({
    name: 'mobile.isWebSocketReady',
    params: null,
    handler: () => {
      const rpcServer = requireRuntimeRpcServer()
      return {
        ready: rpcServer.getWebSocketEndpoint() !== null,
        endpoint: rpcServer.getWebSocketEndpoint()
      }
    }
  }),
  defineMethod({
    name: 'mobile.getWindowsFirewallStatus',
    params: FirewallStatusParams,
    handler: (params) => {
      const rpcServer = requireRuntimeRpcServer()
      const port = getWebSocketPort(rpcServer.getWebSocketEndpoint())
      return inspectWindowsMobileFirewall(port, params?.address, {
        platform: process.platform,
        isPackaged: app.isPackaged,
        executablePath: process.execPath,
        systemRoot: process.env.SystemRoot
      })
    }
  })
]

function getDefaultPairingAddress(): string | null {
  const ifaces = getNetworkInterfaces()
  return ifaces.length > 0 ? ifaces[0]!.address : null
}
