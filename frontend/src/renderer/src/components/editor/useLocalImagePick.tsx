import { useCallback, useRef, useState } from 'react'
import type { Editor } from '@tiptap/react'
import { toast } from 'sonner'
import { insertRichMarkdownImageFromPath } from './rich-markdown-image-insert'
import { insertRichMarkdownImageFromDevServerPath } from './rich-markdown-image-insert-dev-server'
import { extractIpcErrorMessage } from './rich-markdown-ipc-error-message'
import { isWebClientLocation } from '../../lib/web-client-location'
import { useActiveDevServer } from '../../store/slices/dev-servers-selectors'
import { DevServerFilePickerDialog } from '../remote-browser/DevServerFilePickerDialog'

import { shellPickImage } from '../../runtime/runtime-shell-client'

const IMAGE_EXTENSIONS = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico']

type PendingInsert = { insertPos: number; targetDom: HTMLElement }

export type LocalImagePick = {
  pickImage: () => Promise<void>
  /** Render this once alongside the editor — it stays hidden until pickImage() opens it. */
  pickerDialog: React.JSX.Element
}

export function useLocalImagePick(
  editor: Editor | null,
  filePath: string,
  worktreeId: string | null,
  runtimeEnvironmentId?: string | null
): LocalImagePick {
  const [pickerOpen, setPickerOpen] = useState(false)
  const activeDevServer = useActiveDevServer()
  const pendingInsertRef = useRef<PendingInsert | null>(null)

  const pickImage = useCallback(async () => {
    if (!editor) {
      return
    }
    // Why: the native file picker steals focus from the editor, which can cause
    // ProseMirror to lose track of its selection. We snapshot the cursor position
    // before the async dialog so we can insert the image exactly where the user
    // intended, not at whatever position focus() falls back to afterward.
    const insertPos = editor.state.selection.from
    const targetDom = editor.view.dom

    // Why: there is no OS-native file dialog in server/web mode — browse the
    // connected Dev Server's filesystem instead (see DevServerFilePickerDialog).
    if (isWebClientLocation()) {
      if (!activeDevServer) {
        toast.error('Connect a Dev Server to browse its files.')
        return
      }
      pendingInsertRef.current = { insertPos, targetDom }
      setPickerOpen(true)
      return
    }

    try {
      const srcPath = await shellPickImage()
      if (!srcPath) {
        return
      }
      await insertRichMarkdownImageFromPath({
        editor,
        filePath,
        sourcePath: srcPath,
        worktreeId,
        runtimeEnvironmentId,
        insertPos,
        canInsert: (candidate) =>
          !candidate.isDestroyed && candidate.view.dom === targetDom && targetDom.isConnected
      })
    } catch (err) {
      toast.error(extractIpcErrorMessage(err, 'Failed to insert image.'))
    }
  }, [editor, filePath, runtimeEnvironmentId, worktreeId, activeDevServer])

  const handlePickerSelect = useCallback(
    (sourcePath: string) => {
      setPickerOpen(false)
      const pending = pendingInsertRef.current
      pendingInsertRef.current = null
      if (!editor || !pending || !activeDevServer) {
        return
      }
      const { insertPos, targetDom } = pending
      void insertRichMarkdownImageFromDevServerPath({
        editor,
        filePath,
        devServerId: activeDevServer.id,
        sourcePath,
        insertPos,
        canInsert: (candidate) =>
          !candidate.isDestroyed && candidate.view.dom === targetDom && targetDom.isConnected
      })
    },
    [editor, filePath, activeDevServer]
  )

  const pickerDialog = (
    <DevServerFilePickerDialog
      open={pickerOpen}
      mode="file"
      extensions={IMAGE_EXTENSIONS}
      title="Choose an image"
      onSelect={handlePickerSelect}
      onClose={() => setPickerOpen(false)}
    />
  )

  return { pickImage, pickerDialog }
}
