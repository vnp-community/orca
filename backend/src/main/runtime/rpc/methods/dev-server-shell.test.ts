import { describe, expect, it, vi } from 'vitest'
import { DEV_SERVER_METHODS } from './dev-server'
import type { RpcContext } from '../core'

// Why a separate file: dev-server.ts already covers list/add/connect/browseDir/
// mkdir/rmdir elsewhere (no existing test file for that surface either); this
// focuses on the 3 methods added for the web/server-mode DevServerFilePickerDialog
// flow (pathExists/readFile/copyFile) without duplicating the whole method list.

function methodByName(name: string) {
  const method = DEV_SERVER_METHODS.find((m) => m.name === name)
  if (!method) {throw new Error(`Method not found: ${name}`)}
  return method
}

function contextWithRelay(call: (method: string, params: unknown, timeoutMs: number) => Promise<unknown>): RpcContext {
  return {
    devServerManager: {
      getRelay: () => ({ call })
    }
  } as unknown as RpcContext
}

describe('devServer.pathExists', () => {
  it('returns true when fs.stat resolves', async () => {
    const call = vi.fn().mockResolvedValue({ path: '/tmp/x', isFile: true })
    const result = await methodByName('devServer.pathExists').handler(
      { id: 'dev-1', path: '/tmp/x' },
      contextWithRelay(call)
    )
    expect(result).toBe(true)
    expect(call).toHaveBeenCalledWith('fs.stat', { path: '/tmp/x' }, 10_000)
  })

  it('returns false when fs.stat reports the path missing', async () => {
    const call = vi.fn().mockRejectedValue(new Error('Not found: /tmp/missing'))
    const result = await methodByName('devServer.pathExists').handler(
      { id: 'dev-1', path: '/tmp/missing' },
      contextWithRelay(call)
    )
    expect(result).toBe(false)
  })

  it('rethrows a real relay failure instead of reporting false', async () => {
    const call = vi.fn().mockRejectedValue(new Error('Connection lost'))
    await expect(
      methodByName('devServer.pathExists').handler(
        { id: 'dev-1', path: '/tmp/x' },
        contextWithRelay(call)
      )
    ).rejects.toThrow(/connection error/)
  })
})

describe('devServer.readFile', () => {
  it('returns the agent-reported content/encoding', async () => {
    const call = vi.fn().mockResolvedValue({
      path: '/tmp/icon.png',
      content: 'YmFzZTY0',
      encoding: 'base64',
      isBinary: true
    })
    const result = await methodByName('devServer.readFile').handler(
      { id: 'dev-1', path: '/tmp/icon.png' },
      contextWithRelay(call)
    )
    expect(result).toEqual({ content: 'YmFzZTY0', encoding: 'base64', isBinary: true })
  })
})

describe('devServer.copyFile', () => {
  it('relays srcPath/destPath to fs.copyFile', async () => {
    const call = vi.fn().mockResolvedValue({ ok: true, path: '/work/image.png' })
    const result = await methodByName('devServer.copyFile').handler(
      { id: 'dev-1', srcPath: '/tmp/src.png', destPath: '/work/image.png' },
      contextWithRelay(call)
    )
    expect(result).toEqual({ path: '/work/image.png' })
    expect(call).toHaveBeenCalledWith(
      'fs.copyFile',
      { srcPath: '/tmp/src.png', destPath: '/work/image.png' },
      15_000
    )
  })
})
