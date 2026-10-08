import { describe, expect, it } from 'vitest'
import { create } from 'zustand'
import { createReviewUiSlice, ERD_SERVICE_HISTORY_MAX, type ReviewUiSlice } from './review-ui'

function makeStore() {
  return create<ReviewUiSlice>()(
    (...a) =>
      createReviewUiSlice(
        ...(a as unknown as Parameters<typeof createReviewUiSlice>)
      ) as ReviewUiSlice
  )
}

describe('review-ui ERD + storage state', () => {
  it('setErdService keeps a back-stack and clears the table selection', () => {
    const s = makeStore()
    s.getState().setErdService('w', 'infra')
    s.getState().selectErdTable('w', 'infra.agents')
    s.getState().setErdService('w', 'auth')
    expect(s.getState().reviewUiByWorktree.w).toMatchObject({
      erdService: 'auth',
      erdServiceHistory: ['infra'],
      selectedErdTable: null
    })
    s.getState().goBackErdService('w')
    expect(s.getState().reviewUiByWorktree.w).toMatchObject({
      erdService: 'infra',
      erdServiceHistory: []
    })
    s.getState().goBackErdService('w')
    expect(s.getState().reviewUiByWorktree.w.erdService).toBe('infra')
  })

  it('does not push when the service is unchanged and bounds the history', () => {
    const s = makeStore()
    s.getState().setErdService('w', 'a')
    s.getState().setErdService('w', 'a')
    expect(s.getState().reviewUiByWorktree.w.erdServiceHistory ?? []).toEqual([])
    for (let i = 0; i < ERD_SERVICE_HISTORY_MAX + 5; i++) {
      s.getState().setErdService('w', `s${i}`)
    }
    expect(s.getState().reviewUiByWorktree.w.erdServiceHistory).toHaveLength(
      ERD_SERVICE_HISTORY_MAX
    )
  })

  it('setStorageEnv resets the node selection', () => {
    const s = makeStore()
    s.getState().selectStorageNode('w', 'store:pg')
    s.getState().setStorageEnv('w', 'prod')
    expect(s.getState().reviewUiByWorktree.w).toMatchObject({
      storageEnv: 'prod',
      selectedStorageNodeId: null
    })
  })
})
