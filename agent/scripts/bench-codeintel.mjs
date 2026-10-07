import path from 'path'
import { fileURLToPath } from 'url'
import { runTypeScriptEntry } from './esbuild-run-typescript-entry.mjs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const entryPoint = path.resolve(__dirname, '../src/relay/codeintel/codeintel-bench-runner.ts')

async function main() {
  const args = process.argv.slice(2)
  if (args.includes('--help')) {
    console.log('Usage: node bench-codeintel.mjs [--commit <sha>] [--out <dir>] [--cold <n>] [--warm <n>]')
    process.exit(0)
  }

  await runTypeScriptEntry(entryPoint, args)
}

main().catch(err => {
  console.error(err)
  process.exit(1)
})
