import { useEffect, useState } from 'react'
import { appGetFeatureWallAssetBaseUrl } from '@/runtime/runtime-app-client'

export function toFeatureWallAssetUrl(baseUrl: string | null, assetPath: string): string | null {
  if (!baseUrl) {
    return null
  }
  try {
    return new URL(assetPath, baseUrl).toString()
  } catch {
    return null
  }
}

export function useFeatureWallAssetBaseUrl(load: boolean): string | null {
  const [assetBaseUrl, setAssetBaseUrl] = useState<string | null>(null)

  useEffect(() => {
    if (!load || assetBaseUrl !== null) {
      return
    }

    let cancelled = false
    void appGetFeatureWallAssetBaseUrl()
      .then((url) => {
        if (!cancelled) {
          setAssetBaseUrl(url)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setAssetBaseUrl('')
        }
      })
    return () => {
      cancelled = true
    }
  }, [assetBaseUrl, load])

  return assetBaseUrl
}
