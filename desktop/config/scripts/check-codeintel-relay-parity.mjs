import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export function isCodeIntelCoreFile(filename) {
  if (filename.endsWith('.test.ts') || filename.endsWith('.test.mjs')) return false
  if (filename === 'agent-rpc-dispatch-codeintel.ts') return false
  return (
    filename.startsWith('codeintel-') ||
    filename.startsWith('gitnexus-') ||
    filename.startsWith('codegraph-')
  ) && filename.endsWith('.ts')
}

export function checkCodeIntelRelayParity(agentRelayDir, desktopRelayDir) {
  const agentFiles = fs.readdirSync(agentRelayDir).filter(isCodeIntelCoreFile)
  const mismatches = []

  for (const file of agentFiles) {
    const agentFilePath = path.join(agentRelayDir, file)
    const desktopFilePath = path.join(desktopRelayDir, file)

    if (!fs.existsSync(desktopFilePath)) {
      mismatches.push({ file, issue: 'missing in desktop relay' })
      continue
    }

    const agentContent = fs.readFileSync(agentFilePath, 'utf8')
    const desktopContent = fs.readFileSync(desktopFilePath, 'utf8')

    if (agentContent !== desktopContent) {
      mismatches.push({ file, issue: 'content mismatch' })
    }
  }

  return {
    checkedCount: agentFiles.length,
    mismatches
  }
}

// Entrypoint execution
const __filename = fileURLToPath(import.meta.url)
if (process.argv[1] === __filename) {
  const repoRoot = path.resolve(path.dirname(__filename), '../../..')
  const agentRelayDir = path.join(repoRoot, 'agent', 'src', 'relay')
  const desktopRelayDir = path.join(repoRoot, 'desktop', 'src', 'relay')

  const result = checkCodeIntelRelayParity(agentRelayDir, desktopRelayDir)

  if (result.mismatches.length > 0) {
    console.error(`[check-codeintel-relay-parity] FAILED: ${result.mismatches.length} file mismatches found:`)
    for (const m of result.mismatches) {
      console.error(`  - ${m.file}: ${m.issue}`)
    }
    process.exit(1)
  }

  console.log(`[check-codeintel-relay-parity] OK: ${result.checkedCount} core files checked in parity.`)
  process.exit(0)
}
