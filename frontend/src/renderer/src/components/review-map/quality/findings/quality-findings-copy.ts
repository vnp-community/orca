/**
 * quality-findings-copy.ts — FE-CV-TASK-087-12
 *
 * English table of the check-findings strings (catalog group `findings`): list, row, toolbar and
 * waive popover. Call `qf(key, params)` at render time, never at module scope.
 *
 * @module components/review-map/quality/findings/quality-findings-copy
 */

import { createQualityCopy } from '../quality-copy-factory'

export const QUALITY_FINDINGS_COPY = {
  toolbarSeverity: 'Filter by severity',
  toolbarCategory: 'Category',
  toolbarCategoryCount: 'Category ({{count}})',
  toolbarOnlyInScope: 'Only changed files',
  toolbarShowWaived: 'Show waived ({{count}})',
  toolbarSearchPlaceholder: 'Search message, rule or file',
  categoryLint: 'Lint',
  categoryTypecheck: 'Type check',
  categoryTest: 'Tests',
  categoryCoverage: 'Coverage',
  categoryComplexity: 'Complexity',
  categorySecurity: 'Security',
  categoryDependency: 'Dependencies',
  categoryConvention: 'Conventions',
  categoryArchitecture: 'Architecture',
  categoryAi: 'AI review',
  loading: 'Loading…',
  loadError: 'Could not load check findings.',
  retry: 'Retry',
  loadMore: 'Load more',
  emptyNoRun: 'No checks have run yet. Run checks to see findings here.',
  emptyNone: 'No findings in the scope of the checks that ran.',
  emptyFiltered: 'No findings match the current filters.',
  footerShowing: 'Showing {{shown}} of {{total}}',
  footerOutsideScope: '{{count}} outside the changed scope (not counted in the gate)',
  footerCapped: 'Showing the first 5,000 findings. Narrow the filters to see the rest.',
  footerTruncated: 'The tool output was truncated, so this list may be incomplete.',
  rowTool: '{{tool}} {{version}}',
  rowViewDiff: 'View diff',
  rowOpenFile: 'Open file',
  rowFixHint: 'Fix hint',
  rowWaived: 'Waived by {{by}} until {{date}}',
  rowWaivedNoBy: 'Waived until {{date}}',
  waiveOpen: 'Waive',
  waiveRevoke: 'Remove waiver',
  waiveReasonLabel: 'Reason',
  waiveReasonPlaceholder: 'Why is this finding acceptable?',
  waiveReasonCount: '{{count}}/{{max}}',
  waiveExpiryLabel: 'Waiver expires',
  waiveExpiryDays: '{{days}} days',
  waiveExpiryDate: 'Expiry date',
  waiveCancel: 'Cancel',
  waiveSubmit: 'Waive finding',
  waiveSaving: 'Saving…',
  waiveToastWaived: 'Finding waived',
  waiveToastRevoked: 'Waiver removed',
  waiveErrReasonRequired: 'Enter a reason (up to 1000 characters).',
  waiveErrExpiryRequired: 'Choose when the waiver expires.',
  waiveErrExpiryPast: 'The expiry date must be in the future.',
  waiveErrExpiry: 'A waiver can last at most {{maxDays}} days.',
  waiveErrForbidden: 'You do not have permission to waive findings.',
  waiveErrOffline: 'You are offline. The waiver was not saved.',
  waiveErrConflict: 'This finding changed. Reload the list and try again.',
  waiveErrValidation: 'The waiver was rejected. Check the reason and expiry.',
  waiveErrUnknown: 'Could not save the waiver.'
} as const

export type QualityFindingsCopyKey = keyof typeof QUALITY_FINDINGS_COPY

export const qf = createQualityCopy('findings', QUALITY_FINDINGS_COPY)
