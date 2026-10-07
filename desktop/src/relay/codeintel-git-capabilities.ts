import { GitCapabilityCache } from '../shared/git-capability-cache'

let cache: GitCapabilityCache | undefined

export function getCodeIntelGitCapabilities(): GitCapabilityCache {
  if (!cache) {
    cache = new GitCapabilityCache()
  }
  return cache
}
