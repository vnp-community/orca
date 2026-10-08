/**
 * review-report-export-actions.ts — FE-CV-TASK-090-05
 *
 * Clipboard copy and HTML download. Failures are returned, never swallowed, so the
 * UI can say the copy did not happen.
 *
 * @module components/review-map/report/review-report-export-actions
 */

export type ExportResult = { ok: true } | { ok: false; reason: 'clipboard_unavailable' | 'download_failed' }

type ClipboardBridge = { ui?: { writeClipboardText?: (text: string) => Promise<void> | void } }

function legacyCopy(text: string): boolean {
  try {
    const area = document.createElement('textarea')
    area.value = text
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.opacity = '0'
    document.body.appendChild(area)
    area.select()
    const ok = document.execCommand?.('copy') === true
    area.remove()
    return ok
  } catch {
    return false
  }
}

/** Preload bridge first; the web bridge is silent without navigator.clipboard, so an insecure context also gets the textarea fallback. */
export async function copyTextToClipboard(text: string): Promise<ExportResult> {
  const bridge = (globalThis as { api?: ClipboardBridge }).api
  let bridged = false
  try {
    if (bridge?.ui?.writeClipboardText) {
      await bridge.ui.writeClipboardText(text)
      bridged = true
    } else if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return { ok: true }
    }
  } catch {
    // fall through to the textarea fallback
  }
  if (bridged && !isInsecureContext()) {
    return { ok: true }
  }
  return legacyCopy(text) ? { ok: true } : { ok: false, reason: 'clipboard_unavailable' }
}

function isInsecureContext(): boolean {
  return typeof window !== 'undefined' && window.isSecureContext === false
}

/** Basename only: never a path, never characters that are invalid on Windows. */
export function buildReportFileName(repoSlug: string, headCommit: string): string {
  const slug = repoSlug.toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 48) || 'repo'
  const head = headCommit.replace(/[^a-f0-9]/gi, '').slice(0, 7) || 'head'
  return `orca-review-${slug}-${head}.html`
}

export function downloadHtmlFile(html: string, fileName: string): ExportResult {
  try {
    const url = URL.createObjectURL(new Blob([html], { type: 'text/html;charset=utf-8' }))
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.rel = 'noopener'
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    return { ok: true }
  } catch {
    return { ok: false, reason: 'download_failed' }
  }
}
