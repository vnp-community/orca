// Why: shell.* is native/OS-only (dialogs, shell.openPath, shell.openExternal)
// with no remote-runtime-environment equivalent — it always means "this
// Electron process's OS". Unlike runtime-git-client.ts, these wrappers don't
// branch on RuntimeClientTarget; they branch on isWebClientLocation() (the
// same signal the rest of the app already uses to detect the web build) so
// the desktop path goes through the new shell.* RPC surface while the web
// build keeps calling window.api.shell.* — its existing, already-correct
// browser-native fallback (window.open/no-op pickers) — unchanged.
import type { ShellOpenLocalPathResult } from '../../../shared/shell-open-types'
import { isWebClientLocation } from '../lib/web-client-location'
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export async function shellOpenPath(path: string): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.shell.openPath(path)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'shell.openPath', { path })
}

export async function shellOpenInFileManager(path: string): Promise<ShellOpenLocalPathResult> {
  if (isWebClientLocation()) {
    return window.api.shell.openInFileManager(path)
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.openInFileManager', { path })
}

export async function shellOpenInExternalEditor(
  path: string,
  command?: string
): Promise<ShellOpenLocalPathResult> {
  if (isWebClientLocation()) {
    return window.api.shell.openInExternalEditor(path, command)
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.openInExternalEditor', { path, command })
}

export async function shellOpenUrl(url: string): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.shell.openUrl(url)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'shell.openUrl', { url })
}

export async function shellOpenFilePath(path: string): Promise<boolean> {
  if (isWebClientLocation()) {
    return window.api.shell.openFilePath(path)
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.openFilePath', { path })
}

export async function shellOpenFileUri(uri: string): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.shell.openFileUri(uri)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'shell.openFileUri', { uri })
}

export async function shellPathExists(path: string): Promise<boolean> {
  if (isWebClientLocation()) {
    return window.api.shell.pathExists(path)
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.pathExists', { path })
}

export async function shellPickDirectory(args: { defaultPath?: string }): Promise<string | null> {
  if (isWebClientLocation()) {
    return window.api.shell.pickDirectory(args)
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.pickDirectory', args)
}

export async function shellPickAttachment(): Promise<string | null> {
  if (isWebClientLocation()) {
    return window.api.shell.pickAttachment()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.pickAttachment')
}

export async function shellPickImage(): Promise<string | null> {
  if (isWebClientLocation()) {
    return window.api.shell.pickImage()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.pickImage')
}

export async function shellPickRepoIconImage(): Promise<{
  dataUrl: string
  fileName: string
} | null> {
  if (isWebClientLocation()) {
    return window.api.shell.pickRepoIconImage()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.pickRepoIconImage')
}

export async function shellPickAudio(): Promise<string | null> {
  if (isWebClientLocation()) {
    return window.api.shell.pickAudio()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'shell.pickAudio')
}

export async function shellCopyFile(args: { srcPath: string; destPath: string }): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.shell.copyFile(args)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'shell.copyFile', args)
}
