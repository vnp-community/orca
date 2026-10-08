// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { RiskOverrideMenu } from './RiskOverrideMenu'

afterEach(cleanup)

async function openDialog(): Promise<void> {
  const trigger = screen.getByRole('button', { name: 'More gate options' })
  fireEvent.keyDown(trigger, { key: 'Enter' })
  const item = await screen.findByRole('menuitem')
  fireEvent.click(item)
  await screen.findByTestId('risk-override-dialog')
}

describe('RiskOverrideMenu', () => {
  it('renders nothing unless the backend allows overriding', () => {
    const { container } = render(<RiskOverrideMenu gate="solution" allowed={undefined} onOverride={vi.fn()} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('requires a 20+ character reason, then calls onOverride and closes', async () => {
    const onOverride = vi.fn().mockResolvedValue({ ok: true, value: {} })
    render(<RiskOverrideMenu gate="solution" allowed onOverride={onOverride} />)
    await openDialog()
    const submit = screen.getByRole('button', { name: 'Bypass gate' })
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'x'.repeat(19) } })
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'incident 4521: hotfix window' } })
    expect(submit).toBeEnabled()
    expect(submit.className).not.toContain('bg-destructive')
    fireEvent.click(submit)
    await waitFor(() => expect(onOverride).toHaveBeenCalledWith({ gate: 'solution', reason: 'incident 4521: hotfix window' }))
    await waitFor(() => expect(screen.queryByTestId('risk-override-dialog')).toBeNull())
  })

  it('keeps the dialog open and shows a field error on validation failure', async () => {
    const onOverride = vi.fn().mockResolvedValue({ ok: false, error: { kind: 'validation', code: 'x', message: 'm' } })
    render(<RiskOverrideMenu gate="solution" allowed onOverride={onOverride} />)
    await openDialog()
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'x'.repeat(25) } })
    fireEvent.click(screen.getByRole('button', { name: 'Bypass gate' }))
    await waitFor(() => expect(document.querySelector('[role="alert"]')).not.toBeNull())
    expect(screen.getByTestId('risk-override-dialog')).toBeInTheDocument()
  })
})
