import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { CodeIntelIndexWatcher } from './codeintel-index-watcher'
import * as NotificationSink from './codeintel-notification-sink'
import * as IndexBasisProbe from './codeintel-index-basis-probe'
import fs from 'fs'

vi.mock('./codeintel-notification-sink', () => ({
  emitCodeIntelNotification: vi.fn()
}))

vi.mock('./codeintel-index-basis-probe', () => ({
  probeIndexBasis: vi.fn()
}))

vi.mock('fs', async () => {
  const actual = await vi.importActual<any>('fs')
  return {
    ...actual,
    existsSync: vi.fn().mockReturnValue(true),
    watch: vi.fn().mockImplementation((path, cb) => {
      // Return a mock watcher
      return { close: vi.fn() }
    })
  }
})

describe('codeintel-index-watcher', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('notifies with correct payload without mergeBase', async () => {
    const watcher = new CodeIntelIndexWatcher('/repo', '/repo/.gitnexus')
    
    vi.mocked(IndexBasisProbe.probeIndexBasis).mockResolvedValue({
      headCommit: 'abcdef', headCommitTimeMs: 123,
      mergeBase: null, mergeBaseCommitTimeMs: null,
      changedFilesNotInIndex: 0, dirtySinceIndex: false
    })

    // Manually trigger the private notify method to test payload
    await (watcher as any).notify('meta.json', 'gitnexus')

    expect(NotificationSink.emitCodeIntelNotification).toHaveBeenCalledWith('codeintel.indexChanged', {
      reason: 'meta.json',
      tool: 'gitnexus',
      workspaceRoot: '/repo',
      indexScope: 'exact'
    })
    
    const call = vi.mocked(NotificationSink.emitCodeIntelNotification).mock.calls[0][1]
    expect(call).not.toHaveProperty('mergeBase')
    expect(call).not.toHaveProperty('trigger')
  })

  it('notifies with mergeBase', async () => {
    const watcher = new CodeIntelIndexWatcher('/repo', '/repo/.gitnexus')
    
    vi.mocked(IndexBasisProbe.probeIndexBasis).mockResolvedValue({
      headCommit: 'abcdef', headCommitTimeMs: 123,
      mergeBase: '123456', mergeBaseCommitTimeMs: 123,
      changedFilesNotInIndex: 0, dirtySinceIndex: false
    })

    await (watcher as any).notify('meta.json', 'gitnexus')

    expect(NotificationSink.emitCodeIntelNotification).toHaveBeenCalledWith('codeintel.indexChanged', {
      reason: 'meta.json',
      tool: 'gitnexus',
      workspaceRoot: '/repo',
      indexScope: 'exact',
      mergeBase: '123456'
    })
  })
})
