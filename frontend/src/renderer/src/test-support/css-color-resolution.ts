// Pure CSS colour resolution for contrast tests. Why: the repo has no a11y tooling
// (axe), so token contrast is verified by resolving var()/color-mix()/oklch() from main.css.

export type Rgba = { r: number; g: number; b: number; a: number } // r,g,b in 0..255

export type ThemeName = 'light' | 'dark'

function clamp01(v: number): number {
  return Math.min(1, Math.max(0, v))
}

/** Extract `--name: value` declarations of the first block opened by `selector {` at line start. */
export function readDeclarationBlock(css: string, selector: string): Map<string, string> {
  const out = new Map<string, string>()
  const start = css.search(new RegExp(`^${selector.replace(/[.]/g, '\\.')}\\s*\\{`, 'm'))
  if (start < 0) {
    return out
  }
  const open = css.indexOf('{', start)
  let depth = 0
  let end = open
  for (let i = open; i < css.length; i++) {
    if (css[i] === '{') {
      depth++
    } else if (css[i] === '}') {
      depth--
      if (depth === 0) {
        end = i
        break
      }
    }
  }
  const body = css.slice(open + 1, end).replace(/\/\*[\s\S]*?\*\//g, '')
  for (const m of body.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) {
    out.set(m[1], m[2].trim())
  }
  return out
}

export function splitTopLevel(input: string, separator: string): string[] {
  const parts: string[] = []
  let depth = 0
  let current = ''
  for (const ch of input) {
    if (ch === '(') {
      depth++
    } else if (ch === ')') {
      depth--
    }
    if (ch === separator && depth === 0) {
      parts.push(current.trim())
      current = ''
    } else {
      current += ch
    }
  }
  parts.push(current.trim())
  return parts
}

function srgbEncode(linear: number): number {
  const v = clamp01(linear)
  return v <= 0.0031308 ? 12.92 * v : 1.055 * Math.pow(v, 1 / 2.4) - 0.055
}

export function oklchToRgba(lightness: number, chroma: number, hueDeg: number): Rgba {
  const h = (hueDeg * Math.PI) / 180
  const a = chroma * Math.cos(h)
  const b = chroma * Math.sin(h)
  const l_ = Math.pow(lightness + 0.3963377774 * a + 0.2158037573 * b, 3)
  const m_ = Math.pow(lightness - 0.1055613458 * a - 0.0638541728 * b, 3)
  const s_ = Math.pow(lightness - 0.0894841775 * a - 1.291485548 * b, 3)
  const r = 4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_
  const g = -1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_
  const bl = -0.0041960863 * l_ - 0.7034186147 * m_ + 1.707614701 * s_
  return { r: srgbEncode(r) * 255, g: srgbEncode(g) * 255, b: srgbEncode(bl) * 255, a: 1 }
}

function parseHex(value: string): Rgba {
  let hex = value.slice(1)
  if (hex.length === 3 || hex.length === 4) {
    hex = [...hex].map((c) => c + c).join('')
  }
  if (!/^[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/.test(hex)) {
    throw new Error(`Unsupported hex colour: ${value}`)
  }
  return {
    r: Number.parseInt(hex.slice(0, 2), 16),
    g: Number.parseInt(hex.slice(2, 4), 16),
    b: Number.parseInt(hex.slice(4, 6), 16),
    a: hex.length === 8 ? Number.parseInt(hex.slice(6, 8), 16) / 255 : 1
  }
}

function parseNumberOrPercent(token: string, scale: number): number {
  return token.endsWith('%') ? (Number.parseFloat(token) / 100) * scale : Number.parseFloat(token)
}

export type TokenTable = { get(name: string): string | undefined }

export function resolveColorValue(
  value: string,
  table: TokenTable,
  trail: string[] = []
): Rgba {
  const v = value.trim()
  const varMatch = /^var\(\s*(--[\w-]+)\s*(?:,\s*(.+))?\)$/.exec(v)
  if (varMatch) {
    const name = varMatch[1]
    if (trail.includes(name)) {
      throw new Error(`Circular var() reference: ${[...trail, name].join(' -> ')}`)
    }
    const next = table.get(name) ?? varMatch[2]
    if (next === undefined) {
      throw new Error(`Unresolved token ${name}`)
    }
    return resolveColorValue(next, table, [...trail, name])
  }
  if (v === 'transparent') {
    return { r: 0, g: 0, b: 0, a: 0 }
  }
  if (v.startsWith('#')) {
    return parseHex(v)
  }
  const fn = /^([a-z-]+)\((.*)\)$/i.exec(v)
  if (!fn) {
    throw new Error(`Unsupported colour value: ${value}`)
  }
  const name = fn[1].toLowerCase()
  if (name === 'oklch') {
    const [color, alpha] = fn[2].split('/')
    const [l, c, h] = color.trim().split(/\s+/)
    const rgba = oklchToRgba(parseNumberOrPercent(l, 1), parseNumberOrPercent(c, 0.4), Number.parseFloat(h))
    return { ...rgba, a: alpha ? parseNumberOrPercent(alpha.trim(), 1) : 1 }
  }
  if (name === 'rgb' || name === 'rgba') {
    const [color, alpha] = fn[2].split('/')
    const [r, g, b] = color.trim().split(/[\s,]+/).map((t) => parseNumberOrPercent(t, 255))
    return { r, g, b, a: alpha ? parseNumberOrPercent(alpha.trim(), 1) : 1 }
  }
  if (name === 'color-mix') {
    const [space, first, second] = splitTopLevel(fn[2], ',')
    if (space.replace(/\s+/g, ' ') !== 'in srgb' || !second) {
      throw new Error(`Unsupported color-mix: ${value}`)
    }
    const parseStop = (stop: string): { color: string; pct: number | null } => {
      const m = /^(.*?)(?:\s+([\d.]+)%)?$/.exec(stop.trim())!
      return { color: m[1], pct: m[2] === undefined ? null : Number.parseFloat(m[2]) }
    }
    const a = parseStop(first)
    const b = parseStop(second)
    const pa = a.pct ?? (b.pct === null ? 50 : 100 - b.pct)
    const w = pa / 100
    const ca = resolveColorValue(a.color, table, trail)
    const cb = resolveColorValue(b.color, table, trail)
    // Non-premultiplied interpolation: approximate, see task 088-02 risk.
    return {
      r: ca.r * w + cb.r * (1 - w),
      g: ca.g * w + cb.g * (1 - w),
      b: ca.b * w + cb.b * (1 - w),
      a: ca.a * w + cb.a * (1 - w)
    }
  }
  throw new Error(`Unsupported colour function: ${name}`)
}

export function themeTokenTable(css: string, theme: ThemeName): TokenTable {
  const root = readDeclarationBlock(css, ':root')
  const merged = new Map(root)
  if (theme === 'dark') {
    for (const [k, v] of readDeclarationBlock(css, '.dark')) {
      merged.set(k, v)
    }
  }
  return merged
}

export function resolveThemeColor(css: string, token: string, theme: ThemeName): Rgba {
  return resolveColorValue(`var(${token})`, themeTokenTable(css, theme))
}

function channelLuminance(channel255: number): number {
  const c = clamp01(channel255 / 255)
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
}

export function relativeLuminance(c: Rgba): number {
  return 0.2126 * channelLuminance(c.r) + 0.7152 * channelLuminance(c.g) + 0.0722 * channelLuminance(c.b)
}

export function contrastRatio(a: Rgba, b: Rgba): number {
  if (a.a < 1 || b.a < 1) {
    throw new Error('contrastRatio needs opaque colours; composite translucent colours first')
  }
  const la = relativeLuminance(a)
  const lb = relativeLuminance(b)
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05)
}
