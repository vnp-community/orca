import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { codeIntelCache, invalidateShortLivedCache, generateCacheKey } from './codeintel-short-lived-cache'

describe('codeintel-short-lived-cache', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    invalidateShortLivedCache() // clear all
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('generates consistent hash', () => {
    const key1 = generateCacheKey({ registryPath: '/a', versionMarker: 'v1', method: 'm', params: { b: 2, a: 1 } })
    const key2 = generateCacheKey({ registryPath: '/a', versionMarker: 'v1', method: 'm', params: { a: 1, b: 2 } })
    expect(key1).toBe(key2)
  })

  it('singleflight and caching', async () => {
    let computeCount = 0
    const compute = async () => {
      computeCount++
      await new Promise(r => setTimeout(r, 10))
      return { data: { res: computeCount } }
    }

    const p1 = codeIntelCache.getOrCompute('key1', compute, { cacheable: true })
    const p2 = codeIntelCache.getOrCompute('key1', compute, { cacheable: true })

    vi.advanceTimersByTime(20)

    const [res1, res2] = await Promise.all([p1, p2])

    expect(computeCount).toBe(1)
    expect(res1.data.res).toBe(1)
    expect(res2.data.res).toBe(1)

    // from cache
    const p3 = await codeIntelCache.getOrCompute('key1', compute, { cacheable: true })
    expect(p3.data.res).toBe(1)
    expect(computeCount).toBe(1)
  })

  it('expires after TTL', async () => {
    let computeCount = 0
    const compute = async () => {
      computeCount++
      return { data: { res: computeCount } }
    }

    await codeIntelCache.getOrCompute('key1', compute, { cacheable: true })
    expect(computeCount).toBe(1)

    vi.advanceTimersByTime(60 * 1000 + 10)

    await codeIntelCache.getOrCompute('key1', compute, { cacheable: true })
    expect(computeCount).toBe(2)
  })

  it('bypasses cache when cacheable is false', async () => {
    let computeCount = 0
    const compute = async () => {
      computeCount++
      return { data: { res: computeCount } }
    }

    await codeIntelCache.getOrCompute('key1', compute, { cacheable: false })
    await codeIntelCache.getOrCompute('key1', compute, { cacheable: false })
    expect(computeCount).toBe(2)
  })

  it('invalidates by prefix', async () => {
    let computeCount = 0
    const compute = async () => {
      computeCount++
      return { data: { res: computeCount } }
    }

    await codeIntelCache.getOrCompute('/a:v1:m:hash', compute, { cacheable: true })
    invalidateShortLivedCache('/a')

    await codeIntelCache.getOrCompute('/a:v1:m:hash', compute, { cacheable: true })
    expect(computeCount).toBe(2)
  })

  it('evicts LRU when max items reached', async () => {
    for (let i = 0; i < 70; i++) {
      await codeIntelCache.getOrCompute(`key${i}`, async () => ({ data: i }), { cacheable: true })
    }
    
    // First few should be gone (64 limit)
    let computeCount = 0
    const compute = async () => { computeCount++; return { data: 99 } }
    await codeIntelCache.getOrCompute('key0', compute, { cacheable: true })
    expect(computeCount).toBe(1) // it had to recompute
  })
})
