// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, afterEach } from 'vitest'
import { ProjectSettings } from '../ProjectSettings'

vi.mock('../../../store', () => ({
  useAppStore: vi.fn(selector => selector({
    projects: [{ id: 'p1', name: 'Test Project 1' }]
  }))
}))

vi.mock('../MemberManager', () => ({
  MemberManager: () => <div data-testid="member-manager">MemberManager Content</div>
}))

vi.mock('../../ui/dialog', () => ({
  Dialog: (p: any) => <div data-testid="dialog" onClick={p.onOpenChange}>{p.children}</div>,
  DialogContent: (p: any) => <div data-testid="project-settings-dialog">{p.children}</div>,
  DialogHeader: (p: any) => <div>{p.children}</div>,
  DialogTitle: (p: any) => <h2>{p.children}</h2>
}))

vi.mock('../../ui/tabs', () => ({
  Tabs: (p: any) => <div data-testid="tabs">{p.children}</div>,
  TabsList: (p: any) => <div>{p.children}</div>,
  TabsTrigger: (p: any) => <button data-testid={p['data-testid']}>{p.children}</button>,
  TabsContent: (p: any) => <div>{p.value === 'members' ? p.children : <div data-testid="general-content">General</div>}</div>
}))

describe('ProjectSettings', () => {
  afterEach(cleanup)

  it('renders dialog with "General" and "Members" tabs', () => {
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    expect(screen.getByTestId('tab-general')).toBeInTheDocument()
    expect(screen.getByTestId('tab-members')).toBeInTheDocument()
  })

  it('project name appears in dialog title', () => {
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    expect(screen.getByText('Project Settings — Test Project 1')).toBeInTheDocument()
  })

  it('closes when onClose called', () => {
    const onClose = vi.fn()
    render(<ProjectSettings projectId="p1" open={true} onClose={onClose} />)
    fireEvent.click(screen.getByTestId('dialog'))
    expect(onClose).toHaveBeenCalled()
  })

  it('Members tab renders MemberManager', () => {
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    expect(screen.getByTestId('member-manager')).toBeInTheDocument()
  })
})
