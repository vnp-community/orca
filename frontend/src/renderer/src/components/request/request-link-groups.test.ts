import { describe, expect, it } from 'vitest'
import { groupRequestLinks } from './request-link-groups'
import type { RequestLink } from '../../../../shared/request-types'

const link = (id: string, requestId: string, relatedRequestId: string, reason: RequestLink['reason']): RequestLink => ({
  id, requestId, relatedRequestId, reason, createdAt: ''
})

describe('groupRequestLinks', () => {
  it('groups by reason in parent, child, follow-up order and resolves the other end', () => {
    const groups = groupRequestLinks(
      [
        link('1', 'me', 'kid', 'child'),
        link('2', 'dad', 'me', 'parent'),
        link('3', 'me', 'next', 'followup_hotfix'),
        link('4', 'me', 'esc', 'escalation')
      ],
      'me'
    )
    expect(groups.map((g) => g.id)).toEqual(['parent', 'child', 'followUp'])
    expect(groups[0].links[0].otherRequestId).toBe('dad')
    expect(groups[1].links[0].otherRequestId).toBe('kid')
    expect(groups[2].links.map((l) => l.otherRequestId)).toEqual(['next', 'esc'])
  })

  it('skips self links and empty ids', () => {
    expect(groupRequestLinks([link('1', 'me', 'me', 'child'), link('2', 'me', '', 'child')], 'me')).toEqual([])
  })
})
