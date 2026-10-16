/**
 * activity-day-grouping — FE-REQ-TASK-037-07
 *
 * Groups the feed by the viewer's calendar day, newest first.
 *
 * @module components/request/activity/activity-day-grouping
 */

import type { RequestActivityEntry } from '../../../../../shared/request-execution-types'

export const UNDATED_DAY_KEY = 'undated'

export type ActivityDayGroup = {
  /** `YYYY-MM-DD` in the viewer's time zone, or `undated` for an unparsable timestamp. */
  dayKey: string
  relative: 'today' | 'yesterday' | null
  entries: RequestActivityEntry[]
}

// Why: 'en-CA' formats as YYYY-MM-DD, which sorts and compares as plain text.
export function dayKeyOf(iso: string, timeZone?: string): string {
  const time = Date.parse(iso)
  if (Number.isNaN(time)) {
    return UNDATED_DAY_KEY
  }
  return new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).format(time)
}

// Why: step the calendar date, not 24 hours, so a daylight-saving day cannot skip or repeat a label.
function previousDayKey(dayKey: string): string {
  const [y, m, d] = dayKey.split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, d - 1)).toISOString().slice(0, 10)
}

export function groupActivityByDay(
  entries: readonly RequestActivityEntry[],
  now: number = Date.now(),
  timeZone?: string
): ActivityDayGroup[] {
  const today = dayKeyOf(new Date(now).toISOString(), timeZone)
  const yesterday = previousDayKey(today)
  const sorted = entries
    .map((entry, index) => ({ entry, index, time: Date.parse(entry.occurredAt) }))
    // Why: keep the server order for equal timestamps; undated lines go last.
    .sort(
      (a, b) =>
        (Number.isNaN(b.time) ? -Infinity : b.time) - (Number.isNaN(a.time) ? -Infinity : a.time) ||
        a.index - b.index
    )
  const groups: ActivityDayGroup[] = []
  for (const { entry } of sorted) {
    const dayKey = dayKeyOf(entry.occurredAt, timeZone)
    const last = groups.at(-1)
    if (last && last.dayKey === dayKey) {
      last.entries.push(entry)
      continue
    }
    groups.push({
      dayKey,
      relative: dayKey === today ? 'today' : dayKey === yesterday ? 'yesterday' : null,
      entries: [entry]
    })
  }
  return groups
}
