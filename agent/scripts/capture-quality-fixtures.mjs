import fs from 'fs'
import path from 'path'
import os from 'os'
import crypto from 'crypto'
import { exec as execCommand } from 'child_process'
import { promisify } from 'util'

const exec = promisify(execCommand)

async function run(cmd, args, cwd) {
  try {
    const { stdout, stderr } = await exec(cmd + ' ' + args.join(' '), { cwd, timeout: 60000 })
    return { ok: true, stdout, stderr, code: 0 }
  } catch (e) {
    return { ok: false, stdout: e.stdout || '', stderr: e.stderr || '', code: e.code || 1, error: e.message }
  }
}

async function captureTool(toolName, cmd, args, versionArgs, getOutput, cwd, repoRoot) {
  console.log(`Capturing ${toolName}...`)
  
  const v = await run(cmd, versionArgs, cwd)
  if (v.error && !v.stdout && !v.stderr) {
    console.log(`  Skipped (not found): ${v.error}`)
    return
  }

  let verString = (v.stdout + '\\n' + v.stderr).trim().split('\\n')[0].replace(/[^a-zA-Z0-9.-]/g, '_')
  verString = verString.substring(0, 30)
  
  const res = await run(cmd, args, cwd)
  let rawOutput = await getOutput(res, cwd)
  
  // Redact paths
  let redacted = rawOutput.replace(new RegExp(cwd, 'g'), '<cwd>')
  
  const dir = path.join(repoRoot, 'agent', 'src', 'relay', '__fixtures__', 'quality', toolName, verString)
  fs.mkdirSync(dir, { recursive: true })
  
  const outPath = path.join(dir, 'output.txt')
  fs.writeFileSync(outPath, redacted)
  
  const sha256 = crypto.createHash('sha256').update(redacted).digest('hex')
  
  const manifest = {
    tool: toolName,
    toolVersion: verString,
    argv: args,
    markers: {}, // Any special markers
    capturedAt: new Date().toISOString(),
    sha256
  }
  
  fs.writeFileSync(path.join(dir, 'MANIFEST.json'), JSON.stringify(manifest, null, 2))
  console.log(`  Done ${toolName} ${verString}`)
}

async function main() {
  const repoRoot = path.resolve(process.cwd())
  const miniRepoPath = path.join(repoRoot, 'agent', 'src', 'relay', '__fixtures__', 'quality-mini-repo')
  
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-quality-'))
  console.log(`Using temp dir: ${tmpDir}`)
  
  fs.cpSync(miniRepoPath, tmpDir, { recursive: true })
  
  // Setup git
  await run('git', ['init'], tmpDir)
  await run('git', ['config', 'user.name', '"Test"'], tmpDir)
  await run('git', ['config', 'user.email', '"test@test.com"'], tmpDir)
  await run('git', ['add', '.'], tmpDir)
  await run('git', ['commit', '-m', '"Initial commit"'], tmpDir)
  
  // For buf breaking we need a second commit
  await run('git', ['branch', '-M', 'main'], tmpDir)
  
  // Install npm deps if needed for npx/pnpm exec
  console.log('Installing npm dependencies...')
  await run('npm', ['install'], tmpDir)
  
  await run('git', ['commit', '--allow-empty', '-m', '"Second commit"'], tmpDir)
  
  const tools = [
    {
      name: 'oxlint',
      cmd: 'npx',
      args: ['--no-install', 'oxlint', '--format', 'json'],
      versionArgs: ['--no-install', 'oxlint', '--version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    },
    {
      name: 'vitest',
      cmd: 'npx',
      args: ['vitest', 'run', '--reporter=json', '--outputFile=vitest-out.json'],
      versionArgs: ['vitest', '--version'],
      getOutput: async (res, cwd) => {
        try {
          return fs.readFileSync(path.join(cwd, 'vitest-out.json'), 'utf8')
        } catch {
          return res.stdout + '\\n' + res.stderr
        }
      }
    },
    {
      name: 'tsc',
      cmd: 'npx',
      args: ['tsc', '--pretty', 'false'],
      versionArgs: ['tsc', '--version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    },
    {
      name: 'go_vet',
      cmd: 'go',
      args: ['vet', '-json', './...'],
      versionArgs: ['version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    },
    {
      name: 'go_test',
      cmd: 'go',
      args: ['test', '-json', './...'],
      versionArgs: ['version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    },
    {
      name: 'golangci_lint',
      cmd: 'golangci-lint',
      args: ['run', '--out-format', 'json'],
      versionArgs: ['--version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    },
    {
      name: 'buf_lint',
      cmd: 'buf',
      args: ['lint', '--error-format', 'json'],
      versionArgs: ['--version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    },
    {
      name: 'buf_breaking',
      cmd: 'buf',
      args: ['breaking', '--against', '.git#branch=main', '--error-format', 'json'],
      versionArgs: ['--version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    },
    {
      name: 'opa_test',
      cmd: 'opa',
      args: ['test', '--format', 'json', '.'],
      versionArgs: ['version'],
      getOutput: (res) => res.stdout + '\\n' + res.stderr
    }
  ]

  for (const t of tools) {
    await captureTool(t.name, t.cmd, t.args, t.versionArgs, t.getOutput, tmpDir, repoRoot)
  }
  
  const diff = await run('git', ['diff', '--stat'], tmpDir)
  console.log('\\nGit Diff Stat:')
  console.log(diff.stdout)
}

main().catch(console.error)
