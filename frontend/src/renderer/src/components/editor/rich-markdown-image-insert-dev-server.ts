// Web/server-mode counterpart of rich-markdown-image-insert.ts's
// insertRichMarkdownImageFromPath. That function's copy path
// (importExternalPathsToRuntime) reads source bytes off the machine running
// window.api — the desktop Electron process, or a paired runtimeEnvironmentId
// host. Neither applies to a DevServerFilePickerDialog-picked path: both the
// picked source and the markdown file's directory live on the same connected
// Dev Server, so the copy has to happen there too — via devServerCopyFile
// (agent fs.copyFile), not a client-side upload.
import type { Editor } from '@tiptap/react'
import { toast } from 'sonner'
import { dirname, basename } from '@/lib/path'
import { translate } from '@/i18n/i18n'
import { devServerCopyFile, devServerPathExists } from '@/runtime/runtime-dev-server-shell-client'
import { extractIpcErrorMessage } from './rich-markdown-ipc-error-message'

export type InsertRichMarkdownImageFromDevServerPathArgs = {
  editor: Editor
  filePath: string
  devServerId: string
  sourcePath: string
  insertPos: number
  canInsert?: (editor: Editor) => boolean
}

const MAX_DECONFLICT_ATTEMPTS = 50

function splitExtension(name: string): { stem: string; ext: string } {
  const dotIndex = name.lastIndexOf('.')
  if (dotIndex <= 0) {
    return { stem: name, ext: '' }
  }
  return { stem: name.slice(0, dotIndex), ext: name.slice(dotIndex) }
}

async function deconflictDestPath(devServerId: string, dir: string, name: string): Promise<string> {
  const { stem, ext } = splitExtension(name)
  let candidate = `${dir}/${name}`
  for (let attempt = 1; attempt <= MAX_DECONFLICT_ATTEMPTS; attempt++) {
    if (!(await devServerPathExists(devServerId, candidate))) {
      return candidate
    }
    candidate = `${dir}/${stem}-${attempt}${ext}`
  }
  return candidate
}

function encodeMarkdownImageBasename(destPath: string): string {
  // Why: unescaped spaces and delimiters in markdown image destinations make
  // screenshot filenames render as literal text or broken partial paths.
  return encodeURIComponent(basename(destPath))
}

export async function insertRichMarkdownImageFromDevServerPath({
  editor,
  filePath,
  devServerId,
  sourcePath,
  insertPos,
  canInsert
}: InsertRichMarkdownImageFromDevServerPathArgs): Promise<void> {
  try {
    const dir = dirname(filePath)
    const destPath = await deconflictDestPath(devServerId, dir, basename(sourcePath))
    await devServerCopyFile(devServerId, { srcPath: sourcePath, destPath })

    if (canInsert && !canInsert(editor)) {
      return
    }

    const imageSrc = encodeMarkdownImageBasename(destPath)
    const inserted = editor
      .chain()
      .focus()
      .insertContentAt(insertPos, { type: 'image', attrs: { src: imageSrc } })
      .run()
    if (!inserted) {
      toast.error(
        translate('auto.components.editor.useLocalImagePick.175cb8b8ce', 'Failed to insert image.')
      )
    }
  } catch (err) {
    toast.error(extractIpcErrorMessage(err, 'Failed to insert image.'))
  }
}
