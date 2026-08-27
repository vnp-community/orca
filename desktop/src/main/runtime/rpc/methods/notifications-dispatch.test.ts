import { describe, expect, it, vi } from 'vitest'

// Why: avoid importing RpcDispatcher — it eagerly imports the full
// ALL_RPC_METHODS aggregator (every namespace file), which transitively
// pulls in Electron-toolkit modules (app-icon.ts et al.) that this sandbox's
// electron CJS/ESM interop can't load standalone. Invoking the method
// handler directly keeps this suite scoped to notifications.ts's own
// dependency graph. Mock shape mirrors ipc/notifications.test.ts, which
// exercises the same underlying dispatchNotification/probe/loadSound
// functions via the ipcMain handlers.
const { notificationCtorMock, notificationIsSupportedMock, getAllWindowsMock, shellOpenExternalMock } =
  vi.hoisted(() => ({
    notificationCtorMock: vi.fn(() => ({
      on: vi.fn(),
      once: vi.fn(),
      removeListener: vi.fn(),
      show: vi.fn(),
      close: vi.fn()
    })),
    notificationIsSupportedMock: vi.fn(() => true),
    getAllWindowsMock: vi.fn(() => []),
    shellOpenExternalMock: vi.fn()
  }))

vi.mock('electron', () => ({
  ipcMain: { removeHandler: vi.fn(), handle: vi.fn() },
  Notification: Object.assign(notificationCtorMock, { isSupported: notificationIsSupportedMock }),
  BrowserWindow: { getAllWindows: getAllWindowsMock },
  app: { focus: vi.fn() },
  shell: { openExternal: shellOpenExternalMock }
}))

const { readAuthorizationStatusMock } = vi.hoisted(() => ({
  readAuthorizationStatusMock: vi.fn(
    (): Promise<'authorized' | 'denied' | 'not-determined' | 'unknown' | null> =>
      Promise.resolve(null)
  )
}))
vi.mock('../../../ipc/notification-authorization-status', () => ({
  readNotificationAuthorizationStatus: readAuthorizationStatusMock
}))

// Why: notifications.ts pulls in the tray module (for the minimized
// attention dot), which transitively loads app-icon/electron-toolkit; stub
// it so this suite stays focused on notification RPC dispatch (same reason
// ipc/notifications.test.ts stubs it).
vi.mock('../../../tray/system-tray', () => ({ setTrayAttention: vi.fn() }))

import type { RpcMethod } from '../core'
import { NOTIFICATION_METHODS } from './notifications'
import type { NotificationSettings } from '../../../../shared/types'

function findMethod(name: string): RpcMethod {
  const method = NOTIFICATION_METHODS.find((m) => m.name === name)
  if (!method || 'stream' in method) {
    throw new Error(`Expected a non-streaming method named ${name}`)
  }
  return method
}

function disabledSettings(): NotificationSettings {
  return {
    enabled: false,
    agentTaskComplete: true,
    terminalBell: true,
    suppressWhenFocused: false,
    customSoundId: 'system',
    customSoundPath: null,
    customSoundVolume: 100
  }
}

describe('notifications RPC methods (outbound trigger direction)', () => {
  it('notifications.dispatch delegates to the same gating logic as the ipcMain handler', async () => {
    const method = findMethod('notifications.dispatch')
    const getNotificationSettings = vi.fn(disabledSettings)
    const params = method.params!.parse({ source: 'test' })

    const result = await method.handler(params, {
      runtime: { getRuntimeId: () => 'test-runtime', getNotificationSettings } as never
    })

    expect(getNotificationSettings).toHaveBeenCalledTimes(1)
    expect(result).toEqual({ delivered: false, reason: 'disabled' })
  })

  it('notifications.getPermissionStatus reads notificationPermissionRequested off runtime UI state', async () => {
    const method = findMethod('notifications.getPermissionStatus')
    notificationIsSupportedMock.mockReturnValue(true)

    const result = await method.handler(undefined, {
      runtime: {
        getRuntimeId: () => 'test-runtime',
        getUIState: () => ({ notificationPermissionRequested: true })
      } as never
    })

    expect(result).toMatchObject({ requested: true })
  })

  it('notifications.openSystemSettings resolves without throwing', () => {
    const method = findMethod('notifications.openSystemSettings')

    expect(() =>
      method.handler(undefined, { runtime: { getRuntimeId: () => 'test-runtime' } as never })
    ).not.toThrow()
  })

  it('notifications.probeDelivery reports unsupported off darwin', async () => {
    const method = findMethod('notifications.probeDelivery')
    const params = method.params!.parse({ force: true })
    const updateUIState = vi.fn()

    const result = await method.handler(params, {
      runtime: {
        getRuntimeId: () => 'test-runtime',
        getUIState: () => ({ notificationPermissionRequested: false }),
        updateUIState
      } as never
    })

    if (process.platform !== 'darwin') {
      expect(result).toEqual({ state: 'unsupported', authoritative: false })
    }
  })

  it('notifications.playSound surfaces the missing-path reason when no sound is selected', async () => {
    const method = findMethod('notifications.playSound')

    const result = await method.handler(undefined, {
      runtime: {
        getRuntimeId: () => 'test-runtime',
        getNotificationSettings: disabledSettings
      } as never
    })

    expect(result).toEqual({ ok: false, reason: 'missing-path' })
  })
})
