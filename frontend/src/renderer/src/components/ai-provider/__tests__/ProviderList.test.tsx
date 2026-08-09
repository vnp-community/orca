// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const testConnection = vi.fn()
const refresh        = vi.fn()
const mockAccounts = [
  { id: 'acc1', provider: 'anthropic', label: 'Anthropic Main',  devServerId: 'srv1', scope: 'server', status: 'active'  },
  { id: 'acc2', provider: 'openai',    label: 'OpenAI Project',  devServerId: 'srv2', scope: 'project', status: 'invalid' },
]

vi.mock('../../../hooks/useAIProviders', () => ({
  useAIProviders: () => ({ accounts: mockAccounts, isLoading: false, refresh, testConnection, deleteAccount: vi.fn() }),
}))
vi.mock('../../../store', () => ({
  useAppStore: (fn?: any) => fn ? fn({ aiAccounts: mockAccounts, aiUsageByAccount: {} }) : {},
}))
vi.mock('../UsageChart', () => ({
  UsageChart: ({ accountId }: any) => <span data-testid={`usage-${accountId}`}>—</span>,
}))
vi.mock('../HealthStatusBadge', () => ({
  HealthStatusBadge: ({ status }: any) => <span data-testid={`badge-${status}`}>{status}</span>,
}))

import { ProviderList } from '../ProviderList'

afterEach(() => cleanup())

describe('ProviderList', () => {
  it('renders all account rows from store', () => {
    render(<ProviderList />)
    expect(screen.getByTestId('account-row-acc1')).toBeInTheDocument()
    expect(screen.getByTestId('account-row-acc2')).toBeInTheDocument()
  })

  it('shows "Add Account" button', () => {
    render(<ProviderList />)
    expect(screen.getByTestId('add-account-btn')).toBeInTheDocument()
  })

  it('renders scope filter select', () => {
    render(<ProviderList />)
    expect(screen.getByTestId('filter-scope')).toBeInTheDocument()
  })

  it('renders status filter select', () => {
    render(<ProviderList />)
    expect(screen.getByTestId('filter-status')).toBeInTheDocument()
  })

  it('renders test connection buttons for each account', () => {
    render(<ProviderList />)
    expect(screen.getByTestId('test-btn-acc1')).toBeInTheDocument()
    expect(screen.getByTestId('test-btn-acc2')).toBeInTheDocument()
  })
})
