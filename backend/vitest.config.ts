import { resolve } from 'node:path'
import { defineConfig } from 'vitest/config'

/**
 * Vitest config for the Orca Node.js backend.
 *
 * Mirrors vite.config.ts's `resolve.alias` block — most of backend/src/main
 * still imports 'electron' directly (aliased to the NodeAdapter-aware stub,
 * same as the production build) and a handful of terminal-related modules
 * import '@xterm/*' packages whose default entry points don't resolve
 * correctly under Vitest's Node environment without the explicit path.
 * Without these aliases, most src/main/**\/*.test.ts files fail to import
 * at all rather than failing their actual assertions.
 */
export default defineConfig({
  root: import.meta.dirname,
  resolve: {
    alias: {
      'electron': resolve(import.meta.dirname, 'src/platform/stubs/electron-node-wrapper.ts'),
      '@xterm/headless': resolve(
        import.meta.dirname,
        'node_modules/@xterm/headless/lib-headless/xterm-headless.js'
      ),
      '@xterm/addon-serialize': resolve(
        import.meta.dirname,
        'node_modules/@xterm/addon-serialize/lib/addon-serialize.js'
      ),
      '@xterm/addon-unicode11': resolve(
        import.meta.dirname,
        'node_modules/@xterm/addon-unicode11/lib/addon-unicode11.js'
      )
    }
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
    // Why: several suites spin up real SQLite/relay/SSH fixtures (DB migrations,
    // dev-server relay, fleet SSH connections) — Vitest's 5s defaults are too
    // tight for the slowest integration cases, matching desktop/config/vitest.config.ts.
    hookTimeout: 60_000,
    testTimeout: 30_000
  }
})
