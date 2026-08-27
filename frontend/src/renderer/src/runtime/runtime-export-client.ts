// Why: HTML-to-PDF export drives a native save dialog on the desktop process
// — always local, no remote-environment routing.
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export type RuntimeExportHtmlToPdfResult =
  | { success: true; filePath: string }
  | { success: false; cancelled?: boolean; error?: string }

export function exportRuntimeHtmlToPdf(args: {
  html: string
  title: string
}): Promise<RuntimeExportHtmlToPdfResult> {
  return callRuntimeRpc<RuntimeExportHtmlToPdfResult>(LOCAL_TARGET, 'export.htmlToPdf', args)
}
