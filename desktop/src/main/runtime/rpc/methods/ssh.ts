import { z } from 'zod'
import {
  addRegisteredSshPortForward,
  connectRegisteredSshTarget,
  disconnectRegisteredSshTarget,
  getSshConnectionStore,
  getRegisteredSshState,
  getRegisteredSshTarget,
  listRegisteredAllConnectionStates,
  listRegisteredFilteredTargets,
  listRegisteredRemovedSshTargetLabels,
  listRegisteredSshDetectedPorts,
  listRegisteredSshPortForwards,
  listRegisteredSshProjects,
  listRegisteredSshTargets,
  listRegisteredSshTeams,
  needsRegisteredSshPassphrasePrompt,
  removeRegisteredSshPortForward,
  updateRegisteredSshPortForward
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
    name: 'ssh.disconnect',
    params: SshTarget,
    handler: async (params): Promise<void> => {
      await disconnectRegisteredSshTarget(params.targetId)
    }
  }),
  // Why: lets runtime RPC callers (paired/remote clients) avoid surprising the
  // user with an unprompted credential dialog before auto-firing ssh.connect —
  // mirrors desktop's ssh:needsPassphrasePrompt IPC handler.
  defineMethod({
    name: 'ssh.needsPassphrasePrompt',
    params: SshTarget,
    handler: (params) => ({ needsPrompt: needsRegisteredSshPassphrasePrompt(params.targetId) })
  }),
  // ── Port forwarding ──────────────────────────────────────────────────────
  // Why: wraps the same SshPortForwardManager capability desktop's IPC surface
  // exposes, so paired/remote clients get real port-forward CRUD instead of the
  // web client's "unavailable" stubs.
  defineMethod({
    name: 'ssh.addPortForward',
    params: z.object({
      targetId: z.string().min(1),
      localPort: z.number(),
      remoteHost: z.string().min(1),
      remotePort: z.number(),
      label: z.string().optional()
    }),
    handler: async (params) => ({ entry: await addRegisteredSshPortForward(params) })
  }),
  defineMethod({
    name: 'ssh.updatePortForward',
    params: z.object({
      id: z.string().min(1),
      targetId: z.string().min(1),
      localPort: z.number(),
      remoteHost: z.string().min(1),
      remotePort: z.number(),
      label: z.string().optional()
    }),
    handler: async (params) => ({ entry: await updateRegisteredSshPortForward(params) })
  }),
  defineMethod({
    name: 'ssh.removePortForward',
    params: z.object({ id: z.string().min(1) }),
    handler: async (params) => ({ entry: await removeRegisteredSshPortForward(params.id) })
  }),
  defineMethod({
    name: 'ssh.listPortForwards',
    params: z.object({ targetId: z.string().optional() }).nullable(),
    handler: (params) => ({ forwards: listRegisteredSshPortForwards(params?.targetId) })
  }),
  defineMethod({
    name: 'ssh.listDetectedPorts',
    params: SshTarget,
    handler: (params) => ({ ports: listRegisteredSshDetectedPorts(params.targetId) })
  }),
  defineMethod({
    name: 'ssh.listTargets',
    params: null,
    handler: () => ({ targets: listRegisteredSshTargets() })
  }),
  defineMethod({
    name: 'ssh.getUserAccount',
    params: z.object({ serverId: z.string().min(1) }),
    // Matches backend's ssh.getUserAccount: honestly reflects the target's real
    // configured username — desktop has no separate account-provisioning concept either.
    handler: (params) => {
      const target = getRegisteredSshTarget(params.serverId)
      return {
        linuxUsername: target?.username ?? null,
        provisioned: target !== undefined
      }
    }
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
