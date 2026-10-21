// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: async () => ({ ok: false, error: { code: 'x', message: 'x' } })
}))

import { OutcomeSystemDiff } from './OutcomeSystemDiff'
import { makeReview } from '../outcome-review.fixture'
import type { ReviewSection } from '../../../../../../shared/outcome-review-types'

afterEach(cleanup)

const designFlow = {
  section: 'flow',
  variant: 'design',
  type: 'sequence',
  inputDigest: 'd1',
  status: 'cached',
  artifactId: 'a-flow'
}

function show(section: ReviewSection, diagrams: unknown[], phase: 'design' | 'delivered') {
  return render(
    <OutcomeSystemDiff
      requestId="r1"
      review={makeReview({ phase, model: { items: [], diagrams: diagrams as never } })}
      requestStatus="completed"
      section={section}
      onSectionChange={() => undefined}
      canRegenerate={false}
    />
  )
}

describe('OutcomeSystemDiff: a section the agent drew nothing for after the code', () => {
  it('says the agent drew nothing, not "Nothing here", when only the design is drawn', () => {
    show('flow', [designFlow], 'delivered')
    expect(screen.getByTestId('outcome-no-diagram')).toHaveTextContent(
      'The agent drew nothing from the code for this section'
    )
  })

  it('keeps the plain empty text when nothing is drawn at all', () => {
    show('flow', [], 'delivered')
    expect(screen.getByTestId('outcome-no-diagram')).toHaveTextContent('Nothing here.')
  })
})
