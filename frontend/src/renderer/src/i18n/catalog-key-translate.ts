import { translate } from './i18n'

// Why: quality-gate views are written against stable catalog keys (read by name, not
// auto-extracted from English source), so the key doubles as the visible fallback
// only if a locale lacks it; en.json always carries every key.
export function translateCatalogKey(key: string, params?: Record<string, unknown>): string {
  return translate(key, key, params)
}

export type CatalogKeyTranslate = typeof translateCatalogKey
