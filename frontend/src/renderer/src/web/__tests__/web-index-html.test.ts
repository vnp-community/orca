import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'

describe('web-index.html', () => {
  const html = readFileSync('src/renderer/web-index.html', 'utf-8')

  it('has a root div with id="root"', () => {
    expect(html).toContain('id="root"')
  })

  it('references web mode entry (not Electron main.tsx)', () => {
    // Web entry should point to web/main.tsx, not the desktop src/main.tsx
    expect(html).not.toContain('src="/src/main.tsx"')
  })

  it('is valid HTML5 document', () => {
    expect(html.toLowerCase()).toContain('<!doctype html>')
    expect(html).toContain('<html>')
    expect(html).toContain('</html>')
  })
})

describe('vite.web.config.ts', () => {
  const config = readFileSync('vite.web.config.ts', 'utf-8')

  it('uses web-index.html as entry point', () => {
    expect(config).toContain('web-index.html')
  })

  it('outputs to out/web/', () => {
    expect(config).toContain('out/web')
  })
})
