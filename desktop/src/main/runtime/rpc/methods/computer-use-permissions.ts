import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import type {
  ComputerUsePermissionId,
  ComputerUsePermissionResetResult,
  ComputerUsePermissionSetupResult,
  ComputerUsePermissionStatusResult
} from '../../../../shared/computer-use-permissions-types'

const OpenSetup = z.object({ id: z.enum(['accessibility', 'screenshots']).optional() }).optional()

// Why: mirrors desktop/src/main/ipc/computer-use-permissions.ts's three
// handlers exactly — same dynamic import of the macOS-only helper module, no
// reimplemented permission logic.
export const COMPUTER_USE_PERMISSIONS_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'computerUsePermissions.getStatus',
    params: null,
    handler: async (): Promise<ComputerUsePermissionStatusResult> => {
      const { getComputerUsePermissionStatus } = await import(
        '../../../computer/macos-computer-use-permissions'
      )
      return getComputerUsePermissionStatus()
    }
  }),
  defineMethod({
    name: 'computerUsePermissions.openSetup',
    params: OpenSetup,
    handler: async (params): Promise<ComputerUsePermissionSetupResult> => {
      const { openComputerUsePermissions } = await import(
        '../../../computer/macos-computer-use-permissions'
      )
      return openComputerUsePermissions(params?.id as ComputerUsePermissionId | undefined)
    }
  }),
  defineMethod({
    name: 'computerUsePermissions.reset',
    params: null,
    handler: async (): Promise<ComputerUsePermissionResetResult> => {
      const { resetComputerUsePermissions } = await import(
        '../../../computer/macos-computer-use-permissions'
      )
      return resetComputerUsePermissions()
    }
  })
]
