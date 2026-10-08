import type { IntensityBucket } from './heat-intensity-scale'

/** Fill follows the heat token; text flips at step 4 so every step keeps >= 4.5:1 (tested in token contrast). */
export function heatCellStyle(bucket: IntensityBucket | null): {
  style: React.CSSProperties
  textClass: string
} {
  if (bucket === null) {
    return { style: {}, textClass: 'text-muted-foreground' }
  }
  return {
    style: { backgroundColor: `var(--quality-heat-${bucket})` },
    textClass: bucket <= 3 ? 'text-foreground' : 'text-background'
  }
}
