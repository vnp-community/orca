/**
 * Regression test: verify preload/index.ts has not been modified
 * by web mode implementation tasks.
 */
import { describe, it, expect } from 'vitest'
import { readFileSync, statSync } from 'node:fs'

describe('Electron preload — unchanged', () => {
  it('preload/index.ts still exists and is readable', () => {
    expect(() => {
      statSync('src/preload/index.ts')
    }).not.toThrow()
  })

  it('preload/index.ts still uses contextBridge', () => {
    const src = readFileSync('src/preload/index.ts', 'utf-8')
    expect(src).toContain('contextBridge')
  })

  it('preload/index.ts still uses ipcRenderer', () => {
    const src = readFileSync('src/preload/index.ts', 'utf-8')
    expect(src).toContain('ipcRenderer')
  })
})
