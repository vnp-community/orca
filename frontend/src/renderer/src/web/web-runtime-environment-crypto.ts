// FIX BUG-FE-HLD-001: deviceToken (the E2EE pairing bearer credential — see
// web-pairing.ts's own comment: "pairing payloads include the runtime auth
// token") was being written to localStorage as plaintext, readable forever by
// any XSS or by reading the browser's storage files from disk/backup.
//
// Why XOR instead of AES-GCM (crypto.subtle): the original fix design used
// crypto.subtle (AES-GCM), but crypto.subtle.encrypt/decrypt/generateKey are
// all async, and saveStoredWebRuntimeEnvironment/readStoredWebRuntimeEnvironment
// are called from ~15 sites across web-preload-api.ts, main.tsx,
// main-web-bootstrap.tsx, WebConnect.tsx, PairCodeFallback.tsx — including a
// module-scope initializer in web-preload-api.ts
// (`let activeEnvironment = readStoredWebRuntimeEnvironment()`) that cannot
// become `await`-based without a much larger, harder-to-verify refactor of
// that file's module initialization order. A synchronous per-tab XOR key
// achieves the same threat model — a closed tab, or reading localStorage from
// disk/backup, cannot recover the plaintext token, because the key is never
// persisted and only ever lives in this module's memory — with zero call-site
// changes required elsewhere.
let sessionXorKey: Uint8Array | null = null

function getOrCreateSessionXorKey(): Uint8Array {
  if (!sessionXorKey) {
    sessionXorKey = crypto.getRandomValues(new Uint8Array(32))
  }
  return sessionXorKey
}

function xorBytes(data: Uint8Array, key: Uint8Array): Uint8Array {
  const out = new Uint8Array(data.length)
  for (let i = 0; i < data.length; i++) {
    out[i] = (data[i] ?? 0) ^ (key[i % key.length] ?? 0)
  }
  return out
}

function bytesToBase64(bytes: Uint8Array): string {
  return btoa(String.fromCharCode(...bytes))
}

function base64ToBytes(value: string): Uint8Array {
  return Uint8Array.from(atob(value), (c) => c.charCodeAt(0))
}

/** Wrap a plaintext deviceToken for storage. Synchronous — safe to call from any context. */
export function wrapDeviceToken(token: string): string {
  const key = getOrCreateSessionXorKey()
  const bytes = new TextEncoder().encode(token)
  return bytesToBase64(xorBytes(bytes, key))
}

/**
 * Unwrap a previously-wrapped deviceToken. Returns `null` when the session key
 * is gone (new tab / reload) — callers must treat that as "not paired" and
 * prompt re-pairing rather than proceeding with an empty/garbage token.
 */
export function unwrapDeviceToken(wrapped: string): string | null {
  if (!sessionXorKey) {
    return null
  }
  try {
    const bytes = xorBytes(base64ToBytes(wrapped), sessionXorKey)
    return new TextDecoder().decode(bytes)
  } catch {
    return null
  }
}

/** Test-only: reset the in-memory key between test cases. Not for production use. */
export function __resetSessionXorKeyForTests(): void {
  sessionXorKey = null
}
