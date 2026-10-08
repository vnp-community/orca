import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { CreateHostedReviewComposer } from './CreateHostedReviewComposer'

const NOTICE = '<div data-testid="quality-notice">notice</div>'

function render(withNotice: boolean, primaryDisabled: boolean): string {
  return renderToStaticMarkup(
    <TooltipProvider>
      <CreateHostedReviewComposer
        provider="github"
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
        primaryAction={{ disabled: primaryDisabled, title: 'Create PR' }}
        onGenerate={vi.fn()}
        onCancelGenerate={vi.fn()}
        onPrimaryAction={vi.fn()}
        qualityNotice={withNotice ? <div data-testid="quality-notice">notice</div> : undefined}
      />
    </TooltipProvider>
  )
}

describe('CreateHostedReviewComposer qualityNotice slot', () => {
  it('renders the slot content when provided', () => {
    expect(render(true, false)).toContain(NOTICE)
  })

  it('does not change anything else (notice is informational, never blocks)', () => {
    expect(render(true, false).replace(NOTICE, '')).toBe(render(false, false))
    expect(render(true, true).replace(NOTICE, '')).toBe(render(false, true))
  })
})
