#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const targetDir = path.resolve(__dirname, '../src/relay/__fixtures__/coverage-go')

console.log(`Target fixture directory: ${targetDir}`)
if (!fs.existsSync(targetDir)) {
  fs.mkdirSync(targetDir, { recursive: true })
}

const manifestPath = path.join(targetDir, 'MANIFEST.json')
if (fs.existsSync(manifestPath)) {
  console.log('Existing coverage-go MANIFEST.json found.')
} else {
  console.log('No go binary available on system; preserving static fixtures.')
}
