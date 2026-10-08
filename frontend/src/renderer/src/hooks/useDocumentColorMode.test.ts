// @vitest-environment happy-dom
import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { useDocumentColorMode } from './useDocumentColorMode'

afterEach(() => document.documentElement.classList.remove('dark'))

describe('useDocumentColorMode', () => {
  it('reads the initial class and follows changes', async () => {
    const { result, unmount } = renderHook(() => useDocumentColorMode())
    expect(result.current).toBe('light')
    act(() => document.documentElement.classList.add('dark'))
    await waitFor(() => expect(result.current).toBe('dark'))
    act(() => document.documentElement.classList.remove('dark'))
    await waitFor(() => expect(result.current).toBe('light'))
    unmount()
  })
})
