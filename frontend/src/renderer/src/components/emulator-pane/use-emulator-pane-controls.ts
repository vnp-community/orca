import { useCallback, useRef, useState } from 'react'
import { callRuntimeRpc } from '@/runtime/runtime-rpc-client'
import { resolveEmulatorPaneRuntimeTarget } from './emulator-pane-runtime-target'
import type { EmulatorDeviceVisualOrientation } from './emulator-device-frame-layout'
import type { EmulatorGesturePoint } from './emulator-screen-gesture'

export function useEmulatorPaneControls(worktreeId: string, onRotateSettled?: () => void) {
  const nextRotateOrientationRef = useRef<'landscape_left' | 'portrait'>('landscape_left')
  const visualOrientationEpochRef = useRef(0)
  const [visualOrientation, setVisualOrientation] =
    useState<EmulatorDeviceVisualOrientation>('portrait')

  const sendTap = useCallback(
    async (x: number, y: number) => {
      const { target, projectId } = resolveEmulatorPaneRuntimeTarget(worktreeId)
      await callRuntimeRpc(target, 'emulator.tap', { x, y, worktree: worktreeId, projectId })
    },
    [worktreeId]
  )

  const sendButton = useCallback(
    async (name: string) => {
      const { target, projectId } = resolveEmulatorPaneRuntimeTarget(worktreeId)
      await callRuntimeRpc(target, 'emulator.button', { name, worktree: worktreeId, projectId })
    },
    [worktreeId]
  )

  const sendGesture = useCallback(
    async (points: EmulatorGesturePoint[]) => {
      const { target, projectId } = resolveEmulatorPaneRuntimeTarget(worktreeId)
      await callRuntimeRpc(target, 'emulator.gesture', { points, worktree: worktreeId, projectId })
    },
    [worktreeId]
  )

  const sendRotate = useCallback(async () => {
    const orientation = nextRotateOrientationRef.current
    const epoch = visualOrientationEpochRef.current
    const { target, projectId } = resolveEmulatorPaneRuntimeTarget(worktreeId)
    await callRuntimeRpc(target, 'emulator.rotate', {
      orientation,
      worktree: worktreeId,
      projectId
    })
    if (visualOrientationEpochRef.current !== epoch) {
      return null
    }
    const nextVisualOrientation = orientation === 'landscape_left' ? 'landscape' : 'portrait'
    setVisualOrientation(nextVisualOrientation)
    nextRotateOrientationRef.current =
      orientation === 'landscape_left' ? 'portrait' : 'landscape_left'
    onRotateSettled?.()
    return nextVisualOrientation
  }, [onRotateSettled, worktreeId])

  const resetVisualOrientation = useCallback(() => {
    visualOrientationEpochRef.current += 1
    nextRotateOrientationRef.current = 'landscape_left'
    setVisualOrientation('portrait')
  }, [])

  return { sendTap, sendButton, sendGesture, sendRotate, visualOrientation, resetVisualOrientation }
}
