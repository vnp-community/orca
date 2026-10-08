import type { ChartFrameProps } from './ChartFrame'

/** Frame props every concrete chart forwards to ChartFrame. */
export type ChartFrameIdentity = Pick<
  ChartFrameProps,
  'id' | 'title' | 'description' | 'staleNote' | 'error' | 'emptyReason' | 'legend'
> & { status?: ChartFrameProps['status']; minHeight?: number }
