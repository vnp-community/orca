import { describe, expect, it } from 'vitest'
import { discoverIosToolchain, type ExecFileFn } from './device-ios-discovery'

function fakeExecFile(shouldSucceed: boolean): ExecFileFn {
  return ((_cmd: string, _args: string[], _opts: unknown, callback: (error: Error | null) => void) => {
    callback(shouldSucceed ? null : new Error('not found'))
  }) as unknown as ExecFileFn
}

describe('discoverIosToolchain', () => {
  it('reports unavailable on non-darwin hosts without shelling out', async () => {
    const result = await discoverIosToolchain(fakeExecFile(true), 'linux')
    expect(result.simctlOk).toBe(false)
    expect(result.message).toContain('macOS')
  })

  it('reports ok when xcrun -find simctl succeeds on darwin', async () => {
    const result = await discoverIosToolchain(fakeExecFile(true), 'darwin')
    expect(result.simctlOk).toBe(true)
  })

  it('reports unavailable with an actionable message when simctl is missing on darwin', async () => {
    const result = await discoverIosToolchain(fakeExecFile(false), 'darwin')
    expect(result.simctlOk).toBe(false)
    expect(result.message).toContain('Xcode')
  })
})
