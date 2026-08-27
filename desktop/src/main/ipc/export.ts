import { BrowserWindow, dialog, ipcMain } from 'electron'
import { writeFile } from 'node:fs/promises'
import { ExportTimeoutError, htmlToPdf } from '../lib/html-to-pdf'

export type ExportHtmlToPdfArgs = {
  html: string
  title: string
}

export type ExportHtmlToPdfResult =
  | { success: true; filePath: string }
  | { success: false; cancelled?: boolean; error?: string }

// Why: extracted as a standalone function, parameterized by an optional parent
// window, so both the ipc handler (which has an event.sender) and the local
// RPC method (which has no webContents to derive a parent from) call the
// exact same export logic.
export async function exportHtmlToPdf(
  args: ExportHtmlToPdfArgs,
  parentWindow: BrowserWindow | undefined
): Promise<ExportHtmlToPdfResult> {
  const { html, title } = args
  if (!html.trim()) {
    return { success: false, error: 'No content to export' }
  }

  try {
    const pdfBuffer = await htmlToPdf(html)

    // Why: sanitize to keep the suggested filename legal on every platform.
    // Windows forbids /\:*?"<>| in filenames; truncate to keep the OS save
    // dialog stable when titles are pathologically long.
    const sanitizedTitle = title.replace(/[/\\:*?"<>|]/g, '_').slice(0, 100) || 'export'
    const defaultFilename = `${sanitizedTitle}.pdf`

    const dialogOptions = {
      defaultPath: defaultFilename,
      filters: [{ name: 'PDF', extensions: ['pdf'] }]
    }
    const { canceled, filePath } = parentWindow
      ? await dialog.showSaveDialog(parentWindow, dialogOptions)
      : await dialog.showSaveDialog(dialogOptions)

    if (canceled || !filePath) {
      return { success: false, cancelled: true }
    }

    await writeFile(filePath, pdfBuffer)
    return { success: true, filePath }
  } catch (error) {
    if (error instanceof ExportTimeoutError) {
      return { success: false, error: 'Export timed out' }
    }
    return {
      success: false,
      error: error instanceof Error ? error.message : 'Failed to export PDF'
    }
  }
}

export function registerExportHandlers(): void {
  ipcMain.removeHandler('export:html-to-pdf')
  ipcMain.handle(
    'export:html-to-pdf',
    (event, args: ExportHtmlToPdfArgs): Promise<ExportHtmlToPdfResult> =>
      exportHtmlToPdf(args, BrowserWindow.fromWebContents(event.sender) ?? undefined)
  )
}
