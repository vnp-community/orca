import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { SourceControlQualityGateNotice } from './source-control-quality-gate-notice'
import type { QualityGateNoticeViewModel } from './source-control-quality-gate-view-model'

// Echo keys so assertions do not depend on the English catalog wording.
const translate = (key: string, params?: Record<string, unknown>): string =>
  params?.check ? `${key}|${String(params.check)}` : key

function render(viewModel: QualityGateNoticeViewModel, canRunChecks = true): string {
  return renderToStaticMarkup(
    <SourceControlQualityGateNotice
      viewModel={viewModel}
      isLoading={false}
      timedOut={false}
      canRunChecks={canRunChecks}
      onRunChecks={vi.fn()}
      onOpenReason={vi.fn()}
      translate={translate}
    />
  )
}

const failVm: QualityGateNoticeViewModel = {
  visible: true,
  severity: 'fail',
  stale: false,
  unavailable: false,
  reasons: [
    {
      labelKey: 'auto.components.right.sidebar.qualityGateNotice.reason.unknown',
      check: '<b>lint</b>',
      params: {}
    }
  ],
  reasonCount: 4
}

describe('SourceControlQualityGateNotice', () => {
  it('renders nothing for a hidden view model (pass)', () => {
    expect(render({ visible: false })).toBe('')
  })

  it('is a polite status region, not an alert', () => {
    const html = render(failVm)
    expect(html).toContain('role="status"')
    expect(html).toContain('aria-live="polite"')
    expect(html).not.toContain('role="alert"')
  })

  it('shows the title per severity and the extra reason count', () => {
    const html = render(failVm)
    expect(html).toContain('qualityGateNotice.title.fail')
    expect(html).toContain('qualityGateNotice.moreReasons')
  })

  it('renders reason text as plain text (no HTML injection)', () => {
    const html = render(failVm)
    expect(html).not.toContain('<b>lint</b>')
    expect(html).toContain('&lt;b&gt;lint&lt;/b&gt;')
  })

  it('shows unavailable copy for unknown severity', () => {
    const html = render({
      visible: true,
      severity: 'unknown',
      stale: false,
      unavailable: true,
      reasons: [],
      reasonCount: 0
    })
    expect(html).toContain('qualityGateNotice.unavailable')
  })

  it('hides the run button when checks cannot run', () => {
    expect(render(failVm, false)).not.toContain('qualityGateNotice.runChecks')
    expect(render(failVm, true)).toContain('qualityGateNotice.runChecks')
  })

  it('never uses reassuring wording in the English catalog', async () => {
    const en = (await import('../../i18n/locales/en.json')).default as Record<string, unknown>
    const text = JSON.stringify(
      (en as { auto: { components: { right: { sidebar: { qualityGateNotice: unknown } } } } }).auto
        .components.right.sidebar.qualityGateNotice
    ).toLowerCase()
    for (const banned of ['safe', 'approved', 'threshold met', 'all clear']) {
      expect(text).not.toContain(banned)
    }
  })
})
