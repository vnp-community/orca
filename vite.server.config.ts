import { resolve } from 'node:path'
import { defineConfig, type UserConfig } from 'vite'

/**
 * Vite config for building the Orca Node.js backend bundle.
 *
 * Input:  src/server/index.ts
 * Output: out/server/
 *
 * Key design decisions:
 * - alias 'electron' → src/platform/stubs/electron-node-wrapper.ts
 *   so that all src/main/ code importing from 'electron' gets the
 *   NodeAdapter-aware stub instead of the real Electron binary.
 * - SSR mode to keep node_modules external (not bundled)
 * - node22 target (matches production server Node version)
 */
export default defineConfig({
  build: {
    target: 'node22',
    ssr: true,
    outDir: 'out/server',
    lib: {
      entry: {
        index: resolve(__dirname, 'src/server/index.ts'),
        'daemon-entry': resolve(__dirname, 'src/main/daemon/daemon-entry.ts')
      },
      formats: ['cjs'],
      fileName: (_format, entryName) => `${entryName}.js`
    },
    rollupOptions: {
      external: [
        // Native modules — cannot bundle (require native bindings)
        'node-pty',
        'better-sqlite3',
        'keytar',
        'ssh2',
        'cpu-features',
        'fsevents',
        // Node.js built-in modules (node: protocol)
        /^node:/,
        // Legacy node built-ins (non-prefixed)
        'fs',
        'path',
        'os',
        'child_process',
        'crypto',
        'events',
        'net',
        'tls',
        'http',
        'https',
        'stream',
        'util',
        'assert',
        'buffer',
        'url',
        'querystring',
        'module',
        // Third-party modules (externalized via SSR mode)
        'sqlite3',
        'express',
        'ws',
        // Optional database adapters — loaded dynamically, never bundled
        // Use regex to cover sub-path imports (e.g., mysql2/promise, pg/lib/*)
        /^pg(\/.*)?$/,
        /^pg-native(\/.*)?$/,
        /^mysql2(\/.*)?$/,
        /^mysql(\/.*)?$/,
        /^tedious(\/.*)?$/,
        /^oracledb(\/.*)?$/,
        /^mariadb(\/.*)?$/
      ],
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: '[name].js'
      }
    }
  },
  resolve: {
    alias: {
      // KEY: Alias 'electron' → NodeAdapter-aware stub (delegates to getPlatform())
      // This replaces the legacy mocks/electron.ts approach.
      'electron': resolve(__dirname, 'src/platform/stubs/electron-node-wrapper.ts'),
      // xterm aliases — ensure correct headless builds are resolved
      '@xterm/headless': resolve(
        __dirname,
        'node_modules/@xterm/headless/lib-headless/xterm-headless.js'
      ),
      '@xterm/addon-serialize': resolve(
        __dirname,
        'node_modules/@xterm/addon-serialize/lib/addon-serialize.js'
      ),
      '@xterm/addon-unicode11': resolve(
        __dirname,
        'node_modules/@xterm/addon-unicode11/lib/addon-unicode11.js'
      )
    }
  },
  define: {
    // Expose platform identifier to code using process.env.ORCA_PLATFORM
    'process.env.ORCA_PLATFORM': JSON.stringify('node')
  }
}) satisfies UserConfig
