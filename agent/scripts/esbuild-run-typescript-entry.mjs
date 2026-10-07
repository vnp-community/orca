import * as esbuild from 'esbuild'
import path from 'path'
import os from 'os'
import fs from 'fs'

export async function runTypeScriptEntry(entryPoint, args) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'esbuild-run-'))
  const outfile = path.join(tmpDir, 'bundle.mjs')

  try {
    await esbuild.build({
      entryPoints: [entryPoint],
      bundle: true,
      platform: 'node',
      format: 'esm',
      outfile,
      external: ['esbuild', 'vitest', 'fsevents'] // mock or externalize
    })

    const module = await import(outfile)
    if (typeof module.main === 'function') {
      await module.main(args)
    } else {
      console.error(`No main function exported from ${entryPoint}`)
      process.exit(1)
    }
  } catch (err) {
    console.error(`Error running ${entryPoint}:`, err)
    process.exit(1)
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
}

// If invoked directly
if (process.argv[1] === new URL(import.meta.url).pathname) {
  const entryPoint = process.argv[2]
  const args = process.argv.slice(3)
  if (!entryPoint) {
    console.error('Usage: node esbuild-run-typescript-entry.mjs <entryPoint> [args...]')
    process.exit(1)
  }
  runTypeScriptEntry(entryPoint, args)
}
