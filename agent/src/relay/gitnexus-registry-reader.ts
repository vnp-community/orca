import fs from 'fs'
import path from 'path'
import os from 'os'
import { AgentConfig } from './agent-config'
import { CodeIntelError } from './codeintel-errors'

export type GitNexusRegistryEntry = {
  name?: string
  path: string
  storagePath?: string
  indexedAt?: number
  lastCommit?: string
  remoteUrl?: string
  stats?: any
  branch?: string
}

let cachedEntries: GitNexusRegistryEntry[] = []
let cacheMtimeMs = 0

export function readGitNexusRegistry(config: AgentConfig): GitNexusRegistryEntry[] {
  const home = config.toolEnv.HOME ?? os.homedir()
  const registryPath = path.join(home, '.gitnexus', 'registry.json')

  try {
    const stat = fs.statSync(registryPath)
    if (stat.mtimeMs === cacheMtimeMs && cachedEntries.length > 0) {
      return cachedEntries
    }

    const content = fs.readFileSync(registryPath, 'utf8')
    const parsed = JSON.parse(content)
    
    if (!Array.isArray(parsed)) {
      throw new Error('Registry is not an array')
    }

    for (const entry of parsed) {
      if (!entry || typeof entry.path !== 'string' || !path.isAbsolute(entry.path)) {
        throw new Error('Invalid registry entry path')
      }
    }

    cachedEntries = parsed
    cacheMtimeMs = stat.mtimeMs
    return cachedEntries
  } catch (err: any) {
    if (err.code === 'ENOENT') {
      return []
    }
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Failed to read gitnexus registry', { tool: 'gitnexus', reason: 'registry_unreadable' })
  }
}

export function findRegistryEntry(entries: GitNexusRegistryEntry[], realToplevel: string, log?: { warn: (msg: string) => void }): GitNexusRegistryEntry | undefined {
  let matched: GitNexusRegistryEntry[] = []

  for (const entry of entries) {
    try {
      if (fs.realpathSync.native(entry.path) === realToplevel) {
        matched.push(entry)
      }
    } catch {
      // Ignore invalid paths in registry
    }
  }

  if (matched.length === 0) return undefined
  
  if (matched.length > 1) {
    if (log) log.warn('registry_duplicate_path: ' + realToplevel)
    matched.sort((a, b) => (b.indexedAt || 0) - (a.indexedAt || 0))
  }

  return matched[0]
}
