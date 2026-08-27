// devServerId-scoped counterparts of shell.pathExists/shell.copyFile for
// server/web mode (see runtime-shell-client.ts's LOCAL_TARGET comment — those
// wrappers are desktop/OS-only and have no remote-runtime-environment
// equivalent). Callers of these are always paths just returned by
// DevServerFilePickerDialog, so devServerId is already in hand — no new
// "current dev server" resolution is invented here.
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export async function devServerPathExists(devServerId: string, path: string): Promise<boolean> {
  return callRuntimeRpc<boolean>(LOCAL_TARGET, 'devServer.pathExists', { id: devServerId, path })
}

export type DevServerFileContent = {
  content: string
  encoding: 'utf-8' | 'base64'
  isBinary: boolean
}

export async function devServerReadFile(
  devServerId: string,
  path: string
): Promise<DevServerFileContent> {
  return callRuntimeRpc<DevServerFileContent>(LOCAL_TARGET, 'devServer.readFile', {
    id: devServerId,
    path
  })
}

export async function devServerCopyFile(
  devServerId: string,
  args: { srcPath: string; destPath: string }
): Promise<{ path: string }> {
  return callRuntimeRpc<{ path: string }>(LOCAL_TARGET, 'devServer.copyFile', {
    id: devServerId,
    ...args
  })
}
