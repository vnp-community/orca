import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { createNodeAdapter } from '../index'
import type { IPlatformServices } from '../../../types'

const testPath = join(tmpdir(), `orca-adapter-test-${Date.now()}`)

afterEach(() => {
  if (existsSync(testPath)) rmSync(testPath, { recursive: true })
})

describe('createNodeAdapter()', () => {
  let platform: IPlatformServices

  beforeEach(() => {
    platform = createNodeAdapter({ userDataPath: testPath })
  })

  describe('mode', () => {
    it('mode is "node"', () => {
      expect(platform.mode).toBe('node')
    })
  })

  describe('app', () => {
    it('app is defined', () => {
      expect(platform.app).toBeDefined()
    })

    it('app.getPath("userData") returns testPath', () => {
      expect(platform.app.getPath('userData')).toBe(testPath)
    })

    it('app.whenReady() resolves', async () => {
      await expect(platform.app.whenReady()).resolves.toBeUndefined()
    })

    it('app.isPackaged is true', () => {
      expect(platform.app.isPackaged).toBe(true)
    })
  })

  describe('ipc', () => {
    it('ipc is defined', () => {
      expect(platform.ipc).toBeDefined()
    })

    it('ipc.handle + invoke roundtrip', async () => {
      const ipc = platform.ipc as any
      ipc.handle('test:ping', async () => 'pong')
      const result = await ipc.invoke('test:ping', 0)
      expect(result).toBe('pong')
    })
  })

  describe('windowManager', () => {
    it('windowManager is defined', () => {
      expect(platform.windowManager).toBeDefined()
    })

    it('windowManager.getAllWindows() returns []', () => {
      expect(platform.windowManager.getAllWindows()).toEqual([])
    })

    it('windowManager.createWindow() returns window with id', () => {
      const win = platform.windowManager.createWindow({})
      expect(win.id).toBeGreaterThan(0)
    })
  })

  describe('storage', () => {
    it('storage is defined', () => {
      expect(platform.storage).toBeDefined()
    })

    it('storage encrypt/decrypt roundtrip', () => {
      const enc = platform.storage.encryptString('secret')
      expect(platform.storage.decryptString(enc)).toBe('secret')
    })
  })

  describe('system', () => {
    it('system is defined', () => {
      expect(platform.system).toBeDefined()
    })

    it('system.getCpuCount() >= 1', () => {
      expect(platform.system.getCpuCount()).toBeGreaterThanOrEqual(1)
    })
  })

  describe('inter-component wiring', () => {
    it('ipc.sendToWindow routes to window', () => {
      const win = platform.windowManager.createWindow({})
      const received: any[] = []
      ;(win as any).onSend('push:ch', (args: any[]) => received.push(args))

      platform.ipc.sendToWindow(win.id, 'push:ch', 'payload')

      expect(received).toHaveLength(1)
      expect(received[0]).toEqual(['payload'])
    })

    it('ipc.sendToAll broadcasts to all windows', () => {
      const w1 = platform.windowManager.createWindow({})
      const w2 = platform.windowManager.createWindow({})
      const r1: any[] = []
      const r2: any[] = []
      ;(w1 as any).onSend('broadcast', (a: any[]) => r1.push(a))
      ;(w2 as any).onSend('broadcast', (a: any[]) => r2.push(a))

      platform.ipc.sendToAll('broadcast', 'msg')

      expect(r1).toHaveLength(1)
      expect(r2).toHaveLength(1)
    })
  })

  describe('multiple adapters', () => {
    it('each createNodeAdapter() returns independent instances', () => {
      const p1 = createNodeAdapter({ userDataPath: join(testPath, 'p1') })
      const p2 = createNodeAdapter({ userDataPath: join(testPath, 'p2') })

      expect(p1.app).not.toBe(p2.app)
      expect(p1.ipc).not.toBe(p2.ipc)
      expect(p1.windowManager).not.toBe(p2.windowManager)
    })
  })
})
