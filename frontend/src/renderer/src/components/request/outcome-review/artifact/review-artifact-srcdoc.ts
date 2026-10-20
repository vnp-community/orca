/**
 * Review artifact srcdoc — FE-OR-T210
 *
 * Builds the document an artifact is shown in. A drawing is HTML or SVG written by an agent and is never trusted: the frame it
 * goes in has an empty `sandbox` (no scripts, no same-origin), the document carries a Content-Security-Policy that allows nothing
 * but inline styles and data images, and before that this module removes everything that could run, load or navigate. The server
 * checks the drawing too; this is the second lock, for a drawing that got past the first or a server that is not ours.
 *
 * @module components/request/outcome-review/artifact/review-artifact-srcdoc
 */

/** Allows inline styles and data images and nothing else; `base-uri` and `form-action` are not covered by `default-src`. */
export const ARTIFACT_CSP =
  "default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'"

/** The most that may be rendered at once; a drawing is capped at 256 KiB by the server, so this is only a guard. */
export const MAX_SRCDOC_BYTES = 512 * 1024

const REMOVED_ELEMENTS = new Set([
  'script',
  'iframe',
  'frame',
  'frameset',
  'object',
  'embed',
  'applet',
  'link',
  'base',
  'form',
  'input',
  'button',
  'textarea',
  'select',
  'option',
  'audio',
  'video',
  'source',
  'track',
  'portal',
  'template',
  'noscript',
  'foreignobject',
  'meta',
  'title'
])
const URL_ATTRIBUTES = new Set([
  'href',
  'xlink:href',
  'src',
  'data',
  'action',
  'poster',
  'background',
  'formaction',
  'ping',
  'srcset'
])
const SAFE_URL = /^(#|data:image\/(png|jpeg|gif|webp|svg\+xml)[;,])/i

function safeStyleText(css: string): string {
  // Only fragment references (gradients, markers, filters) and data images may be named by url(); an import fetches a file.
  return css
    .replace(/@import[^;]*;?/gi, '')
    .replace(/url\(\s*(['"]?)(.*?)\1\s*\)/gi, (whole, _q: string, target: string) =>
      SAFE_URL.test(target.trim()) ? whole : 'none'
    )
    .replace(/expression\s*\(/gi, '(')
}

function cleanElement(el: Element): void {
  for (const attr of Array.from(el.attributes)) {
    const name = attr.name.toLowerCase()
    if (name.startsWith('on') || name === 'srcdoc') {
      el.removeAttribute(attr.name)
    } else if (URL_ATTRIBUTES.has(name) && !SAFE_URL.test(attr.value.trim())) {
      el.removeAttribute(attr.name)
    } else if (name === 'style') {
      el.setAttribute(attr.name, safeStyleText(attr.value))
    }
  }
  const tag = el.localName.toLowerCase()
  if (tag === 'style') {
    el.textContent = safeStyleText(el.textContent ?? '')
  }
  // An animation can set an href to something else after the check above.
  if (
    (tag === 'animate' || tag === 'set') &&
    /href|src|action/i.test(el.getAttribute('attributeName') ?? '')
  ) {
    el.remove()
  }
}

function sanitize(root: ParentNode): void {
  for (const el of Array.from(root.querySelectorAll('*'))) {
    if (REMOVED_ELEMENTS.has(el.localName.toLowerCase())) {
      el.remove()
    } else {
      cleanElement(el)
    }
  }
}

/** A value is dropped when it could end the declaration or the style block it is written into. */
function safeTokenValue(value: string): string {
  const v = value.trim()
  return v === '' || /[;{}<>\\]|url\(|expression|@import/i.test(v) ? '' : v
}

export type SrcdocInput = {
  /** The drawing as stored: a full HTML document or a bare SVG. */
  html: string
  /** CSS custom properties to define on the root, name → value (see review-artifact-theme-tokens). */
  tokens: Record<string, string>
  theme: 'light' | 'dark'
}

export function buildArtifactSrcdoc({ html, tokens, theme }: SrcdocInput): string {
  // Why a template: its content is parsed into an inert fragment, so nothing in the drawing loads, runs or navigates while it is
  // being cleaned. The head and body tags of a full document are dropped by the parser and their children are kept in order.
  const template = document.createElement('template')
  template.innerHTML = html.slice(0, MAX_SRCDOC_BYTES)
  sanitize(template.content)
  const vars = Object.entries(tokens)
    .map(([name, value]) => [name, safeTokenValue(value)] as const)
    .filter(([name, value]) => /^--[a-z0-9-]+$/.test(name) && value !== '')
    .map(([name, value]) => `${name}:${value}`)
    .join(';')
  return [
    '<!doctype html><html lang="en"><head><meta charset="utf-8">',
    `<meta http-equiv="Content-Security-Policy" content="${ARTIFACT_CSP}">`,
    `<style id="orca-tokens">:root{color-scheme:${theme};${vars}}body{margin:0}</style>`,
    '</head><body>',
    template.innerHTML,
    '</body></html>'
  ].join('')
}

/** `width / height` of a `viewBox="minX minY width height"`, or null when it cannot be used to size a frame. */
export function aspectRatioOf(viewBox: string | undefined): number | null {
  const parts = (viewBox ?? '')
    .trim()
    .split(/[\s,]+/)
    .map(Number)
  if (parts.length !== 4 || parts.some((n) => !Number.isFinite(n))) {
    return null
  }
  const [, , w, h] = parts
  return w > 0 && h > 0 && w / h > 0.1 && w / h < 10 ? w / h : null
}
