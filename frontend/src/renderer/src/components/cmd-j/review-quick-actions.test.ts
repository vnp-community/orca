import { describe, expect, it, vi } from 'vitest'
import { getCmdJQuickActions } from './quick-actions'
import {
  getUnavailableQuickActionMessage,
  type CmdJQuickActionContext
} from './quick-action-context'

function ctx(overrides: Partial<CmdJQuickActionContext> = {}): CmdJQuickActionContext {
  return {
    activeView: 'terminal',
    activeWorktreeId: 'wt',
    activeWorktree: null,
    isLoading: false,
    sshStatus: null,
    runtimeMode: 'local-desktop',
    activeGroupId: 'g',
    openNewBrowserTab: async () => {},
    openNewMarkdownFile: async () => {},
    openNewTerminalTab: async () => {},
    openCreateWorkspace: () => {},
    deleteActiveWorkspace: () => {},
    openAddQuickCommand: () => {},
    codeIntelEnabled: true,
    reviewLensAvailable: () => true,
    openReviewChanges: () => {},
    ...overrides
  }
}

const action = (id: string) => getCmdJQuickActions().find((a) => a.id === id)!

describe('review quick actions', () => {
  it('exposes the three actions', () => {
    for (const id of ['review-changes', 'open-architecture-map', 'open-erd']) {
      expect(action(id)).toBeDefined()
    }
  })

  it('is available only when workspace, flag and lens allow', () => {
    const a = action('review-changes')
    expect(a.isAvailable(ctx())).toEqual({ available: true })
    expect(a.isAvailable(ctx({ isLoading: true }))).toEqual({ available: false, reason: 'loading' })
    expect(a.isAvailable(ctx({ sshStatus: 'disconnected' }))).toEqual({
      available: false,
      reason: 'ssh-disconnected'
    })
    expect(a.isAvailable(ctx({ codeIntelEnabled: false }))).toEqual({
      available: false,
      reason: 'code-intel-disabled'
    })
    expect(a.isAvailable(ctx({ reviewLensAvailable: () => false }))).toEqual({
      available: false,
      reason: 'code-intel-disabled'
    })
    expect(a.isAvailable(ctx({ codeIntelEnabled: undefined }))).toMatchObject({ available: false })
  })

  it('opens Review with the right lens and rechecks at run time', async () => {
    const open = vi.fn()
    await expect(action('open-erd').run(ctx({ openReviewChanges: open }))).resolves.toEqual({
      status: 'ok'
    })
    expect(open).toHaveBeenCalledWith({ lens: 'erd' })
    await action('open-architecture-map').run(ctx({ openReviewChanges: open }))
    expect(open).toHaveBeenLastCalledWith({ lens: 'architecture' })
    const blocked = await action('review-changes').run(
      ctx({ codeIntelEnabled: false, openReviewChanges: open })
    )
    expect(blocked).toEqual({ status: 'unavailable', reason: 'code-intel-disabled' })
    expect(open).toHaveBeenCalledTimes(2)
  })

  it('explains the disabled reason', () => {
    expect(getUnavailableQuickActionMessage('Review Changes', 'code-intel-disabled')).toContain(
      "isn't enabled"
    )
  })
})
