import { parse as parseYaml } from 'yaml'
import semver from 'semver'
import type { RawQualityFinding } from './quality-rule-diff-matcher'

export interface PackageVersionEntry {
  name: string
  version: string
  source?: string
}

export interface DiffPnpmLockResult {
  added: PackageVersionEntry[]
  removed: PackageVersionEntry[]
  bumped: Array<{ name: string; from: string; to: string; major: boolean }>
  downgraded: Array<{ name: string; from: string; to: string }>
  newSource: PackageVersionEntry[]
}

export interface DiffGoModResult {
  requiresAdded: Array<{ module: string; version: string }>
  requiresRemoved: Array<{ module: string; version: string }>
  replaced: Array<{ oldModule: string; newTarget: string; local: boolean }>
}

function parsePnpmPackages(yamlText: string): Map<string, PackageVersionEntry> {
  const map = new Map<string, PackageVersionEntry>()
  if (!yamlText || !yamlText.trim()) return map

  let doc: any
  try {
    doc = parseYaml(yamlText)
  } catch {
    return map
  }

  if (!doc || typeof doc !== 'object') return map
  const packages = doc.packages
  if (!packages || typeof packages !== 'object') return map

  for (const rawKey of Object.keys(packages)) {
    // rawKey can be "/foo@1.0.0" or "foo@1.0.0" or "foo@git+https://..."
    let key = rawKey
    if (key.startsWith('/')) key = key.substring(1)

    // Split name and version (handling scoped @scope/pkg)
    const atIndex = key.lastIndexOf('@')
    if (atIndex <= 0) continue

    const name = key.substring(0, atIndex)
    const versionPart = key.substring(atIndex + 1)

    // Check if source is git, tarball, or non-registry URL
    let source: string | undefined
    if (
      versionPart.startsWith('git') ||
      versionPart.startsWith('http://') ||
      versionPart.startsWith('https://') ||
      versionPart.endsWith('.tgz') ||
      versionPart.endsWith('.tar.gz')
    ) {
      source = versionPart
    }

    // Clean version for semver
    const cleanVer = semver.clean(versionPart) || versionPart

    map.set(name, {
      name,
      version: cleanVer,
      source
    })
  }

  return map
}

export function diffPnpmLock(baseText: string, headText: string): DiffPnpmLockResult {
  const baseMap = parsePnpmPackages(baseText)
  const headMap = parsePnpmPackages(headText)

  const added: PackageVersionEntry[] = []
  const removed: PackageVersionEntry[] = []
  const bumped: Array<{ name: string; from: string; to: string; major: boolean }> = []
  const downgraded: Array<{ name: string; from: string; to: string }> = []
  const newSource: PackageVersionEntry[] = []

  for (const [name, headPkg] of headMap.entries()) {
    const basePkg = baseMap.get(name)
    if (!basePkg) {
      added.push(headPkg)
      if (headPkg.source) newSource.push(headPkg)
    } else {
      if (headPkg.source && !basePkg.source) {
        newSource.push(headPkg)
      }

      if (headPkg.version !== basePkg.version) {
        const baseCoerced = semver.coerce(basePkg.version)
        const headCoerced = semver.coerce(headPkg.version)

        if (baseCoerced && headCoerced) {
          if (semver.gt(headCoerced, baseCoerced)) {
            const major = headCoerced.major > baseCoerced.major
            bumped.push({ name, from: basePkg.version, to: headPkg.version, major })
          } else if (semver.lt(headCoerced, baseCoerced)) {
            downgraded.push({ name, from: basePkg.version, to: headPkg.version })
          }
        } else {
          // Fallback comparison
          bumped.push({ name, from: basePkg.version, to: headPkg.version, major: false })
        }
      }
    }
  }

  for (const [name, basePkg] of baseMap.entries()) {
    if (!headMap.has(name)) {
      removed.push(basePkg)
    }
  }

  return { added, removed, bumped, downgraded, newSource }
}

function parseGoMod(text: string): {
  requires: Map<string, string>
  replaces: Map<string, { target: string; local: boolean }>
} {
  const requires = new Map<string, string>()
  const replaces = new Map<string, { target: string; local: boolean }>()

  if (!text) return { requires, replaces }

  const lines = text.split(/\r?\n/)
  let inRequireBlock = false
  let inReplaceBlock = false

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('//')) continue

    if (trimmed === 'require (') {
      inRequireBlock = true
      continue
    }
    if (trimmed === 'replace (') {
      inReplaceBlock = true
      continue
    }
    if (trimmed === ')') {
      inRequireBlock = false
      inReplaceBlock = false
      continue
    }

    if (inRequireBlock || trimmed.startsWith('require ')) {
      const content = inRequireBlock ? trimmed : trimmed.substring(8).trim()
      const parts = content.split(/\s+/)
      if (parts.length >= 2) {
        requires.set(parts[0], parts[1])
      }
    } else if (inReplaceBlock || trimmed.startsWith('replace ')) {
      const content = inReplaceBlock ? trimmed : trimmed.substring(8).trim()
      const parts = content.split(/\s*=>\s*/)
      if (parts.length === 2) {
        const oldMod = parts[0].trim()
        const newTarget = parts[1].trim()
        const local = newTarget.startsWith('./') || newTarget.startsWith('../') || newTarget.startsWith('/')
        replaces.set(oldMod, { target: newTarget, local })
      }
    }
  }

  return { requires, replaces }
}

export function diffGoMod(baseText: string, headText: string): DiffGoModResult {
  const base = parseGoMod(baseText)
  const head = parseGoMod(headText)

  const requiresAdded: Array<{ module: string; version: string }> = []
  const requiresRemoved: Array<{ module: string; version: string }> = []
  const replaced: Array<{ oldModule: string; newTarget: string; local: boolean }> = []

  for (const [mod, ver] of head.requires.entries()) {
    if (!base.requires.has(mod)) {
      requiresAdded.push({ module: mod, version: ver })
    }
  }

  for (const [mod, ver] of base.requires.entries()) {
    if (!head.requires.has(mod)) {
      requiresRemoved.push({ module: mod, version: ver })
    }
  }

  for (const [mod, rep] of head.replaces.entries()) {
    const baseRep = base.replaces.get(mod)
    if (!baseRep || baseRep.target !== rep.target) {
      replaced.push({ oldModule: mod, newTarget: rep.target, local: rep.local })
    }
  }

  return { requiresAdded, requiresRemoved, replaced }
}

export function checkPackageJsonDrift(
  pkgBaseText: string,
  pkgHeadText: string,
  lockChanged: boolean
): boolean {
  let baseDeps: Record<string, string> = {}
  let headDeps: Record<string, string> = {}

  try {
    const baseJson = JSON.parse(pkgBaseText)
    baseDeps = { ...(baseJson.dependencies || {}), ...(baseJson.devDependencies || {}) }
  } catch {}

  try {
    const headJson = JSON.parse(pkgHeadText)
    headDeps = { ...(headJson.dependencies || {}), ...(headJson.devDependencies || {}) }
  } catch {}

  const depsChanged = JSON.stringify(baseDeps) !== JSON.stringify(headDeps)
  // Drift occurs if dependencies in package.json changed without lockfile changing, or vice versa
  return depsChanged !== lockChanged
}

export function generateDependencyFindings(diffResult: {
  pnpm?: DiffPnpmLockResult
  goMod?: DiffGoModResult
  drift?: boolean
  filePath?: string
}): { findings: RawQualityFinding[]; truncated: boolean } {
  const findings: RawQualityFinding[] = []
  const maxFindings = 2000
  let truncated = false

  const addFinding = (f: RawQualityFinding) => {
    if (findings.length >= maxFindings) {
      truncated = true
      return
    }
    findings.push(f)
  }

  const file = diffResult.filePath || 'pnpm-lock.yaml'

  if (diffResult.pnpm) {
    for (const add of diffResult.pnpm.added) {
      addFinding({
        ruleId: 'DEP-ADDED',
        severity: 'info',
        file,
        message: `Dependency added: ${add.name}@${add.version}`,
        anchorOverride: `${add.name}@${add.version}`
      })
    }
    for (const rem of diffResult.pnpm.removed) {
      addFinding({
        ruleId: 'DEP-REMOVED',
        severity: 'info',
        file,
        message: `Dependency removed: ${rem.name}@${rem.version}`,
        anchorOverride: `${rem.name}@${rem.version}`
      })
    }
    for (const bump of diffResult.pnpm.bumped) {
      addFinding({
        ruleId: bump.major ? 'DEP-MAJOR-BUMP' : 'DEP-BUMP',
        severity: bump.major ? 'warning' : 'info',
        file,
        message: `Dependency updated: ${bump.name} (${bump.from} -> ${bump.to})`,
        anchorOverride: `${bump.name}@${bump.from}->${bump.to}`
      })
    }
    for (const down of diffResult.pnpm.downgraded) {
      addFinding({
        ruleId: 'DEP-DOWNGRADE',
        severity: 'warning',
        file,
        message: `Dependency downgraded: ${down.name} (${down.from} -> ${down.to})`,
        anchorOverride: `${down.name}@${down.from}->${down.to}`
      })
    }
    for (const src of diffResult.pnpm.newSource) {
      addFinding({
        ruleId: 'DEP-NEW-SOURCE',
        severity: 'warning',
        file,
        message: `Dependency uses non-registry source: ${src.name} (${src.source})`,
        anchorOverride: `${src.name}`
      })
    }
  }

  if (diffResult.goMod) {
    const goFile = 'backend-go/go.mod'
    for (const rep of diffResult.goMod.replaced) {
      addFinding({
        ruleId: 'DEP-NEW-SOURCE',
        severity: 'warning',
        file: goFile,
        message: `Module replace directive: ${rep.oldModule} => ${rep.newTarget}${rep.local ? ' (local path)' : ''}`,
        anchorOverride: `${rep.oldModule}`
      })
    }
  }

  if (diffResult.drift) {
    addFinding({
      ruleId: 'DEP-LOCKFILE-DRIFT',
      severity: 'warning',
      file: 'package.json',
      message: 'Mismatch between package.json dependencies and lockfile state',
      anchorOverride: 'package.json'
    })
  }

  return { findings, truncated }
}
