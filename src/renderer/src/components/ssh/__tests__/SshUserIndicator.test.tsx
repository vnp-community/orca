// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { SshUserIndicator } from '../SshUserIndicator'
import { ProvisioningStatus } from '../../../store/slices/ssh'

describe('SshUserIndicator', () => {
  afterEach(() => {
    cleanup()
  })

  it('Renders username khi provisioned (done)', () => {
    render(<SshUserIndicator serverId="s1" linuxUsername="orca-alice" provisioned={true} provisioningStatus={{ phase: 'done', linuxUsername: 'orca-alice' }} />)
    expect(screen.getByText('orca-alice')).toBeInTheDocument()
    expect(screen.getByTitle('Provisioned')).toBeInTheDocument()
  })

  it('Shows progressbar khi provisioning', () => {
    render(<SshUserIndicator serverId="s1" linuxUsername="orca-alice" provisioned={false} provisioningStatus={{ phase: 'provisioning', step: 'creating user', progress: 50 }} />)
    expect(screen.getByText('creating user')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
  })

  it('Error alert khi phase=error', () => {
    render(<SshUserIndicator serverId="s1" linuxUsername="orca-alice" provisioned={false} provisioningStatus={{ phase: 'error', message: 'Failed to create user' }} />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to create user')
  })

  it('No progressbar khi idle', () => {
    render(<SshUserIndicator serverId="s1" linuxUsername="orca-alice" provisioned={false} provisioningStatus={{ phase: 'idle' }} />)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    expect(screen.getByText('orca-alice')).toBeInTheDocument()
  })
})
