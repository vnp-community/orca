import type { Approval, OrcaRequest } from '../../../../../shared/request-types'

export function makeApproval(over: Partial<Approval> & { id: string }): Approval {
  return {
    requestId: 'req-1', subjectType: 'phase', rawSubjectType: 'phase', subjectId: 's1', subjectDigest: 'dg',
    status: 'pending', version: 1, createdAt: '2026-10-01T00:00:00Z', updatedAt: '2026-10-01T00:00:00Z', ...over
  }
}

export function makeRequest(over: Partial<OrcaRequest> & { id: string }): OrcaRequest {
  return {
    projectId: 'p1', number: 12, title: 'Login fails', type: 'bug', status: 'planning',
    createdAt: '2026-10-01T00:00:00Z', updatedAt: '2026-10-01T00:00:00Z', ...over
  }
}
