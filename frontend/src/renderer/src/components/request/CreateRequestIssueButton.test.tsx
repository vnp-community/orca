// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async () => []),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { CreateRequestIssueButton } from './CreateRequestIssueButton'
import type { JiraIssue } from '../../../../shared/jira-types'
import type { GitHubWorkItem } from '../../../../shared/types'

const jira = { key: 'A-1', title: 'T', url: 'https://j/A-1' } as JiraIssue
const gh = (type: 'issue' | 'pr') => ({ type, number: 1, title: 'T', url: 'https://github.com/o/r/issues/1' }) as GitHubWorkItem
const wrap = (ui: React.ReactElement) => render(<TooltipProvider>{ui}</TooltipProvider>)

beforeEach(() => useAppStore.setState({ requestFlowSupport: 'supported' }))
afterEach(cleanup)

describe('CreateRequestIssueButton', () => {
  it('renders for a Jira issue and opens the dialog without bubbling the click', () => {
    const onRow = vi.fn()
    wrap(<div onClick={onRow}><CreateRequestIssueButton issue={jira} /></div>)
    fireEvent.click(screen.getByRole('button', { name: 'Create request' }))
    expect(screen.getByLabelText('Title')).toHaveValue('T')
    expect(onRow).not.toHaveBeenCalled()
  })

  it('renders for a GitHub issue but not for a PR or an unknown repo', () => {
    const { unmount } = wrap(<CreateRequestIssueButton item={gh('issue')} repoIdentity={{ owner: 'o', repo: 'r' }} />)
    expect(screen.getByRole('button', { name: 'Create request' })).toBeInTheDocument()
    unmount()
    const pr = wrap(<CreateRequestIssueButton item={gh('pr')} repoIdentity={{ owner: 'o', repo: 'r' }} />)
    expect(pr.container).toBeEmptyDOMElement()
    pr.unmount()
    const none = wrap(<CreateRequestIssueButton item={gh('issue')} repoIdentity={null} />)
    expect(none.container).toBeEmptyDOMElement()
  })

  it('is hidden when the runtime does not support the request flow', () => {
    useAppStore.setState({ requestFlowSupport: 'unsupported' })
    const { container } = wrap(<CreateRequestIssueButton issue={jira} />)
    expect(container).toBeEmptyDOMElement()
  })
})
