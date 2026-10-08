// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { CreateHostedReviewComposer } from './CreateHostedReviewComposer'
import type { CreateHostedReviewComposerProps } from './CreateHostedReviewComposer'

afterEach(cleanup)

function renderComposer(overrides: Partial<CreateHostedReviewComposerProps> = {}) {
  return render(
    <TooltipProvider>
      <CreateHostedReviewComposer
        provider="gitlab"
        branch="feature"
        base="main"
        setBase={vi.fn()}
        title="t"
        setTitle={vi.fn()}
        body=""
        setBody={vi.fn()}
        draft={false}
        setDraft={vi.fn()}
        baseQuery=""
        setBaseQuery={vi.fn()}
        baseResults={[]}
        setBaseResults={vi.fn()}
        baseSearchError={null}
        aiGenerationEnabled={false}
        generating={false}
        generateDisabled={false}
        generateError={null}
        createError={null}
        isCreating={false}
        primaryAction={{ disabled: false, title: 'Create MR' }}
        onGenerate={vi.fn()}
        onCancelGenerate={vi.fn()}
        onPrimaryAction={vi.fn()}
        {...overrides}
      />
    </TooltipProvider>
  )
}

const label = /insert review report/i

describe('composer "Insert review report" button', () => {
  it('is not rendered without the handler (quality flag off)', () => {
    renderComposer()
    expect(screen.queryByRole('button', { name: label })).toBeNull()
  })

  it('calls the handler on click and locks while it runs', async () => {
    let finish: () => void = () => {}
    const onInsert = vi.fn(() => new Promise<void>((r) => (finish = r)))
    renderComposer({ onInsertReviewReport: onInsert })
    const button = screen.getByRole('button', { name: label }) as HTMLButtonElement
    fireEvent.click(button)
    expect(onInsert).toHaveBeenCalledTimes(1)
    expect(button.disabled).toBe(true)
    fireEvent.click(button)
    expect(onInsert).toHaveBeenCalledTimes(1)
    finish()
    await vi.waitFor(() => expect(button.disabled).toBe(false))
  })

  it('is disabled while AI generation is running', () => {
    renderComposer({ onInsertReviewReport: vi.fn(async () => {}), generating: true })
    expect((screen.getByRole('button', { name: label }) as HTMLButtonElement).disabled).toBe(true)
  })
})
