import fs from 'fs'
import path from 'path'
import { execFile } from 'child_process'
import { promisify } from 'util'
import { AgentConfig } from './agent-config'
import { buildCodeIntelChildEnv } from './codeintel-child-env'

const execFileAsync = promisify(execFile)

export type CodeIntelToolInfo = {
  available: boolean
  version: string | null
  supported: boolean
  binary: string | null
  unsupportedPlatform?: boolean
}

export type CodeIntelToolsReport = {
  gitnexus: CodeIntelToolInfo
  codegraph: CodeIntelToolInfo
  unsupportedPlatform?: boolean
}

let cachedReport: CodeIntelToolsReport | null = null
let cacheTimestamp = 0

import { access } from 'fs/promises'

export async function hasCodeIntelBinary(config: AgentConfig, tool: 'gitnexus' | 'codegraph'): Promise<boolean> {
  if (process.platform === 'win32') return false
  const paths = (config.toolPath || '').split(path.delimiter)
  for (const dir of paths) {
    if (!dir) continue
    const candidate = path.join(dir, tool)
    try {
      await access(candidate, fs.constants.X_OK)
      return true
    } catch {}
  }
  return false
}

export async function detectCodeIntelBinaries(config: AgentConfig): Promise<{ gitnexus: boolean; codegraph: boolean }> {
  const [gitnexus, codegraph] = await Promise.all([
    hasCodeIntelBinary(config, 'gitnexus'),
    hasCodeIntelBinary(config, 'codegraph')
  ])
  return { gitnexus, codegraph }
}

export async function detectCodeIntelTools(config: AgentConfig): Promise<CodeIntelToolsReport> {
  if (process.platform === 'win32') {
    return {
      unsupportedPlatform: true,
      gitnexus: { available: false, version: null, supported: false, binary: null, unsupportedPlatform: true },
      codegraph: { available: false, version: null, supported: false, binary: null, unsupportedPlatform: true }
    }
  }

  const now = Date.now()
  if (cachedReport && now - cacheTimestamp < 60000) {
    return cachedReport
  }

  const env = buildCodeIntelChildEnv({ toolEnv: (config as any).toolEnv ?? {} })

  const checkTool = async (tool: 'gitnexus' | 'codegraph'): Promise<CodeIntelToolInfo> => {
    let binaryPath: string | null = null
    const paths = (config.toolPath || '').split(path.delimiter)
    for (const dir of paths) {
      if (!dir) continue
      const candidate = path.join(dir, tool)
      try {
        fs.accessSync(candidate, fs.constants.X_OK)
        binaryPath = candidate
        break
      } catch {}
    }

    if (!binaryPath) {
      return { available: false, version: null, supported: false, binary: null }
    }

    try {
      const { stdout } = await execFileAsync(binaryPath, ['--version'], {
        timeout: 5000,
        env
      })
      
      const match = stdout.match(/([0-9]+\.[0-9]+\.[0-9]+)/)
      const version = match ? match[1] : null
      let supported = false

      if (version) {
        const parts = version.split('.').map(Number)
        if (tool === 'gitnexus') {
          if (parts[0] === 1 && parts[1] >= 6) supported = true
        } else if (tool === 'codegraph') {
          if (parts[0] === 1 && parts[1] >= 4) supported = true
        }
      }

      return {
        available: true,
        version,
        supported,
        binary: binaryPath
      }
    } catch (err) {
      return { available: true, version: null, supported: false, binary: binaryPath }
    }
  }

  const [gitnexus, codegraph] = await Promise.all([
    checkTool('gitnexus'),
    checkTool('codegraph')
  ])

  cachedReport = { gitnexus, codegraph }
  cacheTimestamp = now

  return cachedReport
}

export function resetCodeIntelToolDetectionCache() {
  cachedReport = null
  cacheTimestamp = 0
}
