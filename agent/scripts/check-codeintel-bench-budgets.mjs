#!/usr/bin/env node
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
import { compareBenchToBudgets } from '../src/relay/codeintel/bench-budget-comparison.ts'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

const reportPath = process.argv[2]
const budgetsPath = process.argv[3] || path.join(__dirname, 'codeintel-budgets.json')

if (!reportPath) {
  console.error('Usage: node check-codeintel-bench-budgets.mjs <report.json> [budgets.json]')
  process.exit(1)
}

if (!fs.existsSync(reportPath)) {
  console.error(`Report file not found: ${reportPath}`)
  process.exit(1)
}

if (!fs.existsSync(budgetsPath)) {
  console.error(`Budgets file not found: ${budgetsPath}`)
  process.exit(1)
}

try {
  const report = JSON.parse(fs.readFileSync(reportPath, 'utf8'))
  const budgets = JSON.parse(fs.readFileSync(budgetsPath, 'utf8'))

  const violations = compareBenchToBudgets(report, budgets)

  if (violations.length === 0) {
    console.log('[check-codeintel-bench-budgets] OK: All scenarios meet performance budgets.')
    process.exit(0)
  } else {
    console.error(`[check-codeintel-bench-budgets] FAILED: ${violations.length} budget violation(s) found:`)
    console.table(violations)
    process.exit(1)
  }
} catch (err) {
  console.error('[check-codeintel-bench-budgets] Error running budget check:', err.message)
  process.exit(1)
}
