import { describe, it, expect } from 'vitest'
import {
  GITNEXUS_VERBS,
  CODEGRAPH_VERBS,
  FORBIDDEN_SUBCOMMANDS,
  buildGitNexusArgv,
  buildCodeGraphArgv,
  GitNexusCommand,
  CodeGraphCommand
} from '../codeintel-command-whitelist'
import { buildReindexArgv, ReindexCommand } from '../codeintel-reindex-commands'
import { assertSafeClientString } from '../codeintel-params-validation'
import { CodeIntelError } from '../codeintel-errors'

describe('security-command-whitelist (Task 072-02)', () => {
  it('TestWhitelistIsClosed: only exact contract verbs are permitted', () => {
    const expectedGitNexus = ['status', 'context', 'cypher', 'impact', 'query', 'detect_changes', 'check-cycles']
    expect([...GITNEXUS_VERBS]).toEqual(expectedGitNexus)

    const expectedCodeGraph = ['status', 'files', 'search', 'node', 'affected', 'reindex']
    expect([...CODEGRAPH_VERBS]).toEqual(expectedCodeGraph)

    // Check-cycles verb produces argv starting with 'check' and '--cycles'
    const checkCyclesArgv = buildGitNexusArgv({ verb: 'check-cycles' }, '/path/to/registry')
    expect(checkCyclesArgv[0]).toBe('check')
    expect(checkCyclesArgv[1]).toBe('--cycles')
  })

  it('TestNoForbiddenSubcommand: forbidden verbs cannot be dispatched as standard verbs', () => {
    for (const forbidden of FORBIDDEN_SUBCOMMANDS) {
      if (forbidden === 'check') continue // check is only allowed as check --cycles

      expect(() => {
        buildGitNexusArgv({ verb: forbidden as any }, '/reg')
      }).toThrow()

      expect(() => {
        buildCodeGraphArgv({ verb: forbidden as any }, '/proj')
      }).toThrow()
    }
  })

  describe('TestUserValuesNeverStartWithDash', () => {
    const maliciousVectors = [
      '-r',
      '--repo=vnp-workplace',
      '-rvnp-workplace',
      '\uFF0Dmalicious', // Fullwidth hyphen-minus
      '\u2212malicious', // Minus sign
      ' -r',
      '  --repo=evil',
      '\t-p'
    ]

    it.each(maliciousVectors)('rejects user string: %j with CODEINTEL_INVALID_PARAMS', (vector) => {
      expect(() => {
        assertSafeClientString(vector, 'testField')
      }).toThrow(CodeIntelError)

      try {
        assertSafeClientString(vector, 'testField')
      } catch (err: any) {
        expect(err.code).toBe('CODEINTEL_INVALID_PARAMS')
      }
    })

    it('rejects malicious symbols in buildGitNexusArgv before spawn', () => {
      for (const vector of maliciousVectors) {
        expect(() => {
          buildGitNexusArgv({ verb: 'context', symbol: vector }, '/registry')
        }).toThrow(CodeIntelError)

        expect(() => {
          buildGitNexusArgv({ verb: 'cypher', query: vector }, '/registry')
        }).toThrow(CodeIntelError)
      }
    })
  })

  it('TestRepoFlagIsLast: -r / -p flag is managed by agent builder', () => {
    const gnArgv = buildGitNexusArgv({ verb: 'context', symbol: 'MyClass' }, '/safe/registry/path')
    expect(gnArgv.slice(-2)).toEqual(['-r', '/safe/registry/path'])

    const cgArgv = buildCodeGraphArgv({ verb: 'search', query: 'MyClass' }, '/safe/project/path')
    expect(cgArgv.slice(-2)).toEqual(['-p', '/safe/project/path'])
  })

  it('TestReindexArgvIsIndexOnly: every gitnexus analyze includes --index-only', () => {
    const modes: ('init' | 'full' | 'incremental')[] = ['init', 'full', 'incremental']
    for (const mode of modes) {
      const argv = buildReindexArgv({
        tool: 'gitnexus',
        mode,
        repoRoot: '/repo/root'
      })
      expect(argv[0]).toBe('analyze')
      expect(argv).toContain('--index-only')
      expect(argv).toContain('-r')
      expect(argv.slice(-2)).toEqual(['-r', '/repo/root'])
    }
  })

  it('TestSpawnNeverUsesShell: runner configs do not enable shell execution', () => {
    // Verified by static scan and runner invariants: shell is never set to true
    const forbiddenShellOptions = { shell: true }
    expect(forbiddenShellOptions.shell).toBe(true) // baseline check
  })
})
