import { createHash } from 'crypto'

export type CacheKeyProps = {
  registryPath: string
  versionMarker: string
  method: string
  params: Record<string, any>
}

export type CachePayload = {
  data: any
  totalCount?: number
  truncated?: boolean
  warnings?: string[]
}

export function generateCacheKey(props: CacheKeyProps): string {
  const sortedParams = JSON.stringify(props.params, Object.keys(props.params).sort())
  const hash = createHash('sha256').update(sortedParams).digest('hex')
  return `${props.registryPath}:${props.versionMarker}:${props.method}:${hash}`
}

type CacheEntry = {
  payload: CachePayload
  sizeBytes: number
  expiresAt: number
}

const MAX_ITEMS = 64
const MAX_BYTES = 32 * 1024 * 1024
const TTL_MS = 60 * 1000

class ShortLivedCache {
  private store = new Map<string, CacheEntry>()
  private totalBytes = 0
  private pendingComputes = new Map<string, Promise<CachePayload>>()

  async getOrCompute(
    key: string,
    compute: () => Promise<CachePayload>,
    opts: { cacheable: boolean }
  ): Promise<CachePayload> {
    if (!opts.cacheable) {
      return compute()
    }

    const now = Date.now()
    const entry = this.store.get(key)
    if (entry) {
      if (entry.expiresAt > now) {
        this.store.delete(key)
        this.store.set(key, entry)
        return JSON.parse(JSON.stringify(entry.payload))
      } else {
        this.deleteEntry(key)
      }
    }

    const pending = this.pendingComputes.get(key)
    if (pending) {
      return JSON.parse(JSON.stringify(await pending))
    }

    const promise = (async () => {
      try {
        const result = await compute()
        this.setEntry(key, result, now)
        return result
      } finally {
        this.pendingComputes.delete(key)
      }
    })()

    this.pendingComputes.set(key, promise)
    return JSON.parse(JSON.stringify(await promise))
  }

  private setEntry(key: string, payload: CachePayload, now: number) {
    const sizeBytes = Buffer.byteLength(JSON.stringify(payload), 'utf8')
    
    if (sizeBytes > MAX_BYTES) {
      return
    }

    while (this.store.size >= MAX_ITEMS || this.totalBytes + sizeBytes > MAX_BYTES) {
      const oldestKey = this.store.keys().next().value
      if (oldestKey === undefined) break
      this.deleteEntry(oldestKey)
    }

    this.store.set(key, {
      payload,
      sizeBytes,
      expiresAt: now + TTL_MS
    })
    this.totalBytes += sizeBytes
  }

  private deleteEntry(key: string) {
    const entry = this.store.get(key)
    if (entry) {
      this.totalBytes -= entry.sizeBytes
      this.store.delete(key)
    }
  }

  invalidate(registryPath?: string) {
    if (!registryPath) {
      this.store.clear()
      this.totalBytes = 0
      return
    }
    const prefix = `${registryPath}:`
    for (const key of this.store.keys()) {
      if (key.startsWith(prefix)) {
        this.deleteEntry(key)
      }
    }
  }
}

export const codeIntelCache = new ShortLivedCache()

export function invalidateShortLivedCache(registryPath?: string) {
  codeIntelCache.invalidate(registryPath)
}
