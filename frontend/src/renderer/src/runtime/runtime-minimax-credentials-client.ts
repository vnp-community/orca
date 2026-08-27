// Why: MiniMax auth is a session cookie stored on the desktop machine (not a
// PTY login) -- additive namespace, local-only, mirrors the desktop
// ipcMain 'minimaxCredentials:*' channels 1:1 through the RPC dispatcher.
import { callRuntimeRpc } from './runtime-rpc-client'

export type MiniMaxCredentialsStatus = {
  configured: boolean
}

export async function getMiniMaxCredentialsStatus(): Promise<MiniMaxCredentialsStatus> {
  return callRuntimeRpc<MiniMaxCredentialsStatus>({ kind: 'local' }, 'minimaxCredentials.getStatus')
}

export async function saveMiniMaxCredentialsCookie(
  cookie: string
): Promise<MiniMaxCredentialsStatus> {
  return callRuntimeRpc<MiniMaxCredentialsStatus>({ kind: 'local' }, 'minimaxCredentials.saveCookie', {
    cookie
  })
}

export async function clearMiniMaxCredentialsCookie(): Promise<MiniMaxCredentialsStatus> {
  return callRuntimeRpc<MiniMaxCredentialsStatus>({ kind: 'local' }, 'minimaxCredentials.clearCookie')
}
