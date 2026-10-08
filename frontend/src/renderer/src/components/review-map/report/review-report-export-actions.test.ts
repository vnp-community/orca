// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buildReportFileName, copyTextToClipboard, downloadHtmlFile } from './review-report-export-actions'

type G = typeof globalThis & { api?: unknown }

beforeEach(() => {
  ;(globalThis as G).api = undefined
})
afterEach(() => {
  vi.restoreAllMocks()
  delete (globalThis as G).api
})

describe('copyTextToClipboard', () => {
  it('uses the preload bridge when available', async () => {
    const write = vi.fn().mockResolvedValue(undefined)
    ;(globalThis as G).api = { ui: { writeClipboardText: write } }
    expect(await copyTextToClipboard('hi')).toEqual({ ok: true })
    expect(write).toHaveBeenCalledWith('hi')
  })

  it('falls back to navigator.clipboard without a bridge', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    expect(await copyTextToClipboard('hi')).toEqual({ ok: true })
    expect(writeText).toHaveBeenCalledWith('hi')
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
  })

  it('reports failure instead of staying silent when every path fails', async () => {
    ;(globalThis as G).api = { ui: { writeClipboardText: vi.fn().mockRejectedValue(new Error('no')) } }
    document.execCommand = vi.fn().mockReturnValue(false)
    expect(await copyTextToClipboard('hi')).toEqual({ ok: false, reason: 'clipboard_unavailable' })
  })

  it('uses the textarea fallback when the bridge throws', async () => {
    ;(globalThis as G).api = { ui: { writeClipboardText: vi.fn().mockRejectedValue(new Error('no')) } }
    document.execCommand = vi.fn().mockReturnValue(true)
    expect(await copyTextToClipboard('hi')).toEqual({ ok: true })
    expect(document.querySelector('textarea')).toBeNull()
  })
})

describe('buildReportFileName', () => {
  it('builds a basename from slug and short head, with no path characters', () => {
    expect(buildReportFileName('My Repo/Name', 'abcdef1234567')).toBe('orca-review-my-repo-name-abcdef1.html')
    expect(buildReportFileName('../../etc', 'zzz')).toBe('orca-review-etc-head.html')
    expect(buildReportFileName('', '')).toBe('orca-review-repo-head.html')
  })
})

describe('downloadHtmlFile', () => {
  it('downloads through a temporary anchor and cleans up', () => {
    const create = vi.fn().mockReturnValue('blob:x')
    const revoke = vi.fn()
    URL.createObjectURL = create
    URL.revokeObjectURL = revoke
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    expect(downloadHtmlFile('<p>x</p>', 'a.html')).toEqual({ ok: true })
    expect(click).toHaveBeenCalledTimes(1)
    expect(revoke).toHaveBeenCalledWith('blob:x')
    expect(document.querySelector('a[download]')).toBeNull()
  })

  it('reports download failure', () => {
    URL.createObjectURL = vi.fn(() => {
      throw new Error('x')
    })
    expect(downloadHtmlFile('x', 'a.html')).toEqual({ ok: false, reason: 'download_failed' })
  })
})
