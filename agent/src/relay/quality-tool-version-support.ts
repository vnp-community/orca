export const SUPPORTED_QUALITY_TOOL_VERSIONS: Record<string, string> = {
  oxlint: '1.71.0',
  tsc: '7.0.2',
  vitest: '4.1.5',
  go: '1.26.0',
  'golangci-lint': '1.62.2',
  buf: '1.72.0',
  opa: '1.19.1',
  'orca-check': '1.0.0'
}

export type VersionSupport = 'verified' | 'untested' | 'incompatible'

export function classifyToolVersion(tool: string, versionText: string): VersionSupport {
  const supported = SUPPORTED_QUALITY_TOOL_VERSIONS[tool]
  if (!supported) return 'untested'

  // Extract semver
  const match = versionText.match(/(\d+)\.(\d+)\.(\d+)/)
  if (!match) return 'untested'

  const major = match[1]
  const supportedMajor = supported.split('.')[0]

  if (major !== supportedMajor) return 'incompatible'

  if (match[0] === supported) return 'verified'

  return 'untested'
}

export async function readToolVersion(
  tool: string,
  execute: (cmd: string, args: string[]) => Promise<{ stdout: string; stderr: string; exitCode: number }>
): Promise<string | null> {
  // Determine args
  let cmd = tool
  let args = ['--version']
  if (tool === 'go') {
    args = ['version']
  } else if (tool === 'orca-check') {
    return '1.0.0' // Builtin scripts
  }

  try {
    const res = await execute(cmd, args)
    if (res.exitCode !== 0) return null
    return res.stdout.trim()
  } catch (e) {
    return null
  }
}
