import { z } from 'zod'
import {
  connectRegisteredSshTarget,
  getSshConnectionStore,
  getRegisteredSshState,
  listRegisteredAllConnectionStates,
  listRegisteredFilteredTargets,
  listRegisteredRemovedSshTargetLabels,
  listRegisteredSshProjects,
  listRegisteredSshTargets,
  listRegisteredSshTeams
} from '../../../ipc/ssh'
import { defineMethod, type RpcMethod } from '../core'
import { bootstrapServer } from '../../../ssh/fleet-bootstrap-service.js'
import { getFleetStatus } from '../../../ssh/fleet-status-service.js'
import { fleetHealthStore } from '../../../ssh/fleet-health-store.js'

const SshTarget = z.object({
  targetId: z.string().min(1)
})

export const SSH_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'ssh.getState',
    params: SshTarget,
    handler: (params) => ({ state: getRegisteredSshState(params.targetId) ?? null })
  }),
  defineMethod({
    name: 'ssh.connect',
    params: SshTarget,
    handler: async (params) => ({ state: await connectRegisteredSshTarget(params.targetId) })
  }),
  defineMethod({
    name: 'ssh.listTargets',
    params: null,
    handler: () => ({ targets: listRegisteredSshTargets() })
  }),
  defineMethod({
    name: 'ssh.listRemovedTargetLabels',
    params: null,
    handler: () => ({ labels: listRegisteredRemovedSshTargetLabels() })
  }),
  // Fleet metadata methods
  defineMethod({
    name: 'ssh.listProjects',
    params: null,
    handler: () => ({ projects: listRegisteredSshProjects() })
  }),
  defineMethod({
    name: 'ssh.listTeams',
    params: null,
    handler: () => ({ teams: listRegisteredSshTeams() })
  }),
  // Fleet query methods
  defineMethod({
    name: 'ssh.filterTargets',
    params: z.object({
      project: z.string().optional(),
      team: z.string().optional(),
      environment: z.enum(['development', 'staging', 'production']).optional(),
      tags: z.array(z.string()).optional(),
      search: z.string().optional(),
    }).nullable(),
    handler: (params) => ({ targets: listRegisteredFilteredTargets(params ?? {}) })
  }),
  defineMethod({
    name: 'ssh.getAllConnectionStates',
    params: null,
    handler: () => ({ states: listRegisteredAllConnectionStates() })
  }),
  // Fleet bootstrap
  defineMethod({
    name: 'ssh.bootstrapServer',
    params: z.object({
      targetId: z.string().min(1),
      fleetConfigPath: z.string().optional(),
      skipNodeInstall: z.boolean().optional(),
      skipGitInstall: z.boolean().optional(),
      skipRepoClone: z.boolean().optional(),
      skipSetupScript: z.boolean().optional(),
      nodeVersion: z.string().optional(),
    }),
    handler: async (params) => {
      const result = await bootstrapServer(params.targetId, {
        fleetConfigPath: params.fleetConfigPath,
        skipNodeInstall: params.skipNodeInstall,
        skipGitInstall: params.skipGitInstall,
        skipRepoClone: params.skipRepoClone,
        skipSetupScript: params.skipSetupScript,
        nodeVersion: params.nodeVersion,
      })
      return { result }
    }
  }),
  // Fleet import
  defineMethod({
    name: 'ssh.importFleetConfig',
    params: z.object({ filePath: z.string().min(1) }),
    handler: async (params) => {
      const store = getSshConnectionStore()
      if (!store) {throw new Error('SSH store not initialized')}
      const result = await store.importFromFleetConfig(params.filePath)
      return { result }
    }
  }),
  // Fleet status / health
  defineMethod({
    name: 'ssh.getFleetStatus',
    params: z.object({
      project: z.string().optional(),
      team: z.string().optional(),
    }).nullable(),
    handler: async (params) => {
      return { report: getFleetStatus(params ?? {}) }
    }
  }),
  defineMethod({
    name: 'ssh.getUptimeHistory',
    params: z.object({
      targetId: z.string().min(1),
      windowMs: z.number().optional(),
    }),
    handler: async (params) => {
      return { uptime: fleetHealthStore.getUptimeForTarget(params.targetId, params.windowMs) }
    }
  })
]
