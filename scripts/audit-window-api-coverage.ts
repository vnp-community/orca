#!/usr/bin/env tsx
// Why: audit script verifies web-preload-api.ts covers all window.api.*
// calls found in hook files so web mode never silently returns undefined.

// Pre-existing mismatch, unrelated to the monorepo split: this repo pins
// glob@7 (exports `sync`), but the script imports the glob@9+ `globSync` name.
import { sync as globSync } from 'glob'
import { readFileSync } from 'node:fs'

// Was 'src/renderer/...' (repo-root renderer source); that tree no longer
// exists after the monorepo split — the renderer now lives under frontend/.
const HOOKS_GLOB = 'frontend/src/renderer/src/hooks/**/*.ts'
const WEB_PRELOAD = 'frontend/src/renderer/src/web/web-preload-api.ts'

const hookFiles = globSync(HOOKS_GLOB)
const apiCalls = new Map<string, Set<string>>()

for (const file of hookFiles) {
  const src = readFileSync(file, 'utf-8')

  // window.api.namespace.method
  for (const match of src.matchAll(/window\.api\.(\w+)\.(\w+)/g)) {
    const ns = match[1]
    const method = match[2]
    if (!apiCalls.has(ns)) {apiCalls.set(ns, new Set())}
    apiCalls.get(ns)!.add(method)
  }

  // window.api.onSomething (top-level event handlers)
  for (const match of src.matchAll(/window\.api\.(on\w+)/g)) {
    if (!apiCalls.has('_root')) {apiCalls.set('_root', new Set())}
    apiCalls.get('_root')!.add(match[1])
  }
}

const preloadSrc = readFileSync(WEB_PRELOAD, 'utf-8')
let missing = 0

console.log('\n=== Window.api Coverage Audit ===\n')

for (const [ns, methods] of apiCalls) {
  for (const method of methods) {
    const label = ns !== '_root' ? `window.api.${ns}.${method}` : `window.api.${method}`
    if (!preloadSrc.includes(method)) {
      console.log(`❌ MISSING: ${label}`)
      missing++
    } else {
      console.log(`✅ OK: ${label}`)
    }
  }
}

if (missing > 0) {
  console.error(`\n❌ ${missing} API method(s) missing from web-preload-api.ts`)
  process.exit(1)
} else {
  console.log('\n✅ All window.api methods covered in web-preload-api.ts')
}
