const { build } = require('vite')
const { resolve } = require('path')

build({
  configFile: false,
  build: {
    target: 'node22',
    ssr: true,
    outDir: 'out/server-test',
    lib: {
      entry: {
        index: resolve(__dirname, 'src/server/index.ts'),
        'daemon-entry': resolve(__dirname, 'src/main/daemon/daemon-entry.ts')
      },
      formats: ['cjs'],
      fileName: (format, entryName) => `${entryName}.js`
    },
    rollupOptions: {
      external: [
        'node-pty', 'better-sqlite3', 'keytar', 'fs', 'path', 'os', 'child_process', 'crypto', 'events', 'net', 'tls', 'http', 'https', 'stream', 'util', 'sqlite3', 'express', 'ws', 'cpu-features', 'ssh2'
      ]
    }
  },
  resolve: {
    alias: {
      'electron': resolve(__dirname, 'src/main/mocks/electron.ts'),
      '@xterm/headless': resolve(__dirname, 'node_modules/@xterm/headless/lib-headless/xterm-headless.js'),
      '@xterm/addon-serialize': resolve(__dirname, 'node_modules/@xterm/addon-serialize/lib/addon-serialize.js'),
      '@xterm/addon-unicode11': resolve(__dirname, 'node_modules/@xterm/addon-unicode11/lib/addon-unicode11.js')
    }
  }
}).then(() => console.log('Done')).catch(console.error)
