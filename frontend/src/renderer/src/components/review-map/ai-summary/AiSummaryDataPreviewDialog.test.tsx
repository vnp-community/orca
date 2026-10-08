// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AiSummaryDataPreviewDialog } from './AiSummaryDataPreviewDialog'
import { parseAiSummaryResponse } from './ai-summary-wire-parser'
import { manifestWire } from './ai-summary.fixture'

afterEach(cleanup)

const translate = (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)
const manifest = (over: Record<string, unknown> = {}) => parseAiSummaryResponse({ manifest: manifestWire(over) }).manifest

describe('AiSummaryDataPreviewDialog', () => {
  it('renders nothing when closed', () => {
    render(<AiSummaryDataPreviewDialog open={false} manifest={manifest()} onConfirm={vi.fn()} onCancel={vi.fn()} translate={translate} />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('lists files, counts, withheld entries and the provider', () => {
    render(<AiSummaryDataPreviewDialog open manifest={manifest()} onConfirm={vi.fn()} onCancel={vi.fn()} translate={translate} />)
    expect(screen.getByText('src/a.ts')).toBeTruthy()
    expect(screen.getByText(/preview\.counts/).textContent).toContain('"redactions":2')
    expect(screen.getByText(/preview\.withheld/).textContent).toContain('"count":1')
    expect(document.body.textContent).toContain('acme-llm')
  })

  it('confirm and cancel call their handlers', () => {
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    render(<AiSummaryDataPreviewDialog open manifest={manifest()} onConfirm={onConfirm} onCancel={onCancel} translate={translate} />)
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.preview.confirm'))
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.preview.cancel'))
    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('locks both buttons while submitting', () => {
    render(<AiSummaryDataPreviewDialog open submitting manifest={manifest()} onConfirm={vi.fn()} onCancel={vi.fn()} translate={translate} />)
    for (const name of ['confirm', 'cancel']) {
      const button = screen.getByText(`auto.components.reviewMap.aiSummary.preview.${name}`) as HTMLButtonElement
      expect(button.disabled).toBe(true)
    }
  })

  it('moves initial focus to Cancel, the safe default', async () => {
    render(<AiSummaryDataPreviewDialog open manifest={manifest()} onConfirm={vi.fn()} onCancel={vi.fn()} translate={translate} />)
    const cancel = screen.getByText('auto.components.reviewMap.aiSummary.preview.cancel')
    await vi.waitFor(() => expect(document.activeElement).toBe(cancel))
  })

  it('warns when injection is suspected and shows file paths as text', () => {
    render(
      <AiSummaryDataPreviewDialog
        open
        manifest={manifest({ suspectedInjection: true, files: [{ path: '<img src=x onerror=1>.ts', bytes: 1, hunks: 1 }] })}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        translate={translate}
      />
    )
    expect(screen.getByRole('alert').textContent).toContain('injectionWarning')
    expect(document.querySelector('img')).toBeNull()
  })
})
