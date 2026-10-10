import type { DirEntry } from '../../../shared/types'
import { callRuntimeRpc } from './runtime-rpc-client'

export type RuntimeServerDirectoryListing = {
  resolvedPath: string
  entries: DirEntry[]
  // Present once the backend caps listings (limit/query); older backends omit them.
  truncated?: boolean
  totalEntries?: number
}

export type RuntimeServerDirectoryBrowseOptions = {
  limit?: number
  query?: string
}

export async function browseRuntimeServerDirectory(
  environmentId: string,
  path: string,
  options: RuntimeServerDirectoryBrowseOptions = {}
): Promise<RuntimeServerDirectoryListing> {
  return callRuntimeRpc<RuntimeServerDirectoryListing>(
    { kind: 'environment', environmentId },
    'files.browseServerDir',
    { path, ...options },
    { timeoutMs: 15_000 }
  )
}
