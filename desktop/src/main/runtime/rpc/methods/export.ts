import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { exportHtmlToPdf, type ExportHtmlToPdfResult } from '../../../ipc/export'

const ExportHtmlToPdfArgs = z.object({
  html: z.string(),
  title: z.string()
})

export const EXPORT_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'export.htmlToPdf',
    params: ExportHtmlToPdfArgs,
    // Why: no webContents/event to derive a parent window from over RPC —
    // exportHtmlToPdf falls back to an unparented save dialog, same as the
    // ipc handler does when event.sender doesn't resolve to a window.
    handler: (params): Promise<ExportHtmlToPdfResult> => exportHtmlToPdf(params, undefined)
  })
]
