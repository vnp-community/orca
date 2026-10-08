import { Text, View } from 'react-native'
import type { MobileReviewSummary } from '../session/mobile-review-summary-rpc'
import { mobileReviewSummaryStyles as styles } from './mobile-review-summary-styles'

const METRICS = [
  ['files', 'Files'],
  ['symbols', 'Symbols'],
  ['flows', 'Flows'],
  ['tables', 'Tables'],
  ['contracts', 'Contracts'],
  ['uncovered', 'Untested']
] as const

export function MobileReviewMetricGrid({
  counts
}: {
  counts: NonNullable<MobileReviewSummary['counts']>
}) {
  return (
    <View style={styles.metricGrid}>
      {METRICS.map(([key, label]) => (
        <View
          key={key}
          style={styles.metricCell}
          accessible
          accessibilityLabel={`${label} ${counts[key]}`}
        >
          <Text style={styles.metricValue}>{counts[key]}</Text>
          <Text style={styles.metricLabel}>{label}</Text>
        </View>
      ))}
    </View>
  )
}
