export const SECRET_ENV_NAME_PATTERN = /(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i
const ALWAYS_DENY_PREFIXES = ['ORCA_', 'AWS_', 'GOOGLE_']
const ALWAYS_DENY_EXACT = ['SSH_AUTH_SOCK', 'ANTHROPIC_API_KEY', 'GITHUB_TOKEN', 'GH_TOKEN', 'AGENT_TOKEN']
const ALLOWED_CORE_ENV = ['HOME', 'USER', 'LOGNAME', 'LANG', 'LC_ALL', 'TZ', 'SHELL', 'GOCACHE', 'GOPATH', 'GOMODCACHE', 'PNPM_HOME', 'npm_config_cache']

export interface BuildQualityChildEnvInput {
  sourceEnv: NodeJS.ProcessEnv
  qualityToolPath: string
  tmpDir: string
  profileEnv: {
    set: Record<string, string>
    allowExtra: string[]
  }
  limits: {
    gomaxprocs: number
    gomemlimitBytes?: number
    nodeOldSpaceMb?: number
  }
  nodeOptions?: string
}

export interface BuildQualityChildEnvOutput {
  env: NodeJS.ProcessEnv
  rejectedExtra: string[]
}

function isDeniedEnv(name: string): boolean {
  if (ALWAYS_DENY_EXACT.includes(name)) return true
  for (const prefix of ALWAYS_DENY_PREFIXES) {
    if (name.startsWith(prefix)) return true
  }
  return SECRET_ENV_NAME_PATTERN.test(name)
}

function checkNodeOptions(val: string): boolean {
  const disallowed = ['--require', '-r ', '--import', '--loader']
  for (const item of disallowed) {
    if (val.includes(item)) return false
  }
  return true
}

export function buildQualityChildEnv(input: BuildQualityChildEnvInput): BuildQualityChildEnvOutput {
  const env: NodeJS.ProcessEnv = {}
  const rejectedExtra: string[] = []

  // Copy allowed core envs
  for (const name of ALLOWED_CORE_ENV) {
    if (input.sourceEnv[name] !== undefined) {
      env[name] = input.sourceEnv[name]
    }
  }

  // Set mandatory envs
  env['PATH'] = input.qualityToolPath
  env['TMPDIR'] = input.tmpDir
  env['CI'] = '1'
  env['NO_COLOR'] = '1'
  env['FORCE_COLOR'] = '0'
  env['TERM'] = 'dumb'

  // Set Go limits
  env['GOTOOLCHAIN'] = 'local'
  env['GOMAXPROCS'] = String(input.limits.gomaxprocs)
  if (input.limits.gomemlimitBytes !== undefined) {
    env['GOMEMLIMIT'] = String(input.limits.gomemlimitBytes)
  }

  // Set Node options
  let nodeOpts = input.nodeOptions || ''
  if (input.limits.nodeOldSpaceMb !== undefined) {
    const memOpt = `--max-old-space-size=${input.limits.nodeOldSpaceMb}`
    if (!nodeOpts.includes('--max-old-space-size')) {
      nodeOpts = nodeOpts ? `${nodeOpts} ${memOpt}` : memOpt
    }
  }

  if (nodeOpts) {
    if (!checkNodeOptions(nodeOpts)) {
      rejectedExtra.push('NODE_OPTIONS')
    } else {
      env['NODE_OPTIONS'] = nodeOpts
    }
  }

  // Handle profileEnv.allowExtra
  for (const extra of input.profileEnv.allowExtra) {
    if (isDeniedEnv(extra)) {
      rejectedExtra.push(extra)
    } else if (input.sourceEnv[extra] !== undefined) {
      env[extra] = input.sourceEnv[extra]
    }
  }

  // Handle profileEnv.set
  for (const [key, val] of Object.entries(input.profileEnv.set)) {
    if (isDeniedEnv(key)) {
      rejectedExtra.push(key)
    } else {
      // Additional check if key is NODE_OPTIONS
      if (key === 'NODE_OPTIONS') {
        if (!checkNodeOptions(val)) {
          rejectedExtra.push('NODE_OPTIONS')
          continue
        }
        // Merge with existing NODE_OPTIONS
        if (env['NODE_OPTIONS']) {
           env['NODE_OPTIONS'] = `${env['NODE_OPTIONS']} ${val}`
        } else {
           env['NODE_OPTIONS'] = val
        }
      } else {
        env[key] = val
      }
    }
  }

  return { env, rejectedExtra }
}
