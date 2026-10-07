import fs from 'fs'
import path from 'path'
import { QualityCheckProfile, validateProfile, definitionHashOf } from './quality-profile-schema'
import { getCatalog } from './quality-profile-catalog'

export interface HostOverrides {
  add: QualityCheckProfile[]
  disable: string[]
  replace: { id: string; reason: string; profile: QualityCheckProfile }[]
}

export interface OverridesDeps {
  warn: (msg: string) => void
}

export function loadHostOverrides(home: string, deps: OverridesDeps): HostOverrides {
  const empty: HostOverrides = { add: [], disable: [], replace: [] }
  const dirPath = path.join(home, '.orca', 'quality')
  const filePath = path.join(dirPath, 'profiles.json')

  try {
    if (!fs.existsSync(filePath)) return empty

    if (process.platform !== 'win32') {
      try {
        const dirStat = fs.statSync(dirPath)
        const fileStat = fs.statSync(filePath)
        if ((dirStat.mode & 0o077) !== 0) {
          deps.warn(`Skipping overrides: Directory ${dirPath} has too broad permissions (needs 0700)`)
          return empty
        }
        if ((fileStat.mode & 0o077) !== 0) {
          deps.warn(`Skipping overrides: File ${filePath} has too broad permissions (needs 0600)`)
          return empty
        }
      } catch (e) {
        deps.warn(`Skipping overrides: Failed to stat: ${(e as Error).message}`)
        return empty
      }
    }

    const content = fs.readFileSync(filePath, 'utf8')
    const parsed = JSON.parse(content)

    if (parsed.version !== 1) {
      deps.warn(`Skipping overrides: Unsupported version ${parsed.version}`)
      return empty
    }

    const out: HostOverrides = { add: [], disable: [], replace: [] }

    if (Array.isArray(parsed.add)) {
      for (const p of parsed.add) {
        const val = validateProfile(p)
        if (!val.ok) deps.warn(`Skipping added profile ${p.id}: ${val.errors.join(', ')}`)
        else out.add.push(val.value)
      }
    }

    if (Array.isArray(parsed.disable)) {
      for (const id of parsed.disable) {
        if (typeof id === 'string') out.disable.push(id)
      }
    }

    if (Array.isArray(parsed.replace)) {
      for (const r of parsed.replace) {
        if (typeof r.id !== 'string' || typeof r.reason !== 'string' || r.reason.length < 10) {
          deps.warn(`Skipping replace: invalid id or reason < 10 chars`)
          continue
        }
        const val = validateProfile(r.profile)
        if (!val.ok) {
          deps.warn(`Skipping replace profile ${r.id}: ${val.errors.join(', ')}`)
          continue
        }
        out.replace.push({ id: r.id, reason: r.reason, profile: val.value })
      }
    }

    return out
  } catch (e) {
    deps.warn(`Skipping overrides: Failed to parse JSON: ${(e as Error).message}`)
    return empty
  }
}

export function applyOverrides(
  catalog: QualityCheckProfile[],
  overrides: HostOverrides,
  deps: OverridesDeps
): { catalog: QualityCheckProfile[]; source: Record<string, 'builtin' | 'host'> } {
  const map = new Map<string, QualityCheckProfile>()
  const source: Record<string, 'builtin' | 'host'> = {}

  for (const p of catalog) {
    map.set(p.id, p)
    source[p.id] = 'builtin'
  }

  for (const id of overrides.disable) {
    if (map.has(id)) {
      map.delete(id)
      delete source[id]
    } else {
      deps.warn(`Cannot disable unknown profile: ${id}`)
    }
  }

  for (const p of overrides.add) {
    if (map.has(p.id)) {
      deps.warn(`Cannot add profile ${p.id}, already exists in builtin`)
    } else {
      map.set(p.id, p)
      source[p.id] = 'host'
    }
  }

  for (const r of overrides.replace) {
    if (!map.has(r.id)) {
      deps.warn(`Cannot replace unknown profile: ${r.id}`)
      continue
    }
    const oldP = map.get(r.id)!
    const hOld = definitionHashOf(oldP)
    const hNew = definitionHashOf(r.profile)
    deps.warn(`Replacing profile ${r.id} (hash ${hOld} -> ${hNew}) due to: ${r.reason}`)
    map.set(r.id, r.profile)
    source[r.id] = 'host'
  }

  return { catalog: Array.from(map.values()), source }
}
