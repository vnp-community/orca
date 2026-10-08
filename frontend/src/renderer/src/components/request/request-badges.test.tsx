// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { RequestStatusBadge } from './RequestStatusBadge'
import { RequestTypeBadge } from './RequestTypeBadge'
import { RequestSourceBadge } from './RequestSourceBadge'
import { ApprovalStatusBadge } from './ApprovalStatusBadge'
import type { RequestStatus, RequestType } from '../../../../shared/request-types'

afterEach(cleanup)

const STATUSES: RequestStatus[] = [
  'submitted', 'classifying', 'awaiting_type_confirmation', 'analyzing', 'awaiting_analysis_approval',
  'awaiting_information', 'planning', 'awaiting_plan_approval', 'executing', 'completed', 'cancelled',
  'request_backlog', 'unknown'
]
const TYPES: RequestType[] = [
  'bug', 'task', 'docs', 'question', 'hotfix', 'security', 'ops_request', 'change_request',
  'refactor', 'spike', 'performance', 'unknown'
]

describe('request badges', () => {
  it.each(STATUSES)('RequestStatusBadge renders icon + text for %s', (status) => {
    const { container } = render(<RequestStatusBadge status={status} />)
    const badge = container.firstElementChild as HTMLElement
    expect(badge.getAttribute('aria-label')).toBeTruthy()
    expect(badge.querySelector('svg')).not.toBeNull()
    expect(badge.textContent?.trim()).toBeTruthy()
  })

  it.each(TYPES)('RequestTypeBadge renders a label for %s', (type) => {
    const { container } = render(<RequestTypeBadge type={type} />)
    expect((container.firstElementChild as HTMLElement).textContent?.trim()).toBeTruthy()
  })

  it('ApprovalStatusBadge renders all statuses', () => {
    for (const status of ['pending', 'approved', 'rejected', 'expired', 'unknown'] as const) {
      const { container, unmount } = render(<ApprovalStatusBadge status={status} />)
      expect(container.textContent?.trim()).toBeTruthy()
      unmount()
    }
  })

  it('RequestSourceBadge links only http(s) urls', () => {
    const { rerender } = render(<RequestSourceBadge provider="jira" ref="ABC-1" url="https://x.example/ABC-1" />)
    expect(screen.getByRole('link')).toHaveAttribute('href', 'https://x.example/ABC-1')
    rerender(<RequestSourceBadge provider="jira" ref="ABC-1" url="javascript:alert(1)" />)
    expect(screen.queryByRole('link')).toBeNull()
    expect(screen.getByText('ABC-1')).toBeInTheDocument()
  })
})

describe('request components stay on design tokens', () => {
  it('has no hex colours or emoji in component sources', () => {
    const dirs = [join(__dirname), join(__dirname, 'solution'), join(__dirname, 'plan')]
    for (const dir of dirs) {
      let files: string[] = []
      try {
        files = readdirSync(dir).filter((f) => f.endsWith('.tsx') && !f.endsWith('.test.tsx'))
      } catch {
        continue
      }
      for (const file of files) {
        const src = readFileSync(join(dir, file), 'utf8')
        expect(src, `${file} hex`).not.toMatch(/#[0-9a-fA-F]{3,8}\b(?![\w-])/)
        expect(src, `${file} emoji`).not.toMatch(/\p{Extended_Pictographic}/u)
      }
    }
  })
})
