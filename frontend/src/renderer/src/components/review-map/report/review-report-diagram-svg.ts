/**
 * review-report-diagram-svg.ts — FE-CV-TASK-090-04
 *
 * Renders vetted Mermaid sources to sanitized SVG strings for the HTML export.
 * Same hardening as MermaidBlock: strict security level plus a DOMPurify pass.
 * Any failure yields null for that diagram; the alt text still ships.
 *
 * @module components/review-map/report/review-report-diagram-svg
 */

import DOMPurify from 'dompurify'
import type MermaidApi from 'mermaid'
import { getMermaidConfig } from '../../editor/mermaid-config'
import { guardMermaidSource } from './review-report-diagram-guard'
import type { ReportDiagram } from './review-report-model-parser'

export async function renderReportDiagramSvgs(
  diagrams: readonly ReportDiagram[],
  isDark: boolean
): Promise<(string | null)[]> {
  const out: (string | null)[] = []
  let mermaid: typeof MermaidApi | null = null
  for (const [index, diagram] of diagrams.entries()) {
    const guarded = diagram.truncated ? null : guardMermaidSource(diagram.mermaid)
    if (!guarded?.ok) {
      out.push(null)
      continue
    }
    try {
      // Why: mermaid is large; load it only when a diagram actually needs rendering.
      mermaid ??= (await import('mermaid')).default
      mermaid.initialize(getMermaidConfig(isDark, false))
      const { svg } = await mermaid.render(`report-diagram-${index}`, guarded.src)
      out.push(DOMPurify.sanitize(svg, { USE_PROFILES: { svg: true } }))
    } catch {
      document.getElementById(`dreport-diagram-${index}`)?.remove()
      out.push(null)
    }
  }
  return out
}
