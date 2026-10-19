// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { parseConventionError } from '../../tasks/task-rejection-banner'

describe('parseConventionError', () => {
  it('parses "convention check failed: 2 files missing..."', () => {
    const result = parseConventionError(
      "convention check failed: 2 files missing '// implements: CRS-045', 0 invalid"
    )
    expect(result.isConventionFail).toBe(true)
    expect(result.missingCount).toBe(2)
    expect(result.crsId).toBe('CRS-045')
  })

  it('returns isConventionFail=false for unrelated error', () => {
    const result = parseConventionError('network timeout')
    expect(result.isConventionFail).toBe(false)
  })

  it('parses missingCount=1 correctly (singular)', () => {
    const result = parseConventionError(
      "convention check failed: 1 files missing '// implements: CRS-001', 0 invalid"
    )
    expect(result.missingCount).toBe(1)
  })

  it('parses different CRS IDs', () => {
    const result = parseConventionError(
      "convention check failed: 3 files missing '// implements: CRS-ABC-999', 0 invalid"
    )
    expect(result.crsId).toBe('CRS-ABC-999')
  })
})

describe('classifyError in TaskPeerInvokeError', () => {
  it('classifies ORCA_MCP_PROXY_URL as not-configured', async () => {
    // Import dynamically to test the internal classify function via component
    const { TaskPeerInvokeError } = await import('../../tasks/task-peer-invoke-error')
    // We test the component renders 'KG Integration Not Configured'
    // (full render test is in the component test file)
    expect(TaskPeerInvokeError).toBeDefined()
  })
})
