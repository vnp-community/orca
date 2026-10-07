import { execSync } from 'child_process'
import path from 'path'
import os from 'os'
import fs from 'fs'
import crypto from 'crypto'

function run(cmd, cwd) {
  try {
    return execSync(cmd, { cwd, encoding: 'utf8', env: { ...process.env, GITNEXUS_WORKER_POOL_SIZE: '1' } })
  } catch (err) {
    if (err.stdout) return err.stdout
    if (err.stderr) return err.stderr
    return err.message
  }
}

function redact(text, repoRoot, wtRoot) {
  if (!text) return text
  return text.replaceAll(repoRoot, '<repo>').replaceAll(wtRoot, '<worktree>')
}

const tools = [
  { cmd: 'gitnexus --version', name: 'gitnexus', regex: /^([\d.a-z-]+)/ },
  { cmd: 'codegraph --version', name: 'codegraph', regex: /^([\d.a-z-]+)/ }
]

async function main() {
  const versions = {}
  for (const t of tools) {
    try {
      const out = run(t.cmd, process.cwd())
      const m = out.match(t.regex)
      if (m) {
        versions[t.name] = m[1]
      } else {
        console.warn(`Could not determine version for ${t.name}: ${out}`)
      }
    } catch {
      console.warn(`${t.name} not available`)
    }
  }

  const tmpBase = fs.mkdtempSync(path.join(os.tmpdir(), 'index-basis-'))
  const repoRoot = path.join(tmpBase, 'repo')
  const wtRoot = path.join(tmpBase, 'wt')

  try {
    fs.mkdirSync(repoRoot)
    run('git init', repoRoot)
    fs.writeFileSync(path.join(repoRoot, 'main.go'), 'package main\n')
    run('git add main.go', repoRoot)
    run('git commit -m "init"', repoRoot)
    
    fs.writeFileSync(path.join(repoRoot, 'main.go'), 'package main\nfunc main() {}\n')
    run('git commit -am "add main"', repoRoot)

    run(`git worktree add ${wtRoot} -b wt-branch`, repoRoot)

    // Run indexing
    if (versions.gitnexus) {
      run('gitnexus analyze --index-only', repoRoot)
    }
    if (versions.codegraph) {
      run('codegraph init', repoRoot)
      run('codegraph index', repoRoot)
    }

    const fixturesDir = path.join(process.cwd(), 'src/relay/__fixtures__/index-basis')
    if (!fs.existsSync(fixturesDir)) fs.mkdirSync(fixturesDir, { recursive: true })

    const scenarios = [
      { name: 'main-clean', cwd: repoRoot, setup: () => { run('git reset --hard', repoRoot) } },
      { name: 'main-dirty', cwd: repoRoot, setup: () => { fs.writeFileSync(path.join(repoRoot, 'main.go'), '// dirty\n') } },
      { name: 'wt-clean', cwd: wtRoot, setup: () => { run('git reset --hard', wtRoot) } },
      { name: 'wt-dirty', cwd: wtRoot, setup: () => { fs.writeFileSync(path.join(wtRoot, 'main.go'), '// dirty\n') } }
    ]

    const files = {}

    for (const sc of scenarios) {
      sc.setup()

      if (versions.gitnexus) {
        const out = redact(run('gitnexus status', sc.cwd), repoRoot, wtRoot)
        const fn = `gitnexus-status-${sc.name}.txt`
        fs.writeFileSync(path.join(fixturesDir, fn), out)
        files[fn] = { sha256: crypto.createHash('sha256').update(out).digest('hex'), bytes: Buffer.byteLength(out) }
      }

      if (versions.codegraph) {
        const out = redact(run('codegraph status -j', sc.cwd), repoRoot, wtRoot)
        const fn = `codegraph-status-${sc.name}.json`
        fs.writeFileSync(path.join(fixturesDir, fn), out)
        files[fn] = { sha256: crypto.createHash('sha256').update(out).digest('hex'), bytes: Buffer.byteLength(out) }
      }
    }

    // Capture meta.json
    if (versions.gitnexus) {
      const metaPath = path.join(repoRoot, '.gitnexus/meta.json')
      if (fs.existsSync(metaPath)) {
        const out = redact(fs.readFileSync(metaPath, 'utf8'), repoRoot, wtRoot)
        fs.writeFileSync(path.join(fixturesDir, 'meta.json'), out)
        files['meta.json'] = { sha256: crypto.createHash('sha256').update(out).digest('hex'), bytes: Buffer.byteLength(out) }
      }
    }

    const manifest = {
      createdAt: new Date().toISOString(),
      tools: versions,
      files
    }
    fs.writeFileSync(path.join(fixturesDir, 'MANIFEST.json'), JSON.stringify(manifest, null, 2))
    console.log('Capture complete. Saved to', fixturesDir)

  } finally {
    fs.rmSync(tmpBase, { recursive: true, force: true })
  }
}

main()
