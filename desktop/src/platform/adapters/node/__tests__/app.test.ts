import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { NodeApp } from '../app'
import { runIAppConformanceTests } from '../../../__tests__/interface-conformance'

const testDataPath = join(tmpdir(), `orca-node-app-test-${Date.now()}`)

// Cleanup after each test group
afterEach(() => {
  if (existsSync(testDataPath)) {rmSync(testDataPath, { recursive: true })}
})

// ── Conformance ──────────────────────────────────────────────────────────────
runIAppConformanceTests(() => new NodeApp({ userDataPath: testDataPath }))

// ── NodeApp-specific ─────────────────────────────────────────────────────────
describe('NodeApp — specific behavior', () => {
  let app: NodeApp

  beforeEach(() => {
    app = new NodeApp({ userDataPath: testDataPath })
  })

  describe('userDataPath resolution', () => {
    it('uses option.userDataPath when provided', () => {
      expect(app.getPath('userData')).toBe(testDataPath)
    })

    it('creates userData directory if missing', () => {
      const freshPath = join(tmpdir(), `orca-fresh-${Date.now()}`)
      new NodeApp({ userDataPath: freshPath })
      expect(existsSync(freshPath)).toBe(true)
      rmSync(freshPath, { recursive: true })
    })

    it('uses ORCA_USER_DATA_PATH env var when no option', () => {
      const envPath = join(tmpdir(), `orca-env-${Date.now()}`)
      process.env.ORCA_USER_DATA_PATH = envPath
      const envApp = new NodeApp()
      expect(envApp.getPath('userData')).toBe(envPath)
      delete process.env.ORCA_USER_DATA_PATH
      if (existsSync(envPath)) {rmSync(envPath, { recursive: true })}
    })

    it('falls back to ~/.orca when no option and no env var', () => {
      delete process.env.ORCA_USER_DATA_PATH
      const defaultApp = new NodeApp()
      expect(defaultApp.getPath('userData')).toContain('.orca')
    })
  })

  describe('getVersion()', () => {
    it('returns ORCA_VERSION env var', () => {
      process.env.ORCA_VERSION = '9.9.9-test'
      expect(app.getVersion()).toBe('9.9.9-test')
      delete process.env.ORCA_VERSION
    })

    it('returns "0.0.0" as fallback', () => {
      delete process.env.ORCA_VERSION
      expect(app.getVersion()).toBe('0.0.0')
    })
  })

  describe('path mappings', () => {
    const paths = ['home', 'temp', 'desktop', 'documents', 'downloads'] as const
    it.each(paths)('getPath("%s") returns a string', (name) => {
      expect(typeof app.getPath(name)).toBe('string')
      expect(app.getPath(name).length).toBeGreaterThan(0)
    })

    it('getPath(unknown) returns path inside userData', () => {
      const p = app.getPath('my-custom-dir' as any)
      expect(p.startsWith(testDataPath)).toBe(true)
      expect(p).toContain('my-custom-dir')
    })
  })

  describe('event emission', () => {
    it('on/emit works via EventEmitter', () => {
      const handler = vi.fn()
      app.on('before-quit', handler)
      app.emit('before-quit')
      expect(handler).toHaveBeenCalledOnce()
    })

    it('off removes listener', () => {
      const handler = vi.fn()
      app.on('test-event', handler)
      app.off('test-event', handler)
      app.emit('test-event')
      expect(handler).not.toHaveBeenCalled()
    })
  })

  describe('no-ops', () => {
    it('relaunch() warns without throwing', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(() => app.relaunch()).not.toThrow()
      expect(warn).toHaveBeenCalledWith(expect.stringContaining('no-op'))
      warn.mockRestore()
    })

    it('setName() does not throw', () => {
      expect(() => app.setName('Orca')).not.toThrow()
    })

    it('disableHardwareAcceleration() does not throw', () => {
      expect(() => app.disableHardwareAcceleration()).not.toThrow()
    })
  })

  describe('whenReady()', () => {
    it('resolves immediately', async () => {
      const start = Date.now()
      await app.whenReady()
      expect(Date.now() - start).toBeLessThan(10)
    })

    it('can be awaited multiple times', async () => {
      await app.whenReady()
      await expect(app.whenReady()).resolves.toBeUndefined()
    })
  })
})
