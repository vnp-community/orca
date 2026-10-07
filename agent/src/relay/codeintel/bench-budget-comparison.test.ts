import { describe, it, expect } from 'vitest'
import {
  compareBenchToBudgets,
  BenchBudgets,
  BenchReport
} from './bench-budget-comparison'

describe('bench-budget-comparison', () => {
  const sampleBudgets: BenchBudgets = {
    provenance: 'test',
    agent: {
      'cold-start': {
        totalMsMax: 1000,
        rssPeakKbMax: 50000,
        payloadBytesMax: 20000
      },
      'concurrency-test': {
        concurrencyMax: 3
      }
    }
  }

  it('reports no violations when report is within budget', () => {
    const report: BenchReport = {
      scenarios: [
        { scenario: 'cold-start', totalMs: 500, rssPeakKbMax: 40000, payloadBytes: 10000 },
        { scenario: 'concurrency-test', concurrency: 2 }
      ]
    }
    const violations = compareBenchToBudgets(report, sampleBudgets)
    expect(violations).toEqual([])
  })

  it('detects exceeded metrics for payloadBytes, rssPeakKbMax, concurrency', () => {
    const report: BenchReport = {
      scenarios: [
        { scenario: 'cold-start', totalMs: 1500, rssPeakKbMax: 60000, payloadBytes: 30000 },
        { scenario: 'concurrency-test', concurrency: 5 }
      ]
    }
    const violations = compareBenchToBudgets(report, sampleBudgets)
    expect(violations.length).toBe(4)

    const payloadV = violations.find(v => v.metric === 'payloadBytes')
    expect(payloadV?.kind).toBe('exceeded')
    expect(payloadV?.actual).toBe(30000)

    const rssV = violations.find(v => v.metric === 'rssPeakKbMax')
    expect(rssV?.kind).toBe('exceeded')
    expect(rssV?.actual).toBe(60000)

    const concurrencyV = violations.find(v => v.metric === 'concurrency')
    expect(concurrencyV?.kind).toBe('exceeded')
    expect(concurrencyV?.actual).toBe(5)
  })

  it('flags missing metric as a missing violation', () => {
    const report: BenchReport = {
      scenarios: [
        { scenario: 'cold-start', totalMs: 500, payloadBytes: 10000 }, // missing rssPeakKbMax
        { scenario: 'concurrency-test', concurrency: 2 }
      ]
    }
    const violations = compareBenchToBudgets(report, sampleBudgets)
    expect(violations.length).toBe(1)
    expect(violations[0].metric).toBe('rssPeakKbMax')
    expect(violations[0].kind).toBe('missing')
  })

  it('throws configuration error when budgets is malformed', () => {
    expect(() => compareBenchToBudgets({ scenarios: [] }, null as any)).toThrow(/Invalid budgets configuration/)
    expect(() => compareBenchToBudgets({ scenarios: [] }, {} as any)).toThrow(/Invalid budgets configuration/)
  })
})
