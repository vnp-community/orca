import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { goTestParser } from './quality-parser-go-test'
import fs from 'fs/promises'
import path from 'path'
import os from 'os'
import { QualityParserInput } from './quality-parser-types'

describe('quality-parser-go-test', () => {
  let tempDir: string

  beforeEach(async () => {
    tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'gotest-'))
  })

  afterEach(async () => {
    await fs.rm(tempDir, { recursive: true, force: true })
  })

  function createMockInput(stdoutPath: string): QualityParserInput {
    return {
      stepId: 'step1',
      stdoutPath,
      stderrPath: '',
      exitCode: 1,
      timedOut: false,
      cancelled: false,
      cwd: '/opt/repos/backend',
      repoRoot: '/opt/repos/backend',
      platform: 'linux',
      toolVersion: '1.22.0',
      scopeFiles: [],
      readSourceLine: async () => null
    }
  }

  it('parses valid json format and cleans up memory on pass', async () => {
    const log = `
{"Action":"output","Package":"pkg1","Test":"TestA","Output":"some output\\n"}
{"Action":"pass","Package":"pkg1","Test":"TestA"}
{"Action":"output","Package":"pkg1","Test":"TestB","Output":"foo_test.go:10: failed\\n"}
{"Action":"fail","Package":"pkg1","Test":"TestB"}
    `
    const logPath = path.join(tempDir, 'output.txt')
    await fs.writeFile(logPath, log.trim())

    const input = createMockInput(logPath)
    const result = await goTestParser.parse(input)
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toEqual({
      ruleId: 'go-test/test-failed',
      message: 'foo_test.go:10: failed',
      file: 'foo_test.go',
      line: 10,
      column: 1,
      severity: 'error'
    })
  })

  it('detects panics', async () => {
    const log = `
{"Action":"output","Package":"pkg1","Test":"TestPanic","Output":"panic: assignment to entry in nil map\\n"}
{"Action":"output","Package":"pkg1","Test":"TestPanic","Output":"goroutine 1 [running]:\\n"}
{"Action":"fail","Package":"pkg1","Test":"TestPanic"}
    `
    const logPath = path.join(tempDir, 'output.txt')
    await fs.writeFile(logPath, log.trim())

    const input = createMockInput(logPath)
    const result = await goTestParser.parse(input)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0].ruleId).toBe('go-test/panic')
    expect(result.findings[0].file).toBe('pkg1') // fallback because no file_test.go found
  })

  it('parses text fallback for build errors', async () => {
    const log = `
{"Action":"output","Package":"pkg2","Output":"pkg2/main.go:5:1: expected declaration\\n"}
{"Action":"fail","Package":"pkg2"}
    `
    const logPath = path.join(tempDir, 'output.txt')
    await fs.writeFile(logPath, log.trim())

    const input = createMockInput(logPath)
    const result = await goTestParser.parse(input)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toMatchObject({
      ruleId: 'go-test/build-failed',
      message: 'expected declaration',
      file: 'pkg2/main.go',
      line: 5,
      column: 1,
      severity: 'error'
    })
  })
})
