import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  deriveAddRepoHostGate,
  isAddRepoHostGateBlocking,
  pickAutoSelectedWebHostId,
  readRememberedWebHostId,
  rememberWebHostId
} from './add-repo-web-host-gate'

const ds1 = 'devServer:ds-1' as const
const ds2 = 'devServer:ds-2' as const

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('pickAutoSelectedWebHostId', () => {
  it('picks the only connected host, nothing when there are none', () => {
    expect(pickAutoSelectedWebHostId([ds1], null)).toBe(ds1)
    expect(pickAutoSelectedWebHostId([], null)).toBeNull()
  })

  it('with several hosts only applies a remembered host that is still connected', () => {
    expect(pickAutoSelectedWebHostId([ds1, ds2], null)).toBeNull()
    expect(pickAutoSelectedWebHostId([ds1, ds2], ds2)).toBe(ds2)
    expect(pickAutoSelectedWebHostId([ds1, ds2], 'devServer:gone')).toBeNull()
  })
})

describe('deriveAddRepoHostGate', () => {
  const servers = [
    { id: 'ds-1', name: 'Mac Studio', status: 'connected' as const },
    { id: 'ds-2', name: 'Build Box', status: 'disconnected' as const },
    { id: 'ds-3', name: 'Flaky', status: 'connecting' as const },
    { id: 'ds-4', name: 'Broken', status: 'error' as const }
  ]

  it('is blocked with no host when nothing is connected and nothing chosen', () => {
    const gate = deriveAddRepoHostGate({ chosenHostId: null, connectedHostIds: [], devServers: [] })
    expect(gate).toEqual({ status: 'no-host' })
    expect(isAddRepoHostGateBlocking(gate)).toBe(true)
  })

  it('asks to choose when several hosts are connected and none is chosen', () => {
    expect(
      deriveAddRepoHostGate({
        chosenHostId: null,
        connectedHostIds: [ds1, ds2],
        devServers: servers
      })
    ).toEqual({ status: 'choose-host' })
  })

  it('is ready for a chosen connected host', () => {
    const gate = deriveAddRepoHostGate({
      chosenHostId: ds1,
      connectedHostIds: [ds1],
      devServers: servers
    })
    expect(gate).toEqual({ status: 'ready' })
    expect(isAddRepoHostGateBlocking(gate)).toBe(false)
  })

  it.each([
    ['devServer:ds-2', 'disconnected', 'Build Box'],
    ['devServer:ds-3', 'connecting', 'Flaky'],
    ['devServer:ds-4', 'error', 'Broken'],
    ['devServer:ds-9', 'removed', 'ds-9']
  ] as const)('keeps a dropped chosen host (%s) as unavailable: %s', (hostId, reason, label) => {
    expect(
      deriveAddRepoHostGate({ chosenHostId: hostId, connectedHostIds: [ds1], devServers: servers })
    ).toEqual({ status: 'host-unavailable', hostId, label, reason })
  })
})

describe('remembered host', () => {
  it('round-trips through localStorage and tolerates it being unavailable', () => {
    const store = new Map<string, string>()
    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, value)
      }
    })
    expect(readRememberedWebHostId()).toBeNull()
    rememberWebHostId(ds2)
    expect(readRememberedWebHostId()).toBe(ds2)

    vi.stubGlobal('window', {
      localStorage: {
        getItem: () => {
          throw new Error('blocked')
        },
        setItem: () => {
          throw new Error('blocked')
        }
      }
    })
    expect(readRememberedWebHostId()).toBeNull()
    expect(() => rememberWebHostId(ds1)).not.toThrow()
  })
})
