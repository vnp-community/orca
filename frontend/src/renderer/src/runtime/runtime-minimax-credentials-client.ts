// Why: MiniMax auth is a session cookie stored on the desktop machine (not a
// PTY login) -- local-only, so it goes straight through
// window.api.minimaxCredentials (real IPC on desktop, an honest
// not-configured/"unsupported" stub on web) -- NOT callRuntimeRpc, which is
// a network call to backend-go and has no minimaxCredentials.* channels
// (same bug class as runtime-cli-client.ts's pre-fix cli.* calls — see
// backend-go solutions/README.md, "Sixteenth").

export type MiniMaxCredentialsStatus = {
  configured: boolean
}

export async function getMiniMaxCredentialsStatus(): Promise<MiniMaxCredentialsStatus> {
  return window.api.minimaxCredentials.getStatus()
}

export async function saveMiniMaxCredentialsCookie(
  cookie: string
): Promise<MiniMaxCredentialsStatus> {
  return window.api.minimaxCredentials.saveCookie(cookie)
}

export async function clearMiniMaxCredentialsCookie(): Promise<MiniMaxCredentialsStatus> {
  return window.api.minimaxCredentials.clearCookie()
}
