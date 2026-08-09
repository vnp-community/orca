/**
 * Vite config for building the Orca Web SPA bundle.
 *
 * Input:  src/renderer/web-index.html
 * Output: out/web/
 *
 * Differences from the existing vite.web.config.ts:
 * - Stubs 'electron' with electron-web-stub.ts (safe no-ops)
 * - Sets ORCA_PLATFORM='web' for conditional code paths in renderer
 * - Configures dev server proxy to route WebSocket → local backend (:6768)
 * - Uses relative base for proxy-path compatibility
 *
 * Usage:
 *   pnpm build:frontend:web     → production web bundle
 *   pnpm dev:web-spa            → dev server on :5174 with WS proxy
 *
 * @see vite.web.config.ts — existing Electron web variant (kept for compatibility)
 * @see src/platform/stubs/electron-web-stub.ts — the Electron no-op stub
 */
import { resolve } from 'node:path'
import { defineConfig, type UserConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  root: resolve(__dirname, 'src/renderer'),

  // Why: assets may live under a reverse-proxy path prefix
  base: './',

  plugins: [react(), tailwindcss()],

  resolve: {
    alias: {
      // Stub Electron APIs with no-ops in web mode
      electron: resolve(__dirname, 'src/platform/stubs/electron-web-stub.ts'),
      // Alias @ and @renderer to renderer src (same as other vite configs)
      '@renderer': resolve(__dirname, 'src/renderer/src'),
      '@': resolve(__dirname, 'src/renderer/src')
    }
  },

  define: {
    // Platform identifier for conditional code paths in renderer
    'import.meta.env.ORCA_PLATFORM': JSON.stringify('web'),
    'process.env.ORCA_PLATFORM': JSON.stringify('web'),
    // Feature flags
    ORCA_FEATURE_WALL_ENABLED: 'true'
  },

  build: {
    outDir: resolve(__dirname, 'out/web'),
    emptyOutDir: true,
    rollupOptions: {
      input: {
        // Entry point is web-index.html
        'web-index': resolve(__dirname, 'src/renderer/web-index.html'),
        'admin-index': resolve(__dirname, 'src/renderer/admin-index.html')
      },
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]'
      }
    }
  },

  worker: {
    format: 'es'
  },

  server: {
    // Different from Electron dev port (5173) and web variant port (5173)
    port: 5174,
    host: '0.0.0.0',
    proxy: {
      // Proxy WebSocket IPC to local Orca backend in dev mode
      '/ws': {
        target: 'ws://localhost:6768',
        ws: true,
        changeOrigin: true
      },
      // Proxy REST/API calls if needed
      '/api': {
        target: 'http://localhost:6768',
        changeOrigin: true
      }
    }
  }
}) satisfies UserConfig
