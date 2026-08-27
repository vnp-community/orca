import type { LinearIssue } from '../../../shared/types'

export type LinearProjectTab = 'overview' | 'issues'

export type LinearGroupSection = {
  key: string
  label: string
  issues: LinearIssue[]
}

export type LinearIssueListRow =
  | { type: 'section'; key: string; label: string; count: number }
  | { type: 'issue'; issue: LinearIssue }
