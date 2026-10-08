import { Pressable, Text, View } from 'react-native'
import { AlertCircle, AlertTriangle, Info } from 'lucide-react-native'
import { colors } from '../theme/mobile-theme'
import {
  canOpenMobileReviewFindingDiff,
  mobileReviewOriginLabel,
  mobileReviewSeverityLabel
} from '../session/mobile-review-summary-model'
import type { MobileReviewSummaryFinding } from '../session/mobile-review-summary-rpc'
import { mobileReviewSummaryStyles as styles } from './mobile-review-summary-styles'

type Props = {
  finding: MobileReviewSummaryFinding
  expanded: boolean
  onToggle: () => void
  onOpenDiff: () => void
}

function SeverityIcon({ severity }: { severity: MobileReviewSummaryFinding['severity'] }) {
  if (severity === 'error') {
    return <AlertCircle size={16} color={colors.statusRed} strokeWidth={2.2} />
  }
  if (severity === 'warning') {
    return <AlertTriangle size={16} color={colors.statusAmber} strokeWidth={2.2} />
  }
  return <Info size={16} color={colors.textSecondary} strokeWidth={2.2} />
}

export function MobileReviewFindingRow({ finding, expanded, onToggle, onOpenDiff }: Props) {
  const location = finding.filePath
    ? `${finding.filePath}${finding.startLine ? `:${finding.startLine}` : ''}`
    : null
  return (
    <Pressable
      style={styles.findingRow}
      onPress={onToggle}
      accessibilityRole="button"
      accessibilityState={{ expanded }}
      accessibilityLabel={`${mobileReviewSeverityLabel(finding.severity)}: ${finding.title}`}
    >
      <View style={styles.findingHead}>
        <SeverityIcon severity={finding.severity} />
        <Text style={styles.metaText}>{mobileReviewSeverityLabel(finding.severity)}</Text>
        <Text style={styles.findingTitle} numberOfLines={expanded ? undefined : 2}>
          {finding.title}
        </Text>
      </View>
      {location ? (
        <Text style={styles.findingPath} numberOfLines={expanded ? undefined : 1}>
          {location}
        </Text>
      ) : null}
      <Text style={styles.metaText}>{mobileReviewOriginLabel(finding.origin)}</Text>
      {expanded ? (
        <View>
          {finding.summary ? <Text style={styles.bodyText}>{finding.summary}</Text> : null}
          {canOpenMobileReviewFindingDiff(finding) ? (
            <Pressable
              style={styles.linkButton}
              onPress={onOpenDiff}
              accessibilityRole="button"
              accessibilityLabel="View file diff"
            >
              <Text style={styles.linkText}>View file diff</Text>
            </Pressable>
          ) : null}
        </View>
      ) : null}
    </Pressable>
  )
}
