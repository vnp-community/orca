/**
 * Vitest config — Server integration tests
 *
 * Tests target the Node.js server HTTP/WS API.
 * Run against a live server:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 pnpm vitest run --config tests/server/vitest.config.ts
 */
import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    name: 'server',
    include: ['tests/server/**/*.spec.ts'],
    testTimeout: 30_000,
    hookTimeout: 15_000,
    // No jsdom — pure Node fetch/WS
    environment: 'node',
    reporters: ['verbose'],
    globals: false
  }
})
