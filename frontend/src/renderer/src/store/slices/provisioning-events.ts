// Provisioning progress event types — shared between preload API types and renderer store (CR-003)
// Why: extracted into a separate file to avoid circular imports between
// api-types.ts (preload boundary) and provisioning.ts (renderer store).

export type ProvisioningProgressEvent =
  | { type: 'server.started'; serverId: string }
  | { type: 'server.relay-deploying'; serverId: string }
  | { type: 'server.done'; serverId: string; relayVersion: string }
  | { type: 'server.error'; serverId: string; error: string }
  | { type: 'server.skipped'; serverId: string; reason: string }
  | { type: 'session.done'; totalDone: number; totalFailed: number }
