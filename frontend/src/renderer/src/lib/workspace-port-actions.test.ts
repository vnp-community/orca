import { describe, expect, it } from 'vitest'
import { mergeWorkspacePortScans } from './workspace-port-actions'
import type { WorkspacePort, WorkspacePortScanResult } from '../../../shared/workspace-ports'

function makePort(id: string, port: number): WorkspacePort {
  return {
    id,
    kind: 'external',
    bindHost: '0.0.0.0',
    connectHost: '127.0.0.1',
    port,
    protocol: 'http'
  }
}

function makeScan(ports: WorkspacePort[], scannedAt = 1): WorkspacePortScanResult {
  return { platform: 'linux', scannedAt, ports }
}

describe('mergeWorkspacePortScans', () => {
  it('returns null when there are no scans', () => {
    expect(mergeWorkspacePortScans({})).toBeNull()
  })

  it('returns the single scan unchanged when only one exists', () => {
    const scan = makeScan([makePort('tcp:3000', 3000)])
    expect(mergeWorkspacePortScans({ local: scan })).toBe(scan)
  })

  it('merges multiple scans, prefixing port ids by scan key', () => {
    const result = mergeWorkspacePortScans({
      local: makeScan([makePort('tcp:3000', 3000)], 1),
      'runtime:env-1': makeScan([makePort('tcp:4000', 4000)], 2)
    })
    expect(result?.ports.map((p) => p.id)).toEqual(['local:tcp:3000', 'runtime:env-1:tcp:4000'])
    expect(result?.scannedAt).toBe(2)
  })

  // Regression test: a scan result whose own fetch failed/hasn't completed
  // can carry `ports` as null/undefined at runtime despite
  // WorkspacePortScanResult declaring it a required array — found live
  // crashing the whole Ports panel (`Cannot read properties of null
  // (reading 'map')`) on the first scan triggered right after creating a
  // project.
  it('tolerates a scan result with ports missing instead of throwing', () => {
    const brokenScan = { platform: 'linux', scannedAt: 1 } as unknown as WorkspacePortScanResult
    const result = mergeWorkspacePortScans({
      local: makeScan([makePort('tcp:3000', 3000)]),
      broken: brokenScan
    })
    expect(result?.ports.map((p) => p.id)).toEqual(['local:tcp:3000'])
  })
})
