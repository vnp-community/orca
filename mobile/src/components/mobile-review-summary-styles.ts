import { StyleSheet } from 'react-native'
import { colors, radii, spacing, typography } from '../theme/mobile-theme'

export const mobileReviewSummaryStyles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: colors.bgBase },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 50,
    paddingHorizontal: spacing.lg,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.borderSubtle
  },
  iconButton: {
    width: 44,
    height: 44,
    borderRadius: radii.button,
    alignItems: 'center',
    justifyContent: 'center'
  },
  iconButtonPressed: { backgroundColor: colors.bgRaised },
  titleBlock: { flex: 1, minWidth: 0 },
  title: {
    color: colors.textPrimary,
    fontSize: typography.titleSize,
    fontWeight: '700'
  },
  subtitle: {
    color: colors.textMuted,
    fontSize: typography.metaSize,
    marginTop: 2
  },
  content: { padding: spacing.lg, gap: spacing.md },
  wideRow: { flexDirection: 'row', gap: spacing.lg, alignItems: 'flex-start' },
  column: { flex: 1, minWidth: 0, gap: spacing.md },
  card: {
    backgroundColor: colors.bgPanel,
    borderRadius: radii.card,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderSubtle,
    padding: spacing.md,
    gap: spacing.xs
  },
  metaText: { color: colors.textSecondary, fontSize: typography.metaSize },
  bodyText: { color: colors.textPrimary, fontSize: typography.bodySize },
  sectionTitle: {
    color: colors.textPrimary,
    fontSize: typography.bodySize,
    fontWeight: '700'
  },
  banner: {
    backgroundColor: colors.bgRaised,
    borderRadius: radii.row,
    padding: spacing.sm
  },
  bannerText: { color: colors.statusAmber, fontSize: typography.metaSize },
  metricGrid: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  metricCell: {
    flexGrow: 1,
    flexBasis: '30%',
    backgroundColor: colors.bgRaised,
    borderRadius: radii.row,
    padding: spacing.sm
  },
  metricValue: {
    color: colors.textPrimary,
    fontSize: typography.titleSize,
    fontWeight: '700'
  },
  metricLabel: { color: colors.textSecondary, fontSize: typography.metaSize },
  filterRow: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  filterChip: {
    paddingHorizontal: spacing.md,
    minHeight: 36,
    justifyContent: 'center',
    borderRadius: radii.button,
    backgroundColor: colors.bgRaised
  },
  filterChipActive: { backgroundColor: colors.accentBlue },
  filterChipText: {
    color: colors.textSecondary,
    fontSize: typography.metaSize,
    fontWeight: '600'
  },
  filterChipTextActive: { color: colors.surfaceBright },
  findingRow: {
    backgroundColor: colors.bgPanel,
    borderRadius: radii.row,
    padding: spacing.md,
    gap: spacing.xs
  },
  findingHead: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  findingTitle: {
    flex: 1,
    color: colors.textPrimary,
    fontSize: typography.bodySize
  },
  findingPath: {
    color: colors.textSecondary,
    fontSize: typography.metaSize,
    fontFamily: typography.monoFamily
  },
  linkButton: {
    alignSelf: 'flex-start',
    minHeight: 36,
    justifyContent: 'center'
  },
  linkText: {
    color: colors.accentBlue,
    fontSize: typography.metaSize,
    fontWeight: '600'
  },
  centered: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    padding: spacing.xl,
    gap: spacing.md
  },
  retryButton: {
    paddingHorizontal: spacing.lg,
    minHeight: 44,
    justifyContent: 'center',
    borderRadius: radii.button,
    backgroundColor: colors.bgRaised
  }
})
