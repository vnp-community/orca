/**
 * request-link-groups.ts — CR-REQ-019-05
 *
 * Groups a request's links into parent / child / follow-up buckets and
 * resolves which end of each link is "the other" request.
 *
 * @module components/request/request-link-groups
 */

import type { RequestLink } from '../../../../shared/request-types'

export type RequestLinkGroupId = 'parent' | 'child' | 'followUp'

export type RequestLinkGroup = {
  id: RequestLinkGroupId
  links: { link: RequestLink; otherRequestId: string }[]
}

const GROUP_ORDER: RequestLinkGroupId[] = ['parent', 'child', 'followUp']

function groupFor(reason: RequestLink['reason']): RequestLinkGroupId {
  if (reason === 'parent') {return 'parent'}
  if (reason === 'child') {return 'child'}
  return 'followUp'
}

export function groupRequestLinks(links: RequestLink[], currentRequestId: string): RequestLinkGroup[] {
  const buckets = new Map<RequestLinkGroupId, RequestLinkGroup>()
  for (const link of links) {
    const otherRequestId = link.requestId === currentRequestId ? link.relatedRequestId : link.requestId
    if (!otherRequestId || otherRequestId === currentRequestId) {continue}
    const id = groupFor(link.reason)
    const bucket = buckets.get(id) ?? { id, links: [] }
    bucket.links.push({ link, otherRequestId })
    buckets.set(id, bucket)
  }
  return GROUP_ORDER.flatMap((id) => (buckets.has(id) ? [buckets.get(id)!] : []))
}
