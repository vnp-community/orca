// Bootstrap progress event types — shared between preload API boundary and renderer store (CR-004)
// Why: extracted to avoid circular import between api-types.ts (preload) and bootstrap.ts (store).

export type BootstrapProgressEvent =
  | { type: 'bootstrap.started'; serverId: string; serverLabel: string }
  | { type: 'bootstrap.step.started'; serverId: string; stepId: string }
  | { type: 'bootstrap.step.done'; serverId: string; stepId: string; detail?: string }
  | { type: 'bootstrap.step.error'; serverId: string; stepId: string; error: string }
  | { type: 'bootstrap.step.skipped'; serverId: string; stepId: string; reason: string }
  | { type: 'bootstrap.log'; serverId: string; line: string }
  | { type: 'bootstrap.done'; serverId: string; serverLabel: string }
  | { type: 'bootstrap.error'; serverId: string; serverLabel: string; error: string }
