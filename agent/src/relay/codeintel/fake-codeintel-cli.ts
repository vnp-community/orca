import fs from 'fs'
import path from 'path'

export interface FakeCliOptions {
  toolName: string
  dir: string
  delayMs?: number
  stdout?: string
}

export function createFakeCli(opts: FakeCliOptions): string {
  const filePath = path.join(opts.dir, opts.toolName)
  const lockDir = path.join(opts.dir, `locks-${opts.toolName}`)
  if (!fs.existsSync(lockDir)) {
    fs.mkdirSync(lockDir, { recursive: true })
  }

  const scriptContent = `#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

const lockDir = ${JSON.stringify(lockDir)};
const id = process.pid;
const lockFile = path.join(lockDir, String(id));

fs.writeFileSync(lockFile, '1');

setTimeout(() => {
  try { fs.unlinkSync(lockFile); } catch {}
  process.stdout.write(${JSON.stringify(opts.stdout ?? '{"ok":true}')});
  process.exit(0);
}, ${opts.delayMs ?? 50});
`

  fs.writeFileSync(filePath, scriptContent, { mode: 0o755 })
  return filePath
}

export function countRunningInstances(dir: string, toolName: string): number {
  const lockDir = path.join(dir, `locks-${toolName}`)
  if (!fs.existsSync(lockDir)) return 0
  return fs.readdirSync(lockDir).length
}
