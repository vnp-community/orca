import type { BacklogGroupData, BacklogTaskRowData, RequestBacklogRowData } from '../../../../../shared/request-backlog-types'

export const backlogRow = (over: Partial<RequestBacklogRowData> = {}): RequestBacklogRowData => ({
  requestId: 'r1', number: 9, title: 'Crash on save', type: 'bug', sourceProvider: 'github', sourceRef: 'o/r#9',
  sourceUrl: 'https://github.com/o/r/issues/9', returnedFromStage: 'plan', returnedCategory: 'infeasible',
  returnReason: 'Needs a vendor change', returnedBy: 'u-1', returnedAt: '2026-10-06T12:00:00Z', parentRequestIds: [], ...over
})

export const backlogTask = (over: Partial<BacklogTaskRowData> & { taskId: string }): BacklogTaskRowData => ({
  title: 'Write tests', status: 'open', estimatedHours: null, blockedByTaskIds: [], failedAttempts: 0, ...over
})

export const backlogGroup = (over: Partial<BacklogGroupData> = {}): BacklogGroupData => ({
  requestId: 'r1', planTaskId: 'pl1', planTitle: 'Auth plan', gateStatus: 'approved', tasks: [backlogTask({ taskId: 't1' })], ...over
})
