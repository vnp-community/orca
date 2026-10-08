/**
 * data-flow-export.ts — FE-CV-TASK-056-04
 *
 * Clipboard and file export of a rendered flow. SVG is read from the DOM that MermaidBlock
 * already sanitized (DOMPurify); nothing is re-rendered here.
 */

export function dataFlowExportFilename(label: string, ext: 'mmd' | 'svg', now: Date): string {
  const slug =
    label
      .toLowerCase()
      .normalize('NFKD')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 40)
      .replace(/-+$/g, '') || 'flow'
  const p = (n: number): string => String(n).padStart(2, '0')
  const stamp = `${now.getFullYear()}${p(now.getMonth() + 1)}${p(now.getDate())}-${p(now.getHours())}${p(now.getMinutes())}`
  return `dataflow-${slug}-${stamp}.${ext}`
}

export async function copyMermaidSource(source: string): Promise<boolean> {
  const api = (window as unknown as { api?: { ui?: { writeClipboardText?: (t: string) => Promise<unknown> | unknown } } }).api
  try {
    if (api?.ui?.writeClipboardText) {
      await api.ui.writeClipboardText(source)
      return true
    }
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(source)
      return true
    }
  } catch {
    // fall through
  }
  return false
}

function downloadBlob(content: string, mime: string, filename: string): void {
  const url = URL.createObjectURL(new Blob([content], { type: mime }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  // Why deferred: revoking synchronously can cancel the download in some Electron versions.
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export function downloadMermaidSource(source: string, filename: string): void {
  downloadBlob(source, 'text/plain;charset=utf-8', filename)
}

/** Returns false when there is no rendered svg or it contains script content. */
export function downloadSvgFromContainer(container: ParentNode | null, filename: string): boolean {
  const svg = container?.querySelector('svg')
  if (!svg) {
    return false
  }
  const markup = svg.outerHTML
  // Defense in depth: never write an svg with script out to disk.
  if (/<script[\s>]/i.test(markup) || /\son[a-z]+\s*=/i.test(markup)) {
    return false
  }
  const withNs = markup.includes('xmlns=') ? markup : markup.replace('<svg', '<svg xmlns="http://www.w3.org/2000/svg"')
  downloadBlob(withNs, 'image/svg+xml;charset=utf-8', filename)
  return true
}
