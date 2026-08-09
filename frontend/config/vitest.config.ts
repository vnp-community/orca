import { resolve } from 'node:path'
import { defineConfig } from 'vitest/config'

const windowsTestWorkerOptions = process.platform === 'win32' ? { maxWorkers: 4 } : {}

// Why: process.cwd() is always the project root when tests are invoked via
// `pnpm test` or `pnpm exec vitest`. The explicit root option is required when
// the config file lives in a subdirectory (config/vitest.config.ts): without it
// Vitest infers the Vite project root from the config file location, causing
// @/ alias resolution and server.deps.inline to malfunction (aliases are
// computed relative to the wrong directory).
//
// Ported from desktop/config/vitest.config.ts — this "isolated copy, split
// from monorepo" package (see package.json description) never received its
// own test-runner config during the split. Added while executing
// specs/frontend/bugs/hld-v1/tasks/ so `pnpm test` can actually verify fixes.
const projectRoot = resolve(process.cwd())
const rendererSrc = resolve(projectRoot, 'src/renderer/src')

export default defineConfig({
  root: projectRoot,
  define: {
    ORCA_FEATURE_WALL_ENABLED: 'true'
  },
  resolve: {
    alias: {
      '@renderer': rendererSrc,
      '@': rendererSrc
    }
  },
  test: {
    environment: 'node',
    root: projectRoot,
    // Why: no tests/e2e/ dir in this package (unlike desktop/) — omitted rather
    // than left as copy-paste cruft that would silently match nothing.
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx', 'config/scripts/**/*.test.mjs'],
    // Why: the full suite runs heavy TS transforms plus real git/http fixtures;
    // the Vitest 5s defaults are too tight for the slowest integration cases.
    hookTimeout: 60_000,
    testTimeout: 30_000,
    // Why: Windows process and shell startup are slower under full-suite load;
    // macOS/Linux keep Vitest's default worker parallelism.
    ...windowsTestWorkerOptions,
    server: {
      deps: {
        // Why: Vitest node environment externalizes files that are not in
        // node_modules when they are imported transitively. This causes @/ alias
        // imports to fail with ERR_MODULE_NOT_FOUND because Node's native ESM
        // loader doesn't know about Vite aliases. Inlining renderer/src files
        // forces them through Vite's transform pipeline so alias resolution is
        // applied at every import level.
        //
        // Why regex /renderer\/src/ (not /src\/renderer\/src/): Vitest matches
        // server.deps.inline patterns against the RESOLVED absolute file path,
        // not the module specifier. Both patterns work with absolute paths;
        // using the shorter suffix is sufficient and avoids OS path separator
        // issues on Windows.
        //
        // Why NOT inline: true: inlining ALL modules (including src/main/)
        // causes transform overhead that makes the first dynamic import in
        // tests like dev-server-manager.test.ts exceed the 30s testTimeout.
        // Scoping to renderer/src avoids this problem while still fixing the
        // alias issue for all renderer files.
        inline: [/renderer\/src/]
      }
    }
  }
})
