// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mermaid = vi.hoisted(() => ({
  initialize: vi.fn(),
  render: vi.fn()
}))
const purify = vi.hoisted(() => ({ sanitize: vi.fn((svg: string) => svg.replace(/<script[\s\S]*?<\/script>/g, '')) }))
vi.mock('mermaid', () => ({ default: mermaid }))
// Why: happy-dom's DOM is not a faithful DOMPurify host; assert the call contract instead.
vi.mock('dompurify', () => ({ default: purify }))

import { renderReportDiagramSvgs } from './review-report-diagram-svg'

beforeEach(() => {
  mermaid.initialize.mockReset()
  mermaid.render.mockReset()
})

const diagram = (over: Record<string, unknown> = {}) => ({
  kind: 'components' as const,
  mermaid: 'flowchart TD\n A --> B',
  alt: [],
  truncated: false,
  ...over
})

describe('renderReportDiagramSvgs', () => {
  it('renders vetted sources and sanitizes the SVG', async () => {
    mermaid.render.mockResolvedValue({ svg: '<svg><script>alert(1)</script><g/></svg>' })
    const [svg] = await renderReportDiagramSvgs([diagram()], false)
    expect(svg).toContain('<svg')
    expect(svg).not.toContain('<script')
    expect(purify.sanitize).toHaveBeenCalledWith(expect.any(String), { USE_PROFILES: { svg: true } })
    expect(mermaid.initialize).toHaveBeenCalledWith(expect.objectContaining({ securityLevel: 'strict' }))
  })

  it('never renders unsafe, truncated or failing diagrams', async () => {
    mermaid.render.mockRejectedValueOnce(new Error('bad syntax'))
    const out = await renderReportDiagramSvgs(
      [diagram({ mermaid: '%%{init: {}}%%\ngraph TD' }), diagram({ truncated: true }), diagram()],
      true
    )
    expect(out).toEqual([null, null, null])
    expect(mermaid.render).toHaveBeenCalledTimes(1)
  })
})
