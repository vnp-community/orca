import { spawnSync } from 'child_process'
import path from 'path'
import os from 'os'
import fs from 'fs'
import { runTypeScriptEntry } from './esbuild-run-typescript-entry.mjs'

async function main() {
  const args = process.argv.slice(2)
  if (args.includes('--help')) {
    console.log('Usage: node capture-codeintel-fixtures.mjs [--tool <gitnexus|codegraph>] [--version <ver>]')
    process.exit(0)
  }

  // Placeholder capture script
  console.log('Capture script running (not fully implemented).')
  console.log('It will use runTypeScriptEntry to load fixture-capture-plan, then run the tools in a temp dir.')
}

main().catch(err => {
  console.error(err)
  process.exit(1)
})
