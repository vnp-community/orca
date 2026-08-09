// Why: per-device tokens replace the shared runtime auth token for WebSocket
// (mobile) connections. Each paired device gets its own revocable token so
// compromising one device doesn't expose others. The registry is a simple
// JSON file with hardened permissions matching the runtime metadata pattern.
import { randomBytes, randomUUID } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { hardenExistingSecureFile, writeSecureJsonFile } from '../../shared/secure-file'
import type { DeviceScope } from '../../shared/runtime-types'
import type { ScopedPairingToken, OrcaUser } from '../../shared/rbac-types'
import { DEVICE_REGISTRY_FILENAME } from './mobile-pairing-files'

export type { DeviceScope }

export type DeviceEntry = {
  deviceId: string
  name: string
  token: string
  scope: DeviceScope
  pairedAt: number
  lastSeenAt: number
}

export class DeviceRegistry {
  private readonly registryPath: string
  private devices: DeviceEntry[] = []

  // Scoped tokens are in-memory only (not persisted) — they expire in 24h.
  private scopedTokens = new Map<string, ScopedPairingToken>()
  private readonly SCOPED_TOKEN_TTL_MS = 24 * 60 * 60 * 1000

  constructor(userDataPath: string) {
    this.registryPath = join(userDataPath, DEVICE_REGISTRY_FILENAME)
    this.load()
  }

  addDevice(name: string, scope: DeviceScope = 'mobile'): DeviceEntry {
    const entry: DeviceEntry = {
      deviceId: randomUUID(),
      name,
      token: randomBytes(24).toString('hex'),
      scope,
      pairedAt: Date.now(),
      lastSeenAt: 0
    }
    this.devices.push(entry)
    this.save()
    return entry
  }

  // Why: coalesce repeated QR-regenerate clicks onto a single pending token.
  // Each call to addDevice() produces a valid auth credential; without
  // coalescing, every renderer call to mobile:getPairingQR (e.g. the new
  // copy-button flow that encourages regeneration) leaves an orphaned token
  // forever. Returns an existing never-scanned entry if present; otherwise
  // mints a new one and drops any stale pending entries.
  getOrCreatePendingDevice(name: string, scope: DeviceScope = 'mobile'): DeviceEntry {
    const existing = this.devices.find((d) => d.lastSeenAt === 0 && d.scope === scope)
    if (existing) {
      return existing
    }
    return this.addDevice(name, scope)
  }

  // Why: explicit rotation path for "Regenerate QR" — invalidates any
  // existing never-scanned token (e.g. one that was screenshotted, copied
  // to clipboard, or shown on a screen-share) and mints a fresh one. Without
  // this, getOrCreatePendingDevice keeps returning the same token forever
  // until a phone actually pairs, so users have no way to revoke a leaked
  // pre-pairing token.
  rotatePendingDevice(name: string, scope: DeviceScope = 'mobile'): DeviceEntry {
    this.devices = this.devices.filter((d) => d.lastSeenAt !== 0 || d.scope !== scope)
    return this.addDevice(name, scope)
  }

  removeDevice(deviceId: string): boolean {
    const before = this.devices.length
    this.devices = this.devices.filter((d) => d.deviceId !== deviceId)
    if (this.devices.length < before) {
      this.save()
      return true
    }
    return false
  }

  getDevice(deviceId: string): DeviceEntry | null {
    return this.devices.find((d) => d.deviceId === deviceId) ?? null
  }

  listDevices(): readonly DeviceEntry[] {
    return this.devices
  }

  validateToken(token: string): DeviceEntry | null {
    return this.devices.find((d) => d.token === token) ?? null
  }

  updateLastSeen(deviceId: string): void {
    const device = this.devices.find((d) => d.deviceId === deviceId)
    if (device) {
      device.lastSeenAt = Date.now()
      this.save()
    }
  }

  // ── Scoped Pairing Tokens ──────────────────────────────────────────────
  // In-memory only — not persisted to disk. Expire after 24h.

  generateScopedToken(
    user: OrcaUser,
    allowedServerIds: string[],
    allowedProjects: string[],
    agentTrust: 'minimal' | 'standard' | 'full'
  ): ScopedPairingToken {
    this.pruneExpiredTokens()
    const token: ScopedPairingToken = {
      token: randomBytes(32).toString('hex'),
      userId: user.id,
      userEmail: user.email,
      userName: user.name,
      teams: user.teams,
      allowedServerIds,
      allowedProjects,
      agentTrust,
      issuedAt: Date.now(),
      expiresAt: Date.now() + this.SCOPED_TOKEN_TTL_MS,
    }
    this.scopedTokens.set(token.token, token)
    return token
  }

  getScopedToken(token: string): ScopedPairingToken | null {
    const found = this.scopedTokens.get(token)
    if (!found) {return null}
    if (found.expiresAt < Date.now()) {
      this.scopedTokens.delete(token)
      return null
    }
    return found
  }

  revokeScopedToken(token: string): void {
    this.scopedTokens.delete(token)
  }

  revokeAllUserTokens(userId: string): void {
    for (const [key, value] of this.scopedTokens) {
      if (value.userId === userId) {this.scopedTokens.delete(key)}
    }
  }

  pruneExpiredTokens(): void {
    const now = Date.now()
    for (const [key, value] of this.scopedTokens) {
      if (value.expiresAt < now) {this.scopedTokens.delete(key)}
    }
  }

  private load(): void {
    if (!existsSync(this.registryPath)) {
      this.devices = []
      return
    }
    try {
      hardenExistingSecureFile(this.registryPath)
      const parsed = JSON.parse(readFileSync(this.registryPath, 'utf-8')) as DeviceEntry[]
      this.devices = parsed.map((device) => ({
        ...device,
        // Why: older registries only existed for phone pairing. Treat missing
        // scope as mobile so legacy device tokens do not gain new CLI powers.
        scope: device.scope === 'runtime' ? 'runtime' : 'mobile'
      }))
    } catch {
      this.devices = []
    }
  }

  private save(): void {
    writeSecureJsonFile(this.registryPath, this.devices)
  }
}
