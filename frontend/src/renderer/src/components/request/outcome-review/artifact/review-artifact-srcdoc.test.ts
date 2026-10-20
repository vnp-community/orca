// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { ARTIFACT_CSP, aspectRatioOf, buildArtifactSrcdoc } from './review-artifact-srcdoc'
import { currentTheme, readThemeTokens } from './review-artifact-theme-tokens'

const build = (
  html: string,
  tokens: Record<string, string> = {},
  theme: 'light' | 'dark' = 'light'
) => buildArtifactSrcdoc({ html, tokens, theme })
// Parsed inside a template so that the test itself never loads what an attack names.
const parse = (srcdoc: string) => {
  const t = document.createElement('template')
  t.innerHTML = srcdoc
  return t.content
}

const SVG =
  '<svg viewBox="0 0 800 400" xmlns="http://www.w3.org/2000/svg"><title>Architecture</title><rect width="10" height="10" fill="var(--rv-diff-add, red)"/><text x="1" y="1">ms-gateway</text></svg>'

describe('buildArtifactSrcdoc', () => {
  it('puts the policy first in the head, then the tokens, and keeps the drawing', () => {
    const out = build(SVG, { '--rv-diff-add': 'rgb(1, 2, 3)' })
    expect(
      out.startsWith(
        '<!doctype html><html lang="en"><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy"'
      )
    ).toBe(true)
    expect(out).toContain(`content="${ARTIFACT_CSP}"`)
    expect(out.indexOf('Content-Security-Policy')).toBeLessThan(out.indexOf('<svg'))
    expect(out).toContain('--rv-diff-add:rgb(1, 2, 3)')
    const doc = parse(out)
    expect(doc.querySelector('svg rect')).not.toBeNull()
    expect(doc.textContent).toContain('ms-gateway')
  })

  it('wraps a bare svg and keeps the styles and the body of a full document', () => {
    expect(parse(build(SVG)).querySelector('svg')).not.toBeNull()
    const full =
      '<!doctype html><html><head><style>.a{fill:red}</style></head><body><svg><g class="a"/></svg></body></html>'
    const out = build(full)
    expect(out).toContain('.a{fill:red}')
    expect(parse(out).querySelector('svg g.a')).not.toBeNull()
  })

  it('defines the colour scheme of the theme', () => {
    expect(build(SVG, {}, 'dark')).toContain('color-scheme:dark')
    expect(build(SVG, {}, 'light')).toContain('color-scheme:light')
  })

  it('refuses a token that could break out of its declaration or fetch something', () => {
    const out = build(SVG, {
      '--rv-accent': 'red;}</style><script>x</script>',
      '--rv-bg': 'url(https://evil/x)',
      color: 'red',
      '--rv-ok': 'blue'
    })
    expect(out).not.toContain('evil')
    expect(out).not.toContain('<script')
    expect(out).not.toContain('--rv-accent:')
    expect(out).not.toContain(';color:')
    expect(out).toContain('--rv-ok:blue')
  })

  describe('the hostile set', () => {
    const attacks: Record<string, string> = {
      script: '<svg><script>alert(1)</script><rect/></svg>',
      'svg script with onload':
        '<svg onload="alert(1)"><rect onclick="x()"/><script>1</script></svg>',
      foreignObject:
        '<svg><foreignObject><iframe src="https://evil/"></iframe><img src=x onerror=alert(1)></foreignObject></svg>',
      'meta refresh':
        '<html><head><meta http-equiv="refresh" content="0;url=https://evil/"></head><body><svg/></body></html>',
      base: '<html><head><base href="https://evil/"></head><body><svg><image href="a.png"/></svg></body></html>',
      'self-submitting form':
        '<body><form action="https://evil/" method="post"><input name="a" value="b"></form><script>document.forms[0].submit()</script><svg/></body>',
      'css import':
        '<svg><style>@import url("https://evil/x.css"); .a{background:url(https://evil/p.png)}</style><rect style="fill:url(https://evil/q)"/></svg>',
      'javascript href':
        '<svg><a href="javascript:alert(1)"><text>x</text></a><use xlink:href="https://evil/#a"/></svg>',
      'embedded other document':
        '<svg><image href="https://evil/a.svg"/></svg><object data="https://evil/"></object><embed src="https://evil/">',
      'animated href':
        '<svg><a><animate attributeName="href" values="https://evil/"/><set attributeName="xlink:href" to="https://evil/"/></a></svg>',
      'link stylesheet':
        '<html><head><link rel="stylesheet" href="https://evil/s.css"></head><body><svg/></body></html>',
      'srcdoc and event attributes':
        '<div onmouseover="x()" srcdoc="<script>1</script>"><svg/></div>'
    }
    for (const [name, html] of Object.entries(attacks)) {
      it(`${name} leaves nothing that can run, load or navigate`, () => {
        const out = build(html)
        const doc = parse(out)
        for (const el of Array.from(doc.querySelectorAll('*'))) {
          const tag = el.localName.toLowerCase()
          expect([
            'script',
            'iframe',
            'object',
            'embed',
            'base',
            'link',
            'form',
            'input',
            'foreignobject'
          ]).not.toContain(tag)
          if (tag === 'meta') {
            expect(el.getAttribute('http-equiv') ?? el.getAttribute('charset')).toMatch(
              /^(Content-Security-Policy|utf-8)$/
            )
          }
          for (const attr of Array.from(el.attributes)) {
            expect(attr.name.toLowerCase().startsWith('on')).toBe(false)
            expect(attr.name).not.toBe('srcdoc')
            if (/href|src|action/i.test(attr.name)) {
              expect(attr.value).toMatch(/^(#|data:image\/)/)
            }
          }
        }
        expect(out).not.toMatch(/evil/)
        expect(out).not.toMatch(/javascript:/i)
        expect(out).not.toMatch(/@import/i)
        expect(out.match(/Content-Security-Policy/g)).toHaveLength(1)
      })
    }

    it('keeps what a drawing needs: fragment references, data images, inline styles', () => {
      const html =
        '<svg><defs><linearGradient id="g"/><marker id="m"/></defs><rect fill="url(#g)" marker-end="url(#m)"/><use href="#g"/><image href="data:image/png;base64,AAAA"/><style>.a{fill:url(#g)}</style></svg>'
      const out = build(html)
      expect(out).toContain('url(#g)')
      expect(out).toContain('href="#g"')
      expect(out).toContain('data:image/png;base64,AAAA')
    })

    it('drops a policy of its own so only one applies, and never lets the drawing redefine it', () => {
      const html =
        '<html><head><meta http-equiv="Content-Security-Policy" content="default-src *"></head><body><svg/></body></html>'
      const out = build(html)
      expect(out).not.toContain('default-src *')
      expect(out.match(/Content-Security-Policy/g)).toHaveLength(1)
    })
  })
})

describe('aspectRatioOf', () => {
  it('reads a usable viewBox and refuses the rest', () => {
    expect(aspectRatioOf('0 0 800 400')).toBe(2)
    expect(aspectRatioOf('0,0,100,100')).toBe(1)
    for (const bad of [
      undefined,
      '',
      '0 0 0 100',
      '0 0 100',
      'a b c d',
      '0 0 1000 10',
      '0 0 10 1000',
      '0 0 -5 5'
    ]) {
      expect(aspectRatioOf(bad)).toBeNull()
    }
  })
})

describe('theme tokens', () => {
  it('reads the roles from the tokens in force and skips what is not defined', () => {
    document.documentElement.style.setProperty('--status-success', 'rgb(10, 20, 30)')
    document.documentElement.style.setProperty('--destructive', 'rgb(200, 0, 0)')
    const tokens = readThemeTokens()
    expect(tokens['--rv-diff-add']).toBe('rgb(10, 20, 30)')
    expect(tokens['--rv-diff-del']).toBe('rgb(200, 0, 0)')
    expect(tokens['--rv-diff-mod']).toBeUndefined()
    document.documentElement.style.removeProperty('--status-success')
    document.documentElement.style.removeProperty('--destructive')
  })

  it('knows the theme from the root class', () => {
    document.documentElement.classList.remove('dark')
    expect(currentTheme()).toBe('light')
    document.documentElement.classList.add('dark')
    expect(currentTheme()).toBe('dark')
    document.documentElement.classList.remove('dark')
  })
})
