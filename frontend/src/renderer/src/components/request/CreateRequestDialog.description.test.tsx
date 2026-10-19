// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async () => []),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))

import { CreateRequestDialog } from './CreateRequestDialog'

afterEach(cleanup)

describe('CreateRequestDialog', () => {
  // Why: the live e2e run logged Radix's "Missing Description" warning on every open of this dialog.
  it('opens without the Radix missing-description warning', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    render(<CreateRequestDialog open onOpenChange={() => {}} />)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(warn.mock.calls.flat().join(' ')).not.toMatch(/Missing `Description`|aria-describedby/)
    warn.mockRestore()
  })
})
