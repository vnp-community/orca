import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import {
  copyPickedFile,
  openFileUri,
  openInExternalEditor,
  openInFileManager,
  openUrl,
  openWithSystemDefault,
  pathExists,
  pickAttachment,
  pickAudio,
  pickDirectory,
  pickImage,
  pickRepoIconImage
} from '../../../ipc/shell'

const PathParams = z.object({ path: z.string() })
const UrlParams = z.object({ url: z.string() })
const UriParams = z.object({ uri: z.string() })
const OpenInExternalEditorParams = z.object({
  path: z.string(),
  command: z.string().optional()
})
const PickDirectoryParams = z.object({ defaultPath: z.string().optional() })
const CopyFileParams = z.object({ srcPath: z.string(), destPath: z.string() })

// Why: shell.* is native/OS-only (dialogs, shell.openPath, shell.openExternal)
// and previously reachable only via window.api. These wrappers call the exact
// same functions the desktop ipcMain 'shell:*' handlers already call, so both
// transports share one behavior — see desktop/src/main/ipc/shell.ts.
export const SHELL_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'shell.openPath',
    params: PathParams,
    // Why: mirrors the legacy fire-and-forget 'shell:openPath' contract,
    // which reveals the path in the file manager rather than opening it.
    handler: async (params) => {
      void (await openInFileManager(params.path))
    }
  }),
  defineMethod({
    name: 'shell.openInFileManager',
    params: PathParams,
    handler: (params) => openInFileManager(params.path)
  }),
  defineMethod({
    name: 'shell.openInExternalEditor',
    params: OpenInExternalEditorParams,
    handler: (params) => openInExternalEditor(params.path, params.command)
  }),
  defineMethod({
    name: 'shell.openUrl',
    params: UrlParams,
    handler: (params) => openUrl(params.url)
  }),
  defineMethod({
    name: 'shell.openFilePath',
    params: PathParams,
    handler: (params) => openWithSystemDefault(params.path)
  }),
  defineMethod({
    name: 'shell.openFileUri',
    params: UriParams,
    handler: (params) => openFileUri(params.uri)
  }),
  defineMethod({
    name: 'shell.pathExists',
    params: PathParams,
    handler: (params) => pathExists(params.path)
  }),
  defineMethod({
    name: 'shell.pickDirectory',
    params: PickDirectoryParams,
    handler: (params) => pickDirectory(params)
  }),
  defineMethod({
    name: 'shell.pickAttachment',
    params: null,
    handler: () => pickAttachment()
  }),
  defineMethod({
    name: 'shell.pickImage',
    params: null,
    handler: () => pickImage()
  }),
  defineMethod({
    name: 'shell.pickRepoIconImage',
    params: null,
    handler: () => pickRepoIconImage()
  }),
  defineMethod({
    name: 'shell.pickAudio',
    params: null,
    handler: () => pickAudio()
  }),
  defineMethod({
    name: 'shell.copyFile',
    params: CopyFileParams,
    handler: (params) => copyPickedFile(params)
  })
]
