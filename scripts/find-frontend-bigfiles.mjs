#!/usr/bin/env node
// scripts/find-frontend-bigfiles.mjs
//
// Why: surfaces frontend/src source files over a line-count threshold so they
// can be tracked as split-file bugs (see specs/frontend/bugs/bigfile_v1/).
// This is intentionally separate from the oxlint max-lines ratchet
// (config/max-lines-baseline.txt / config/scripts/check-max-lines-ratchet.mjs)
// — that ratchet only tracks files carrying an active max-lines disable
// comment, not every file that happens to be large but still passes lint
// (e.g. a .tsx file under the 400-line oxlint budget isn't flagged there, but
// a 1200-line file well past any reasonable review/maintenance size is still
// worth tracking here regardless of whether it currently lints clean).
//
// Zero dependencies (pure node:fs), so it runs the same on macOS/Linux/
// Windows without relying on `find`/`wc` being on PATH.
//
// Usage:
//   node scripts/find-frontend-bigfiles.mjs [--threshold=1000] [--format=table|json|csv]
//   [--include-tests] [--root=frontend/src]

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, extname } from 'node:path'
import process from 'node:process'

const DEFAULT_THRESHOLD = 1000
const DEFAULT_ROOT = 'frontend/src'
const SOURCE_EXTENSIONS = new Set(['.ts', '.tsx'])
const SKIP_DIR_NAMES = new Set(['node_modules', 'out', 'dist', 'build', 'coverage', '.git'])

function parseArgs(argv) {
  const flags = {
    threshold: DEFAULT_THRESHOLD,
    format: 'table',
    includeTests: false,
    root: DEFAULT_ROOT
  }
  for (const arg of argv) {
    if (arg.startsWith('--threshold=')) {
      const value = Number(arg.slice('--threshold='.length))
      if (Number.isFinite(value) && value > 0) {
        flags.threshold = value
      }
    } else if (arg.startsWith('--format=')) {
      const value = arg.slice('--format='.length)
      if (value === 'json' || value === 'csv' || value === 'table') {
        flags.format = value
      }
    } else if (arg === '--include-tests') {
      flags.includeTests = true
    } else if (arg.startsWith('--root=')) {
      flags.root = arg.slice('--root='.length)
    }
  }
  return flags
}

function isTestFile(filePath) {
  return /\.(test|spec)\.tsx?$/.test(filePath)
}

/** Counts lines the way editors/oxlint report them: a trailing newline does
 *  not count as an extra blank final line. */
function countLines(filePath) {
  const text = readFileSync(filePath, 'utf-8')
  if (text.length === 0) {
    return 0
  }
  const parts = text.split('\n')
  return parts.at(-1) === '' ? parts.length - 1 : parts.length
}

function* walk(dir) {
  let entries
  try {
    entries = readdirSync(dir, { withFileTypes: true })
  } catch {
    return
  }
  for (const entry of entries) {
    if (entry.isDirectory()) {
      if (SKIP_DIR_NAMES.has(entry.name)) {
        continue
      }
      yield* walk(join(dir, entry.name))
      continue
    }
    if (SOURCE_EXTENSIONS.has(extname(entry.name))) {
      yield join(dir, entry.name)
    }
  }
}

function collectEntries({ root, threshold, includeTests }) {
  const entries = []
  for (const filePath of walk(root)) {
    if (filePath.endsWith('.d.ts')) {
      continue
    }
    const isTest = isTestFile(filePath)
    if (isTest && !includeTests) {
      continue
    }
    const lines = countLines(filePath)
    if (lines <= threshold) {
      continue
    }
    entries.push({ file: relative(process.cwd(), filePath).split('\\').join('/'), lines, isTest })
  }
  entries.sort((a, b) => b.lines - a.lines)
  return entries
}

function printTable(entries, threshold, root) {
  console.log(`\n=== Files over ${threshold} lines under ${root} (${entries.length} found) ===\n`)
  if (entries.length === 0) {
    console.log('(none)')
    return
  }
  const fileWidth = Math.max(...entries.map((e) => e.file.length), 4)
  for (const e of entries) {
    const marker = e.isTest ? '  [test]' : ''
    console.log(`${String(e.lines).padStart(6)}  ${e.file.padEnd(fileWidth)}${marker}`)
  }
  console.log('')
}

function printCsv(entries) {
  console.log('file,lines,isTest')
  for (const e of entries) {
    console.log(`${e.file},${e.lines},${e.isTest}`)
  }
}

function main() {
  const flags = parseArgs(process.argv.slice(2))
  const statResult = (() => {
    try {
      return statSync(flags.root)
    } catch {
      return null
    }
  })()
  if (!statResult?.isDirectory()) {
    console.error(`error: root path does not exist or is not a directory: ${flags.root}`)
    process.exitCode = 1
    return
  }

  const entries = collectEntries(flags)

  if (flags.format === 'json') {
    console.log(JSON.stringify({ threshold: flags.threshold, root: flags.root, entries }, null, 2))
  } else if (flags.format === 'csv') {
    printCsv(entries)
  } else {
    printTable(entries, flags.threshold, flags.root)
  }
}

main()
