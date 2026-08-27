import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import type {
  DeveloperPermissionId,
  DeveloperPermissionRequestResult,
  DeveloperPermissionState
} from '../../../../shared/developer-permissions-types'
import {
  DEVELOPER_PERMISSION_IDS,
  getPermissionState,
  openPrivacyPane,
  requestPermission
} from '../../../ipc/developer-permissions'

const PermissionIdSchema = z.enum([
  'microphone',
  'camera',
  'screen',
  'accessibility',
  'full-disk-access',
  'automation',
  'local-network',
  'usb',
  'bluetooth'
])

const RequestPermission = z.object({ id: PermissionIdSchema })
const OpenSettings = z.object({ id: PermissionIdSchema })

// Why: reuses the exact getPermissionState/requestPermission/openPrivacyPane
// functions desktop/src/main/ipc/developer-permissions.ts's ipcMain handlers
// call — same macOS TCC probing and prompting, not a reimplementation.
export const DEVELOPER_PERMISSIONS_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'developerPermissions.getStatus',
    params: null,
    handler: async (): Promise<DeveloperPermissionState[]> => {
      return Promise.all(DEVELOPER_PERMISSION_IDS.map(getPermissionState))
    }
  }),
  defineMethod({
    name: 'developerPermissions.request',
    params: RequestPermission,
    handler: async (params): Promise<DeveloperPermissionRequestResult> => {
      const id = params.id as DeveloperPermissionId
      if (!DEVELOPER_PERMISSION_IDS.includes(id)) {
        return { id, status: 'unsupported', openedSystemSettings: false }
      }
      return requestPermission(id)
    }
  }),
  defineMethod({
    name: 'developerPermissions.openSettings',
    params: OpenSettings,
    handler: async (params): Promise<void> => {
      await openPrivacyPane(params.id as DeveloperPermissionId)
    }
  })
]
