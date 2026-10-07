import fs from 'fs'
import path from 'path'
import { exec as execCommand } from 'child_process'
import { promisify } from 'util'

const exec = promisify(execCommand)

async function run(cmd, args) {
  try {
    const { stdout, stderr } = await exec(cmd + ' ' + args.join(' '), { timeout: 10000 })
    return { ok: true, stdout, stderr }
  } catch (e) {
    // some tools exit non-zero for --help
    if (e.stdout || e.stderr) {
      return { ok: false, stdout: e.stdout || '', stderr: e.stderr || '' }
    }
    return { ok: false, error: e.message }
  }
}

async function capture(tool, verArgs, helpArgs) {
  console.log(`Capturing ${tool}...`)
  const v = await run(tool, verArgs)
  if (v.error) {
    console.log(`  Skipped (not found): ${v.error}`)
    return null
  }
  
  let verString = (v.stdout + '\\n' + v.stderr).trim().split('\\n')[0].replace(/[^a-zA-Z0-9.-]/g, '_')
  // For safety, truncate it
  verString = verString.substring(0, 30)

  const dir = path.join(process.cwd(), 'src', 'relay', '__fixtures__', 'quality-catalog', tool, verString)
  fs.mkdirSync(dir, { recursive: true })

  fs.writeFileSync(path.join(dir, 'version.txt'), v.stdout + '\\n' + v.stderr)

  for (const h of helpArgs) {
    const res = await run(tool, h.args)
    if (!res.error) {
      let out = (res.stdout + '\\n' + res.stderr).substring(0, 20480)
      out = out.replace(new RegExp(process.cwd(), 'g'), '<cwd>')
      fs.writeFileSync(path.join(dir, h.name + '.txt'), out)
    }
  }
  return verString
}

async function main() {
  const catalog = [
    { tool: 'npx --no-install oxlint', verArgs: ['--version'], helpArgs: [{ name: 'help', args: ['--help'] }] },
    { tool: 'pnpm exec vitest', verArgs: ['--version'], helpArgs: [{ name: 'help', args: ['--help'] }] },
    { tool: 'pnpm exec tsc', verArgs: ['--version'], helpArgs: [{ name: 'help', args: ['--help'] }] },
    { tool: 'go', verArgs: ['version'], helpArgs: [{ name: 'help-vet', args: ['help', 'vet'] }] },
    { tool: 'golangci-lint', verArgs: ['--version'], helpArgs: [{ name: 'help', args: ['--help'] }, { name: 'help-run', args: ['run', '--help'] }] },
    { tool: 'buf', verArgs: ['--version'], helpArgs: [{ name: 'help-lint', args: ['lint', '--help'] }, { name: 'help-breaking', args: ['breaking', '--help'] }] },
    { tool: 'opa', verArgs: ['version'], helpArgs: [{ name: 'help-test', args: ['test', '--help'] }] }
  ]

  const evidence = { tools: {} }
  for (const c of catalog) {
    const ver = await capture(c.tool, c.verArgs, c.helpArgs)
    if (ver) evidence.tools[c.tool] = ver
  }

  // Check some files exist (just hardcode a few for tests if we have them)
  const filesToCheck = [
    'tsconfig.json',
    'vitest.config.ts'
  ]
  evidence.files = {}
  for (const f of filesToCheck) {
    evidence.files[f] = fs.existsSync(path.join(process.cwd(), f))
  }

  const fixDir = path.join(process.cwd(), 'src', 'relay', '__fixtures__', 'quality-catalog')
  fs.mkdirSync(fixDir, { recursive: true })
  fs.writeFileSync(path.join(fixDir, 'evidence.json'), JSON.stringify(evidence, null, 2))
}

main().catch(console.error)
