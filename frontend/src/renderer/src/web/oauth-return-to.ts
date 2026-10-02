// Why: return_to arrives via query string, so it is allow-listed to same-origin OAuth paths
// to rule out open redirects and javascript: URLs.
const RETURN_TO_ALLOWED = [
  /^\/oauth\/authorize\?/,
  /^\/oauth\/consent\?request_id=[A-Za-z0-9-]{8,64}$/
]
const STORAGE_KEY = 'orca.oauth.returnTo.v1'
const ATTEMPT_KEY = 'orca.oauth.returnTo.attempts'
// Why: caps the /login <-> /oauth/authorize ping-pong when the cookie never matches.
const MAX_ATTEMPTS = 2
const STASH_TTL_MS = 10 * 60 * 1000
const CONSENT_PATH = '/oauth/consent'

type LocationLike = Pick<Location, 'pathname' | 'search'>

export function sanitizeReturnTo(raw: string | null | undefined): string | null {
  if (!raw || raw.length > 4096 || !raw.startsWith('/') || raw.startsWith('//')) {
    return null
  }
  // eslint-disable-next-line no-control-regex
  if (raw.includes('\\') || /[\u0000-\u001f\u007f]/.test(raw)) {
    return null
  }
  return RETURN_TO_ALLOWED.some((re) => re.test(raw)) ? raw : null
}

export function isOAuthConsentPath(loc: Pick<Location, 'pathname'>): boolean {
  return loc.pathname === CONSENT_PATH
}

/** Login target that brings an expired consent session back to the same request. */
export function loginUrlFor(loc: Partial<LocationLike>): string {
  const here = `${loc.pathname ?? ''}${loc.search ?? ''}`
  return sanitizeReturnTo(here) && isOAuthConsentPath({ pathname: loc.pathname ?? '' })
    ? `/login?return_to=${encodeURIComponent(here)}`
    : '/login'
}

function readQueryTarget(loc: LocationLike): string | null {
  try {
    return sanitizeReturnTo(new URLSearchParams(loc.search).get('return_to'))
  } catch {
    return null
  }
}

function store(): Storage | null {
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

function writeStash(target: string): void {
  const s = store()
  try {
    const prev = s?.getItem(STORAGE_KEY)
    const prevTarget = prev ? (JSON.parse(prev) as { target?: string }).target : undefined
    s?.setItem(STORAGE_KEY, JSON.stringify({ target, at: Date.now() }))
    if (prevTarget !== target) {
      s?.setItem(ATTEMPT_KEY, '0')
    }
  } catch {
    /* storage unavailable: only the post-login return is lost */
  }
}

function readStash(): string | null {
  try {
    const raw = store()?.getItem(STORAGE_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw) as { target?: unknown; at?: unknown }
    if (typeof parsed.at !== 'number' || Date.now() - parsed.at > STASH_TTL_MS) {
      return null
    }
    return typeof parsed.target === 'string' ? sanitizeReturnTo(parsed.target) : null
  } catch {
    return null
  }
}

export function clearReturnTo(): void {
  try {
    store()?.removeItem(STORAGE_KEY)
    store()?.removeItem(ATTEMPT_KEY)
  } catch {
    /* storage unavailable */
  }
}

/** Call while NOT signed in: remember where to go after local login or SSO. */
export function stashReturnToFromLocation(loc: LocationLike): void {
  const target = isOAuthConsentPath(loc)
    ? sanitizeReturnTo(`${loc.pathname}${loc.search}`)
    : readQueryTarget(loc)
  if (target) {
    writeStash(target)
  }
}

/** Call while signed in: returns the path to forward to, or null (also when attempts ran out). */
export function takeForwardTarget(loc: LocationLike): string | null {
  if (isOAuthConsentPath(loc)) {
    clearReturnTo()
    return null
  }
  const target = readQueryTarget(loc) ?? readStash()
  if (!target) {
    return null
  }
  const s = store()
  let attempts = 0
  try {
    attempts = Number(s?.getItem(ATTEMPT_KEY) ?? '0') || 0
    if (attempts >= MAX_ATTEMPTS) {
      clearReturnTo()
      return null
    }
    s?.setItem(ATTEMPT_KEY, String(attempts + 1))
    s?.setItem(STORAGE_KEY, JSON.stringify({ target, at: Date.now() }))
  } catch {
    /* without storage there is no loop guard: forward once per page load */
  }
  return target
}
