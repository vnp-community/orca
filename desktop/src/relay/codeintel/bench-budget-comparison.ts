export interface BudgetRule {
  payloadBytesMax?: number
  rssPeakKbMax?: number
  totalMsMax?: number
  concurrencyMax?: number
}

export interface BenchBudgets {
  provenance: string
  agent: Record<string, BudgetRule>
}

export interface BenchReportItem {
  scenario: string
  payloadBytes?: number
  rssPeakKbMax?: number
  totalMs?: number
  concurrency?: number
}

export interface BenchReport {
  scenarios: BenchReportItem[]
}

export interface BudgetViolation {
  scenario: string
  metric: string
  actual?: number
  max?: number
  kind: 'exceeded' | 'missing'
}

export function compareBenchToBudgets(report: BenchReport, budgets: BenchBudgets): BudgetViolation[] {
  if (!budgets || typeof budgets !== 'object' || !budgets.agent || typeof budgets.agent !== 'object') {
    throw new Error('Invalid budgets configuration: missing agent section')
  }

  const violations: BudgetViolation[] = []
  const reportMap = new Map<string, BenchReportItem>()
  if (report && Array.isArray(report.scenarios)) {
    for (const item of report.scenarios) {
      reportMap.set(item.scenario, item)
    }
  }

  for (const [scenarioName, rules] of Object.entries(budgets.agent)) {
    const item = reportMap.get(scenarioName)
    if (!item) {
      violations.push({
        scenario: scenarioName,
        metric: 'scenario',
        kind: 'missing'
      })
      continue
    }

    if (rules.payloadBytesMax !== undefined) {
      if (item.payloadBytes === undefined) {
        violations.push({ scenario: scenarioName, metric: 'payloadBytes', kind: 'missing' })
      } else if (item.payloadBytes > rules.payloadBytesMax) {
        violations.push({ scenario: scenarioName, metric: 'payloadBytes', actual: item.payloadBytes, max: rules.payloadBytesMax, kind: 'exceeded' })
      }
    }

    if (rules.rssPeakKbMax !== undefined) {
      if (item.rssPeakKbMax === undefined) {
        violations.push({ scenario: scenarioName, metric: 'rssPeakKbMax', kind: 'missing' })
      } else if (item.rssPeakKbMax > rules.rssPeakKbMax) {
        violations.push({ scenario: scenarioName, metric: 'rssPeakKbMax', actual: item.rssPeakKbMax, max: rules.rssPeakKbMax, kind: 'exceeded' })
      }
    }

    if (rules.totalMsMax !== undefined) {
      if (item.totalMs === undefined) {
        violations.push({ scenario: scenarioName, metric: 'totalMs', kind: 'missing' })
      } else if (item.totalMs > rules.totalMsMax) {
        violations.push({ scenario: scenarioName, metric: 'totalMs', actual: item.totalMs, max: rules.totalMsMax, kind: 'exceeded' })
      }
    }

    if (rules.concurrencyMax !== undefined) {
      if (item.concurrency === undefined) {
        violations.push({ scenario: scenarioName, metric: 'concurrency', kind: 'missing' })
      } else if (item.concurrency > rules.concurrencyMax) {
        violations.push({ scenario: scenarioName, metric: 'concurrency', actual: item.concurrency, max: rules.concurrencyMax, kind: 'exceeded' })
      }
    }
  }

  return violations
}
