import { buildGitNexusArgv, buildCodeGraphArgv, GitNexusCommand, CodeGraphCommand } from '../codeintel-command-whitelist'

export interface CaptureStep {
  name: string
  argv: string[]
  writeStderr?: boolean
}

export function buildCapturePlan(tool: string, version: string, registryPath: string, repoPath: string): CaptureStep[] {
  const steps: CaptureStep[] = []

  if (tool === 'gitnexus') {
    if (version !== '1.6.9') {
      throw new Error(`Unsupported gitnexus version for fixtures: ${version}`)
    }

    const cmds: Array<{ name: string; cmd: GitNexusCommand }> = [
      { name: 'status', cmd: { verb: 'status' } },
      { name: 'context', cmd: { verb: 'context', symbol: 'User' } },
      { name: 'cypher', cmd: { verb: 'cypher', query: 'MATCH (n) RETURN count(n) as count' } },
      { name: 'impact', cmd: { verb: 'impact', symbol: 'User', direction: 'upstream' } },
      { name: 'query', cmd: { verb: 'query', filter: 'domain' } },
      { name: 'detect_changes', cmd: { verb: 'detect_changes' } },
      { name: 'check-cycles', cmd: { verb: 'check-cycles' } }
    ]

    for (const c of cmds) {
      const argv = buildGitNexusArgv(c.cmd, registryPath)
      steps.push({
        name: c.name,
        argv,
        writeStderr: c.cmd.verb === 'check-cycles'
      })
    }
  } else if (tool === 'codegraph') {
    if (version !== '1.4.1') {
      throw new Error(`Unsupported codegraph version for fixtures: ${version}`)
    }

    const cmds: Array<{ name: string; cmd: CodeGraphCommand }> = [
      { name: 'status', cmd: { verb: 'status' } },
      { name: 'files', cmd: { verb: 'files' } },
      { name: 'search', cmd: { verb: 'search', query: 'User' } },
      { name: 'node', cmd: { verb: 'node', id: '1' } }, // maybe node 1 won't exist but it's a fixture
      { name: 'affected', cmd: { verb: 'affected', files: ['internal/domain/user.go'] } },
      { name: 'reindex', cmd: { verb: 'reindex' } }
    ]

    for (const c of cmds) {
      const argv = buildCodeGraphArgv(c.cmd, repoPath)
      steps.push({
        name: c.name,
        argv,
        writeStderr: false
      })
    }
  } else {
    throw new Error(`Unknown tool: ${tool}`)
  }

  return steps
}
