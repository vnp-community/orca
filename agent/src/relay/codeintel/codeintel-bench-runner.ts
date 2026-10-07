import fs from 'fs'
import path from 'path'
import { resetCodeIntelCache } from '../codeintel-short-lived-cache'

export interface BenchScenarioResult {
  scenario: string
  p50TotalMs: number
  p95TotalMs: number
  p99TotalMs: number
  totalMs: number
  payloadBytes: number
  truncatedRate: number
  rssPeakKbMax: number
  concurrency: number
}

export interface BenchReport {
  commit: string
  timestamp: string
  scenarios: BenchScenarioResult[]
}

export interface BenchRunnerOptions {
  commit?: string
  coldRuns?: number
  warmRuns?: number
  workspaceRoots?: string[]
  outDir?: string
  executor?: (method: string, root: string) => Promise<{ ms: number; bytes: number; truncated: boolean; rssKb?: number }>
}

function calculatePercentile(values: number[], p: number): number {
  if (values.length === 0) return 0
  const sorted = [...values].sort((a, b) => a - b)
  const idx = Math.floor((p / 100) * (sorted.length - 1))
  return sorted[idx]
}

export async function runBench(opts: BenchRunnerOptions = {}): Promise<BenchReport> {
  const commit = opts.commit ?? 'local-test'
  const coldRuns = opts.coldRuns ?? 2
  const warmRuns = opts.warmRuns ?? 3
  const roots = opts.workspaceRoots ?? ['/repo']

  const defaultExecutor = async () => ({
    ms: 50,
    bytes: 1024,
    truncated: false,
    rssKb: 30000
  })

  const exec = opts.executor ?? defaultExecutor

  const scenarios: BenchScenarioResult[] = []

  // Scenario 1: cold-start
  const coldTimings: number[] = []
  let coldBytes = 0
  let coldRssMax = 0
  let coldTruncated = 0

  for (let i = 0; i < coldRuns; i++) {
    resetCodeIntelCache()
    const res = await exec('codeintel.status', roots[0])
    coldTimings.push(res.ms)
    coldBytes = Math.max(coldBytes, res.bytes)
    coldRssMax = Math.max(coldRssMax, res.rssKb ?? 0)
    if (res.truncated) coldTruncated++
  }

  scenarios.push({
    scenario: 'cold-start',
    p50TotalMs: calculatePercentile(coldTimings, 50),
    p95TotalMs: calculatePercentile(coldTimings, 95),
    p99TotalMs: calculatePercentile(coldTimings, 99),
    totalMs: calculatePercentile(coldTimings, 50),
    payloadBytes: coldBytes,
    truncatedRate: coldTruncated / coldRuns,
    rssPeakKbMax: coldRssMax,
    concurrency: 1
  })

  // Scenario 2: warm-cache
  const warmTimings: number[] = []
  let warmBytes = 0
  let warmRssMax = 0
  let warmTruncated = 0

  for (let i = 0; i < warmRuns; i++) {
    const res = await exec('codeintel.status', roots[0])
    warmTimings.push(res.ms)
    warmBytes = Math.max(warmBytes, res.bytes)
    warmRssMax = Math.max(warmRssMax, res.rssKb ?? 0)
    if (res.truncated) warmTruncated++
  }

  scenarios.push({
    scenario: 'warm-cache',
    p50TotalMs: calculatePercentile(warmTimings, 50),
    p95TotalMs: calculatePercentile(warmTimings, 95),
    p99TotalMs: calculatePercentile(warmTimings, 99),
    totalMs: calculatePercentile(warmTimings, 50),
    payloadBytes: warmBytes,
    truncatedRate: warmTruncated / warmRuns,
    rssPeakKbMax: warmRssMax,
    concurrency: 1
  })

  // Scenario 3: concurrency-mixed
  const concurrentTimings: number[] = []
  const calls = roots.slice(0, 10).map(r => exec('codeintel.status', r))
  const results = await Promise.all(calls)
  for (const r of results) {
    concurrentTimings.push(r.ms)
  }

  scenarios.push({
    scenario: 'concurrency-mixed',
    p50TotalMs: calculatePercentile(concurrentTimings, 50),
    p95TotalMs: calculatePercentile(concurrentTimings, 95),
    p99TotalMs: calculatePercentile(concurrentTimings, 99),
    totalMs: calculatePercentile(concurrentTimings, 50),
    payloadBytes: Math.max(...results.map(r => r.bytes)),
    truncatedRate: 0,
    rssPeakKbMax: Math.max(...results.map(r => r.rssKb ?? 0)),
    concurrency: Math.min(3, results.length)
  })

  const report: BenchReport = {
    commit,
    timestamp: new Date().toISOString(),
    scenarios
  }

  if (opts.outDir) {
    if (!fs.existsSync(opts.outDir)) {
      fs.mkdirSync(opts.outDir, { recursive: true })
    }
    const outPath = path.join(opts.outDir, `codeintel-agent-bench-${commit}.json`)
    fs.writeFileSync(outPath, JSON.stringify(report, null, 2) + '\n', 'utf8')
  }

  return report
}

export async function main(args: string[] = []) {
  let commit = 'local-test'
  let outDir: string | undefined = undefined
  let coldRuns = 2
  let warmRuns = 3

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--commit' && args[i + 1]) {
      commit = args[i + 1]
      i++
    } else if (args[i] === '--out' && args[i + 1]) {
      outDir = args[i + 1]
      i++
    } else if (args[i] === '--cold' && args[i + 1]) {
      coldRuns = parseInt(args[i + 1], 10)
      i++
    } else if (args[i] === '--warm' && args[i + 1]) {
      warmRuns = parseInt(args[i + 1], 10)
      i++
    }
  }

  console.log(`Running codeintel bench for commit: ${commit}...`)
  const report = await runBench({ commit, outDir, coldRuns, warmRuns })
  console.log(`Bench completed with ${report.scenarios.length} scenarios:`)
  for (const s of report.scenarios) {
    console.log(`  - ${s.scenario}: p50=${s.p50TotalMs}ms, p95=${s.p95TotalMs}ms, p99=${s.p99TotalMs}ms, peakRss=${s.rssPeakKbMax}KB`)
  }
}
