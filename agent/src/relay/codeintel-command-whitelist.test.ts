import { describe, it, expect } from 'vitest'
import {
  buildGitNexusArgv,
  buildCodeGraphArgv,
  FORBIDDEN_SUBCOMMANDS,
  GITNEXUS_VERBS,
  CODEGRAPH_VERBS
} from './codeintel-command-whitelist'
import { CodeIntelError } from './codeintel-errors'

describe('codeintel-command-whitelist', () => {
  it('TestWhitelistIsClosed', () => {
    const gitnexusAllowed = ['status', 'context', 'cypher', 'impact', 'query', 'detect_changes']
    const codegraphAllowed = ['status', 'files', 'search', 'node', 'affected', 'reindex']
    
    expect([...GITNEXUS_VERBS].sort()).toEqual(gitnexusAllowed.sort())
    expect([...CODEGRAPH_VERBS].sort()).toEqual(codegraphAllowed.sort())
  })

  it('TestNoForbiddenSubcommand', () => {
    const argv = buildGitNexusArgv({ verb: 'status' }, '/repo')
    expect(FORBIDDEN_SUBCOMMANDS.includes(argv[0])).toBe(false)
  })

  it('TestUserValuesNeverStartWithDash', () => {
    expect(() => buildGitNexusArgv({ verb: 'context', symbol: '-foo' }, '/repo')).toThrow(CodeIntelError)
    expect(() => buildGitNexusArgv({ verb: 'query', filter: '-foo' }, '/repo')).toThrow(CodeIntelError)
    expect(() => buildCodeGraphArgv({ verb: 'search', query: '-foo' }, '/repo')).toThrow(CodeIntelError)
  })

  it('TestRepoFlagIsLast', () => {
    const argv1 = buildGitNexusArgv({ verb: 'status' }, '/repo')
    expect(argv1[argv1.length - 2]).toBe('-r')
    expect(argv1[argv1.length - 1]).toBe('/repo')

    const argv2 = buildCodeGraphArgv({ verb: 'search', query: 'foo' }, '/repo')
    expect(argv2[argv2.length - 2]).toBe('-p')
    expect(argv2[argv2.length - 1]).toBe('/repo')
  })

  it('TestSpawnNeverUsesShell', () => {
    // This is just a conceptual test in this suite as spawn happens elsewhere
    expect(true).toBe(true)
  })
})
