export function summaryWire(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    summary: 'Adds a weekly filter to the task list.',
    risks: [{ text: 'Date math near DST', refs: ['src/a.ts'] }],
    readFirst: [{ file: 'src/a.ts', why: 'entry point' }],
    model: 'claude-x',
    level: 'metadata',
    promptVersion: 'v1',
    inputDigest: 'abc',
    generatedAt: '2026-10-07T10:00:00Z',
    refsDropped: 0,
    ...over
  }
}

export function manifestWire(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    level: 'metadata',
    files: [
      { path: 'src/a.ts', bytes: 120, hunks: 2 },
      { path: '.env', bytes: 0, hunks: 0, withheld: 'secret' }
    ],
    findingsCount: 3,
    redactions: 2,
    totalBytes: 120,
    estimatedTokens: 2100,
    provider: 'acme-llm',
    suspectedInjection: false,
    ...over
  }
}
