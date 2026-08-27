// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'

const { toastError, toastSuccess, writeClipboardText } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  writeClipboardText: vi.fn()
}))

vi.mock('sonner', () => ({ toast: { success: toastSuccess, error: toastError } }))
vi.mock('@/i18n/i18n', () => ({ translate: (_key: string, fallback: string) => fallback }))
vi.mock('@/runtime/runtime-ui-client', () => ({
  uiWriteClipboardText: (text: string) => writeClipboardText(text)
}))

import { copyRichMarkdownLink } from './rich-markdown-link-clipboard'

describe('copyRichMarkdownLink', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('reports success only after the clipboard write resolves', async () => {
    let resolveWrite: (() => void) | undefined
    writeClipboardText.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveWrite = resolve
      })
    )
    const copy = copyRichMarkdownLink('https://example.com')
    expect(toastSuccess).not.toHaveBeenCalled()
    resolveWrite?.()
    await copy
    expect(toastSuccess).toHaveBeenCalledWith('Copied link')
  })

  it('reports a rejected clipboard write without throwing', async () => {
    writeClipboardText.mockRejectedValue(new Error('clipboard unavailable'))
    await expect(copyRichMarkdownLink('https://example.com')).resolves.toBeUndefined()
    expect(toastError).toHaveBeenCalledWith('Failed to copy link')
  })
})
