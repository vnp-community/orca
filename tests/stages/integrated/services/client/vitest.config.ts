/**
 * Vitest config — Client integration tests
 *
 * Tests target the web client bundle (fetch/WS from browser perspective).
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 pnpm vitest run --config tests/client/vitest.config.ts
 */
import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    name: 'client',
    include: ['tests/client/**/*.spec.ts'],
    testTimeout: 30_000,
    hookTimeout: 15_000,
    environment: 'node',  // Uses native fetch (Node 22+)
    reporters: ['verbose'],
    globals: false
  }
})
