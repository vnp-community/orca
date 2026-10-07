import path from 'path'
import { QualityCheckProfile } from './quality-profile-schema'
import { PlannedStep } from './quality-run-types'
import { buildQualityChildEnv } from './quality-child-env'

export interface PlanRunInput {
  suiteIds?: string[]
  profileIds?: string[]
  catalog: QualityCheckProfile[]
  suites: { id: string; profiles: string[] }[]
  workspaceRoot: string
  changedFiles: string[] | null // null means worktree
  tmpRunDir: string
  base?: string
  sourceEnv: NodeJS.ProcessEnv
  deps: {
    resolveBin: (bin: string) => string
    gitCommonDir: string
    readGoWork: () => string[] // returns paths of modules relative to workspaceRoot, e.g. ['backend-go', 'backend-go/common']
  }
}

export interface PlanRunOutput {
  steps: PlannedStep[]
  scopeWidened: boolean
  scopeFiles: string[]
}

export class QualityPlanError extends Error {
  constructor(public reason: string, public available?: string[]) {
    super(`Plan error: ${reason}`)
  }
}

export function planRun(input: PlanRunInput): PlanRunOutput {
  const profileSet = new Set<string>()
  
  if (input.suiteIds) {
    for (const sid of input.suiteIds) {
      const suite = input.suites.find(s => s.id === sid)
      if (suite) {
        suite.profiles.forEach(p => profileSet.add(p))
      }
    }
  }
  if (input.profileIds) {
    input.profileIds.forEach(p => profileSet.add(p))
  }

  const catalogMap = new Map<string, QualityCheckProfile>()
  input.catalog.forEach(p => catalogMap.set(p.id, p))

  const selectedProfiles: QualityCheckProfile[] = []
  for (const pid of profileSet) {
    if (!catalogMap.has(pid)) {
      throw new QualityPlanError('PROFILE_UNKNOWN', Array.from(catalogMap.keys()))
    }
    selectedProfiles.push(catalogMap.get(pid)!)
  }

  const steps: PlannedStep[] = []
  let scopeWidened = false
  const scopeFilesSet = new Set<string>()

  const selectScriptRules = (changedFiles: readonly string[]): string[] => {
    const selected: string[] = []
    if (changedFiles.some(f => f.includes('/i18n/') || f.startsWith('i18n/'))) selected.push('ORCA-004')
    if (changedFiles.some(f => f.endsWith('.tsx') && f.includes('renderer/'))) selected.push('ORCA-005')
    if (changedFiles.some(f => f.includes('resources/onboarding/feature-wall'))) selected.push('ORCA-006')
    return selected
  }

  for (const profile of selectedProfiles) {
    if (profile.kind === 'repo-rules') {
      if (!input.changedFiles) {
        steps.push({
          id: profile.id,
          toolPath: '',
          args: [],
          cwd: input.workspaceRoot,
          env: {},
          timeoutMs: profile.timeoutMs || 60000,
          maxOutputBytes: profile.maxOutputBytes || 1024 * 1024,
          heavy: profile.heavy || false,
          parser: profile.parser,
          inProcess: true,
          skipReason: 'base_required'
        })
        continue
      }

      let selectedArgs: string[] = []
      if (profile.id === 'repo-rules-scripts') {
        selectedArgs = selectScriptRules(input.changedFiles)
        if (selectedArgs.length === 0) continue
      }

      steps.push({
        id: profile.id,
        toolPath: '',
        args: selectedArgs,
        cwd: input.workspaceRoot,
        env: {},
        timeoutMs: profile.timeoutMs || 60000,
        maxOutputBytes: profile.maxOutputBytes || 1024 * 1024,
        heavy: profile.heavy || false,
        parser: profile.parser,
        inProcess: true
      })
      continue
    }

    let stepFiles = input.changedFiles || []
    
    // filter by fileExtensions
    if (profile.fileExtensions && profile.fileExtensions.length > 0 && input.changedFiles) {
      stepFiles = input.changedFiles.filter(f => 
        profile.fileExtensions!.some(ext => f.endsWith(ext))
      )
    }

    let appendFiles: string[] = []
    let skip = false

    const strategy = profile.scopeStrategy || 'none'
    
    if (strategy === 'append-files') {
      if (input.changedFiles) {
        if (stepFiles.length === 0) {
          skip = true
        } else if (stepFiles.length <= 300) {
          appendFiles = stepFiles
        } else {
          scopeWidened = true
        }
      }
    } else if (strategy === 'go-modules') {
      if (input.changedFiles) {
        const modules = input.deps.readGoWork()
        const changedModules = new Set<string>()
        
        let commonChanged = false
        for (const f of input.changedFiles) {
          if (f.startsWith('backend-go/common/') || f.startsWith('backend-go/proto/')) {
            commonChanged = true
            break
          }
        }

        if (commonChanged) {
          scopeWidened = true
          for (const m of modules) changedModules.add(m)
        } else {
          for (const f of stepFiles) {
            // find longest prefix
            let best = ''
            for (const m of modules) {
              if (f.startsWith(m + '/')) {
                if (m.length > best.length) best = m
              }
            }
            if (best) changedModules.add(best)
          }
        }
        
        if (changedModules.size === 0) {
          skip = true
        } else {
          appendFiles = Array.from(changedModules)
        }
      } else {
        // worktree
        appendFiles = [] // maybe run on . ? Or pass all modules? The parser/argv will handle it if `{files}` is missing? 
        // wait, if {files} is present and worktree -> we just replace {files} with empty or maybe nothing? 
        // We'll replace {files} with all modules if worktree for go-modules?
        // Actually, if worktree, appendFiles = [] usually means no file list, the tool runs on all.
      }
    }

    if (skip) continue

    for (const f of stepFiles) scopeFilesSet.add(f)

    let { env } = buildQualityChildEnv({
      sourceEnv: input.sourceEnv,
      qualityToolPath: process.env.PATH || '',
      tmpDir: input.tmpRunDir,
      profileEnv: { set: profile.env?.set || {}, allowExtra: profile.env?.copy || [] },
      limits: { gomaxprocs: 2 }
    })

    const args: string[] = []
    let toolPath = ''

    for (const token of profile.argv) {
      if (token === '{files}') {
        args.push(...appendFiles)
      } else if (token === '{files|d}') {
        const dirs = new Set(appendFiles.map(f => path.dirname(f)))
        args.push(...Array.from(dirs))
      } else {
        const resolved = token.replace(/\{(bin:[^}]+|tmp:[^}]+|tmp|base|gitCommonDir)\}/g, (match, inner) => {
          if (inner.startsWith('bin:')) {
            const binName = inner.substring(4)
            const r = input.deps.resolveBin(binName)
            if (!toolPath) toolPath = r
            return r
          }
          if (inner.startsWith('tmp:')) return path.join(input.tmpRunDir, inner.substring(4))
          if (inner === 'tmp') return input.tmpRunDir
          if (inner === 'base') return input.base || ''
          if (inner === 'gitCommonDir') return input.deps.gitCommonDir
          return match
        })
        args.push(resolved)
        // If it didn't contain bin but it's the first token
        if (!toolPath && args.length === 1) toolPath = resolved
      }
    }

    let cwd = input.workspaceRoot
    if (profile.cwd) {
      cwd = path.resolve(input.workspaceRoot, profile.cwd)
      if (!cwd.startsWith(input.workspaceRoot + path.sep) && cwd !== input.workspaceRoot) {
        throw new QualityPlanError(`Invalid cwd: ${profile.cwd}`)
      }
    }

    steps.push({
      id: profile.id,
      toolPath,
      args,
      cwd,
      env,
      timeoutMs: profile.timeoutMs || 60000,
      maxOutputBytes: profile.maxOutputBytes || 1024 * 1024,
      heavy: profile.heavy || false,
      parser: profile.parser
    })
  }

  return {
    steps,
    scopeWidened,
    scopeFiles: Array.from(scopeFilesSet)
  }
}
