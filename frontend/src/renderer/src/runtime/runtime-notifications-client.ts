// Why: mirrors runtime-git-client.ts's local-vs-environment routing. Desktop
// notifications are physically tied to this machine, so the 'local' branch
// is unchanged from the existing direct window.api.notifications.* calls
// (zero behavior change for the common case). The 'environment' branch is
// new capability — routes over the generic RPC channel to
// desktop/src/main/runtime/rpc/methods/notifications.ts's dispatch/
// playSound/getPermissionStatus/openSystemSettings/probeDelivery methods,
// which previously had no way to be reached at all.
import type {
  NotificationDeliveryProbeResult,
  NotificationDispatchRequest,
  NotificationDispatchResult,
  NotificationPermissionStatusResult,
  NotificationSoundDataResult
} from '../../../shared/types'
import { callRuntimeRpc, type RuntimeClientTarget } from './runtime-rpc-client'

const LOCAL_TARGET: RuntimeClientTarget = { kind: 'local' }

export async function dispatchRuntimeNotification(
  args: NotificationDispatchRequest,
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<NotificationDispatchResult> {
  if (target.kind === 'local') {
    return window.api.notifications.dispatch(args)
  }
  return callRuntimeRpc<NotificationDispatchResult>(target, 'notifications.dispatch', args)
}

export async function getRuntimeNotificationPermissionStatus(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<NotificationPermissionStatusResult> {
  if (target.kind === 'local') {
    return window.api.notifications.getPermissionStatus()
  }
  return callRuntimeRpc<NotificationPermissionStatusResult>(
    target,
    'notifications.getPermissionStatus'
  )
}

export async function openRuntimeNotificationSystemSettings(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<void> {
  if (target.kind === 'local') {
    await window.api.notifications.openSystemSettings()
    return
  }
  await callRuntimeRpc<void>(target, 'notifications.openSystemSettings')
}

export async function probeRuntimeNotificationDelivery(
  args?: { force?: boolean },
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<NotificationDeliveryProbeResult> {
  if (target.kind === 'local') {
    return window.api.notifications.probeDelivery(args)
  }
  return callRuntimeRpc<NotificationDeliveryProbeResult>(target, 'notifications.probeDelivery', args)
}

// Why: unlike window.api.notifications.playSound() — which decodes and plays
// audio through a preload-owned Audio element that only exists for the
// local desktop — the RPC method can only hand back the resolved sound
// bytes (data/mimeType/path). There is no environment-target equivalent of
// "play a sound on the local desktop" over RPC today, so this is exposed
// separately from the window.api playback helper rather than replacing it.
export async function loadRuntimeNotificationSoundData(
  target: RuntimeClientTarget
): Promise<NotificationSoundDataResult> {
  return callRuntimeRpc<NotificationSoundDataResult>(target, 'notifications.playSound')
}
