#!/usr/bin/env node
import fs from 'fs/promises'
import path from 'node:path'
import { spawn } from 'node:child_process'
import crypto from 'node:crypto'
import os from 'node:os'

const REPO_ROOT = path.resolve(import.meta.dirname, '../../')
const FIXTURES_DIR = path.join(import.meta.dirname, '../src/relay/__fixtures__/quality-rules')

const SCRIPTS = [
  { name: 'check-max-lines-ratchet', script: 'desktop/config/scripts/check-max-lines-ratchet.mjs', args: [], cwd: REPO_ROOT },
  { name: 'check-styled-scrollbars', script: 'desktop/config/scripts/check-styled-scrollbars.mjs', args: [], cwd: REPO_ROOT },
  { name: 'check-reliability-gates', script: 'desktop/config/scripts/check-reliability-gates.mjs', args: [], cwd: REPO_ROOT },
  { name: 'verify-localization-catalog', script: 'desktop/config/scripts/verify-localization-catalog.mjs', args: [], cwd: REPO_ROOT },
  { name: 'audit-localization-coverage', script: 'desktop/config/scripts/audit-localization-coverage.mjs', args: ['--check'], cwd: REPO_ROOT },
  { name: 'check-feature-wall-assets', script: 'desktop/config/scripts/check-feature-wall-assets.mjs', args: [], cwd: REPO_ROOT }
]

function getSha(text) {
  return crypto.createHash('sha1').update(text).digest('hex')
}

async function capture() {
  await fs.mkdir(FIXTURES_DIR, { recursive: true })

  for (const item of SCRIPTS) {
    const scriptPath = path.join(REPO_ROOT, item.script)
    let scriptContent = ''
    try {
      scriptContent = await fs.readFile(scriptPath, 'utf8')
    } catch (e) {
      console.log(`[SKIP] Script not found: ${item.script}`)
      continue
    }

    const sha = getSha(scriptContent)
    const dir = path.join(FIXTURES_DIR, item.name, sha)
    await fs.mkdir(dir, { recursive: true })

    console.log(`Running ${item.name} ...`)
    const start = performance.now()
    const proc = spawn('node', [scriptPath, ...item.args], { cwd: item.cwd, stdio: 'pipe' })
    
    let stdout = ''
    let stderr = ''
    proc.stdout.on('data', chunk => stdout += chunk.toString('utf8'))
    proc.stderr.on('data', chunk => stderr += chunk.toString('utf8'))

    const exitCode = await new Promise(resolve => {
      proc.on('close', resolve)
      proc.on('error', () => resolve(1))
    })

    const durationMs = Math.round(performance.now() - start)

    const home = os.homedir()
    const sanitize = (text) => {
      let t = text.split(REPO_ROOT).join('<repo>')
      t = t.split(home).join('<home>')
      return t
    }

    // truncate 20KiB max
    const MAX_BYTES = 20480
    const truncate = (text) => {
      const buf = Buffer.from(text, 'utf8')
      if (buf.length > MAX_BYTES) return buf.subarray(0, MAX_BYTES).toString('utf8') + '...[TRUNCATED]'
      return text
    }

    await fs.writeFile(path.join(dir, 'stdout.txt'), truncate(sanitize(stdout)))
    await fs.writeFile(path.join(dir, 'stderr.txt'), truncate(sanitize(stderr)))

    const manifest = {
      script: item.script,
      args: item.args,
      cwd: item.cwd === REPO_ROOT ? '<repo>' : item.cwd,
      exitCode,
      durationMs,
      scriptSha: sha,
      env: {}
    }

    await fs.writeFile(path.join(dir, 'MANIFEST.json'), JSON.stringify(manifest, null, 2))
    console.log(`[OK] Captured ${item.name}`)
  }
}

capture().catch(console.error)
