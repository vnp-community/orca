import { describe, expect, it } from 'vitest'
import { parseExecutionHostId } from '../../../../shared/execution-host'
import { deriveAddRepoHostRouting } from './add-repo-host-routing'

const hostOptions = [
  { id: 'devServer:ds-1', label: 'Mac Studio' },
  { id: 'ssh:t1', label: 'Build SSH' },
  { id: 'runtime:env-1', label: 'Runtime' },
  { id: 'local', label: 'This computer' }
]

function route(hostId: string | null): ReturnType<typeof deriveAddRepoHostRouting> {
  return deriveAddRepoHostRouting({
    selectedHostId: hostId as never,
    selectedParsedHost: parseExecutionHostId(hostId),
    hostOptions
  })
}

describe('deriveAddRepoHostRouting', () => {
  it('routes a dev server host to the host path step', () => {
    expect(route('devServer:ds-1')).toMatchObject({
      devServerId: 'ds-1',
      runtimeEnvironmentId: null,
      browseTarget: 'server-path',
      browseHostKind: 'runtime',
      manualCreateParentEntry: true,
      selectedHostLabel: 'Mac Studio'
    })
  })

  it('routes a runtime environment host to the host path step', () => {
    expect(route('runtime:env-1')).toMatchObject({
      runtimeEnvironmentId: 'env-1',
      isRuntimeEnvironmentActive: true,
      browseTarget: 'server-path',
      manualCreateParentEntry: true
    })
  })

  it('routes an SSH host to the SSH picker', () => {
    expect(route('ssh:t1')).toMatchObject({
      browseTarget: 'ssh',
      browseHostKind: 'ssh',
      manualCreateParentEntry: true
    })
  })

  it('keeps the OS folder picker for the local host (desktop)', () => {
    expect(route('local')).toMatchObject({
      browseTarget: 'local',
      browseHostKind: 'local',
      manualCreateParentEntry: false,
      devServerId: null,
      runtimeEnvironmentId: null
    })
  })

  it('has no label before a host is chosen', () => {
    expect(route(null).selectedHostLabel).toBeNull()
  })
})
