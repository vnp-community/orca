import { clipboard, nativeImage } from 'electron'
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import {
  assertClipboardTextWithinLimitWithYield,
  assertClipboardTextWriteWithinLimitWithYield,
  type ReadClipboardTextOptions
} from '../../../../shared/clipboard-text'
import {
  assertClipboardImageBase64LengthWithinLimit,
  assertClipboardImageByteLengthWithinLimit,
  assertClipboardImageDimensionsWithinLimit
} from '../../../../shared/clipboard-image'
import { saveClipboardImageBufferForTarget } from '../../../window/clipboard-ipc-handlers'
import type { SaveClipboardImageAsTempFileArgs } from '../../../window/clipboard-image-temp-file'

const ReadClipboardTextParams = z
  .object({ maxBytes: z.number().finite().optional() })
  .nullable()
  .optional()

const WriteClipboardTextParams = z.string()

const WriteClipboardImageParams = z.string()

const SaveClipboardImageAsTempFileParams = z
  .object({
    connectionId: z.string().nullable().optional(),
    runtimeEnvironmentId: z.string().nullable().optional()
  })
  .nullable()
  .optional()

// Why: these wrappers call the exact same clipboard primitives and shared
// validators the desktop ipcMain 'clipboard:*' handlers already use — see
// desktop/src/main/window/clipboard-ipc-handlers.ts. That file's sender-trust
// check (assertTrustedClipboardSender) has no RPC equivalent; RPC requests are
// already authenticated at the transport layer (authToken/deviceToken), so it
// is intentionally not reproduced here. clipboard:writeFile is not migrated —
// its resolveAuthorizedPath call needs the full persistence Store, and
// RpcContext only exposes the narrower RuntimeStore surface.
export const UI_ACTIONS_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'ui.readClipboardText',
    params: ReadClipboardTextParams,
    handler: (params): Promise<string> =>
      assertClipboardTextWithinLimitWithYield(
        clipboard.readText(),
        (params ?? undefined) as ReadClipboardTextOptions | undefined
      )
  }),
  defineMethod({
    name: 'ui.writeClipboardText',
    params: WriteClipboardTextParams,
    handler: async (params): Promise<void> => {
      clipboard.writeText(await assertClipboardTextWriteWithinLimitWithYield(params))
    }
  }),
  defineMethod({
    name: 'ui.writeClipboardImage',
    params: WriteClipboardImageParams,
    handler: (params): void => {
      // Why: only accept validated PNG data URIs — mirrors the ipcMain
      // 'clipboard:writeImage' handler's defense-in-depth check.
      const prefix = 'data:image/png;base64,'
      if (!params.startsWith(prefix)) {
        return
      }
      const contentBase64 = params.slice(prefix.length)
      try {
        assertClipboardImageBase64LengthWithinLimit(contentBase64.length)
      } catch {
        return
      }
      const buffer = Buffer.from(contentBase64, 'base64')
      try {
        assertClipboardImageByteLengthWithinLimit(buffer.byteLength)
      } catch {
        return
      }
      const image = nativeImage.createFromBuffer(buffer)
      if (image.isEmpty()) {
        return
      }
      try {
        assertClipboardImageDimensionsWithinLimit(image.getSize())
      } catch {
        return
      }
      clipboard.writeImage(image)
    }
  }),
  defineMethod({
    name: 'ui.saveClipboardImageAsTempFile',
    params: SaveClipboardImageAsTempFileParams,
    handler: (params): Promise<string | null> => {
      const image = clipboard.readImage()
      if (image.isEmpty()) {
        return Promise.resolve(null)
      }
      assertClipboardImageDimensionsWithinLimit(image.getSize())
      return saveClipboardImageBufferForTarget(
        image.toPNG(),
        (params ?? undefined) as SaveClipboardImageAsTempFileArgs | undefined
      )
    }
  })
]
