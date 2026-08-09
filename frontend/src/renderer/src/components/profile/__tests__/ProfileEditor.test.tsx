// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ProfileEditor } from '../ProfileEditor'
import { useProfile, useProfileActions } from '../../../hooks/useProfile'

vi.mock('../../../hooks/useProfile', () => ({
  useProfile: vi.fn(),
  useProfileActions: vi.fn()
}))

vi.mock('../ProfileFieldRow', () => ({
  ProfileFieldRow: ({ label, children, locked }: any) => (
    <div data-testid="profile-field-row">
      <span>{label}</span>
      {locked && <span>Only Company Admins</span>}
      {children}
    </div>
  )
}))

vi.mock('../../ui/button', () => ({ Button: (p: any) => <button {...p} /> }))
vi.mock('../../ui/tabs', () => ({
  Tabs: (p: any) => <div {...p} />,
  TabsList: (p: any) => <div {...p} />,
  TabsTrigger: (p: any) => <button {...p} />
}))

vi.mock('../../ui/select', () => ({
  Select: (p: any) => <div {...p} />,
  SelectTrigger: (p: any) => <button role="combobox" onClick={p.onClick}>{p.children}</button>,
  SelectValue: () => <span>Value</span>,
  SelectContent: () => <div />,
  SelectItem: () => <div />
}))

vi.mock('../../../store', () => ({
  useAppStore: vi.fn(selector => selector({ resolvedProfile: null }))
}))

describe('ProfileEditor', () => {
  const mockSaveProfile = vi.fn()

  afterEach(cleanup)

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useProfile).mockReturnValue({
      resolvedProfile: { security: { approvedModels: [] } },
      userProfile: { agent: { preferredModel: 'gpt-4' } },
      profileIsLoading: false
    } as any)
    vi.mocked(useProfileActions).mockReturnValue({ saveProfile: mockSaveProfile } as any)
  })

  it('renders My Settings tab for user scope', () => {
    render(<ProfileEditor scope="user" />)
    expect(screen.getByText(/my settings/i)).toBeInTheDocument()
  })

  it('renders Effective Settings tab only when scope=user', () => {
    const { rerender } = render(<ProfileEditor scope="user" />)
    expect(screen.queryByText(/effective settings/i)).toBeInTheDocument()

    rerender(<ProfileEditor scope="company" />)
    expect(screen.queryByText(/effective settings/i)).not.toBeInTheDocument()
  })

  it('security section is readOnly when scope !== company', () => {
    render(<ProfileEditor scope="user" />)
    expect(screen.getByText(/only company admins/i)).toBeInTheDocument()
  })

  it('security section is editable when scope === company', () => {
    render(<ProfileEditor scope="company" />)
    expect(screen.queryByText(/only company admins/i)).not.toBeInTheDocument()
  })

  it('Save Changes button calls saveProfile', () => {
    render(<ProfileEditor scope="user" />)
    const saveBtn = screen.getByTestId('save-profile-btn')
    fireEvent.click(saveBtn)
    expect(mockSaveProfile).toHaveBeenCalledWith('user', { agent: { preferredModel: 'gpt-4' } }, undefined)
  })
})
