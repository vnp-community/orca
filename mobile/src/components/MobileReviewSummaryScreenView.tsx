import { useMemo } from 'react'
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, Text, View } from 'react-native'
import { SafeAreaView } from 'react-native-safe-area-context'
import { useResponsiveLayout } from '../layout/responsive-layout'
import { colors } from '../theme/mobile-theme'
import {
  filterMobileReviewFindings,
  formatMobileReviewAge,
  MOBILE_REVIEW_SUMMARY_FILTERS,
  mobileReviewRiskLabel,
  mobileReviewSummaryFilterLabel,
  mobileReviewTruncationLabel,
  sortMobileReviewFindings
} from '../session/mobile-review-summary-model'
import type { MobileReviewSummary } from '../session/mobile-review-summary-rpc'
import type { useMobileReviewSummaryController } from '../session/use-mobile-review-summary-controller'
import { MobileReviewFindingRow } from './MobileReviewFindingRow'
import { MobileReviewMetricGrid } from './MobileReviewMetricGrid'
import { MobileReviewSummaryHeader } from './MobileReviewSummaryHeader'
import { mobileReviewSummaryStyles as styles } from './mobile-review-summary-styles'

type Controller = ReturnType<typeof useMobileReviewSummaryController>

type Props = {
  controller: Controller
  onBack: () => void
}

export function MobileReviewSummaryScreenView({ controller, onBack }: Props) {
  const { screenState } = controller
  return (
    <SafeAreaView style={styles.safeArea} edges={['top']}>
      <MobileReviewSummaryHeader
        worktreeLabel={controller.worktreeLabel}
        onBack={onBack}
        onRefresh={() => void controller.refresh()}
      />
      {screenState.kind === 'loading' ? (
        <View style={styles.centered}>
          <ActivityIndicator color={colors.textSecondary} />
        </View>
      ) : screenState.kind === 'ready' ? (
        <SummaryBody controller={controller} summary={screenState.summary} />
      ) : (
        <View style={styles.centered}>
          <Text style={styles.bodyText}>{screenState.message}</Text>
          {screenState.kind === 'error' ? (
            <Pressable
              style={styles.retryButton}
              onPress={() => void controller.refresh()}
              accessibilityRole="button"
              accessibilityLabel="Retry"
            >
              <Text style={styles.bodyText}>Retry</Text>
            </Pressable>
          ) : null}
        </View>
      )}
    </SafeAreaView>
  )
}

function SummaryBody({
  controller,
  summary
}: {
  controller: Controller
  summary: MobileReviewSummary
}) {
  const { isWideLayout } = useResponsiveLayout()
  const items = summary.findings?.items
  const visible = useMemo(
    () => sortMobileReviewFindings(filterMobileReviewFindings(items ?? [], controller.filter)),
    [items, controller.filter]
  )
  const truncation = mobileReviewTruncationLabel(summary)
  const overview = (
    <View style={styles.column}>
      <IndexCard summary={summary} />
      {summary.counts ? <MobileReviewMetricGrid counts={summary.counts} /> : null}
    </View>
  )
  const findings = (
    <View style={styles.column}>
      <Text style={styles.sectionTitle}>Findings ({summary.findings?.totalOpen ?? 0})</Text>
      <View style={styles.filterRow}>
        {MOBILE_REVIEW_SUMMARY_FILTERS.map((filter) => {
          const active = controller.filter === filter
          return (
            <Pressable
              key={filter}
              style={[styles.filterChip, active && styles.filterChipActive]}
              onPress={() => controller.setFilter(filter)}
              accessibilityRole="button"
              accessibilityState={{ selected: active }}
            >
              <Text style={[styles.filterChipText, active && styles.filterChipTextActive]}>
                {mobileReviewSummaryFilterLabel(filter)}
              </Text>
            </Pressable>
          )
        })}
      </View>
      {truncation ? <Text style={styles.metaText}>{truncation}</Text> : null}
      {visible.length === 0 ? (
        <Text style={styles.metaText}>No findings match this filter.</Text>
      ) : (
        visible.map((finding) => (
          <MobileReviewFindingRow
            key={finding.key}
            finding={finding}
            expanded={controller.expandedKey === finding.key}
            onToggle={() => controller.toggleExpanded(finding.key)}
            onOpenDiff={() => controller.openFileDiff(finding)}
          />
        ))
      )}
    </View>
  )
  return (
    <ScrollView
      contentContainerStyle={styles.content}
      refreshControl={
        <RefreshControl
          refreshing={controller.refreshing}
          onRefresh={() => void controller.refresh()}
          tintColor={colors.textSecondary}
        />
      }
    >
      {isWideLayout ? (
        <View style={styles.wideRow}>
          {overview}
          {findings}
        </View>
      ) : (
        <>
          {overview}
          {findings}
        </>
      )}
    </ScrollView>
  )
}

function IndexCard({ summary }: { summary: MobileReviewSummary }) {
  const age = formatMobileReviewAge(summary.index?.indexedAt, Date.now())
  const commit = summary.index?.indexedCommit?.slice(0, 7)
  const indexLine = [commit ? `Index ${commit}` : null, age].filter(Boolean).join(' · ')
  return (
    <View style={styles.card}>
      {indexLine ? <Text style={styles.metaText}>{indexLine}</Text> : null}
      {summary.stale || summary.index?.state === 'stale' ? (
        <View style={styles.banner}>
          <Text style={styles.bannerText}>The index is behind the latest commit.</Text>
        </View>
      ) : null}
      {summary.risk ? (
        <>
          <Text style={styles.bodyText}>Risk: {mobileReviewRiskLabel(summary.risk.level)}</Text>
          {summary.risk.reasons.map((reason) => (
            <Text key={reason} style={styles.metaText}>
              {reason}
            </Text>
          ))}
        </>
      ) : null}
    </View>
  )
}
