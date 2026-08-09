// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { SshProvisioningProgress } from '../SshProvisioningProgress'

describe('SshProvisioningProgress', () => {
  afterEach(() => {
    cleanup()
  })

  it('Progressbar aria-valuenow = progress value', () => {
    render(<SshProvisioningProgress step="Setting up" progress={30} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '30')
  })

  it('Renders step description text', () => {
    render(<SshProvisioningProgress step="Configuring SSH keys" progress={50} />)
    expect(screen.getByText('Configuring SSH keys')).toBeInTheDocument()
  })

  it('100% complete state', () => {
    render(<SshProvisioningProgress step="Done" progress={100} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
  })
})
