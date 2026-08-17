import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import {
  getCliInstallStatus,
  getWslCliInstallStatus,
  installCli,
  installWslCli,
  removeCli,
  removeWslCli
} from '../../../ipc/cli'

const WslDistroArgs = z
  .object({
    distro: z.string().min(1).nullable().optional()
  })
  .optional()

export const CLI_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'cli.getInstallStatus',
    params: null,
    handler: () => getCliInstallStatus()
  }),
  defineMethod({
    name: 'cli.install',
    params: null,
    handler: () => installCli()
  }),
  defineMethod({
    name: 'cli.remove',
    params: null,
    handler: () => removeCli()
  }),
  defineMethod({
    name: 'cli.getWslInstallStatus',
    params: WslDistroArgs,
    handler: (params) => getWslCliInstallStatus(params ?? undefined)
  }),
  defineMethod({
    name: 'cli.installWsl',
    params: WslDistroArgs,
    handler: (params) => installWslCli(params ?? undefined)
  }),
  defineMethod({
    name: 'cli.removeWsl',
    params: WslDistroArgs,
    handler: (params) => removeWslCli(params ?? undefined)
  })
]
