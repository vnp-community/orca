import { defineConfig } from 'vitest/config'

export default defineConfig({
  root: import.meta.dirname,
  test: {
    environment: 'node',
    include: [
      'src/relay/codeintel/**/*.test.ts',
      'src/relay/codeintel-*.test.ts',
      'src/relay/gitnexus-*.test.ts',
      'src/relay/codegraph-*.test.ts',
      'src/relay/quality-*.test.ts'
    ]
  }
})
