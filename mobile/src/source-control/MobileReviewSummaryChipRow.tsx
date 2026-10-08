import { Pressable, Text, View } from 'react-native'
import { AlertTriangle, ChevronRight } from 'lucide-react-native'
import { colors } from '../theme/mobile-theme'
import { hubStyles } from './mobile-source-control-hub-styles'
import type {
  MobileReviewSummaryChip,
  MobileReviewSummaryChipTone
} from '../session/mobile-review-summary-chip'

type Props = {
  chip: MobileReviewSummaryChip
  onPress: () => void
}

function toneColor(tone: MobileReviewSummaryChipTone): string {
  return tone === 'danger'
    ? colors.statusRed
    : tone === 'warning'
      ? colors.statusAmber
      : colors.textSecondary
}

// Branch-card entry to the read-only review summary; tone is always paired with
// text so severity never relies on colour alone.
export function MobileReviewSummaryChipRow({ chip, onPress }: Props) {
  const color = toneColor(chip.tone)
  return (
    <Pressable
      style={({ pressed }) => [hubStyles.chip, pressed && hubStyles.chipPressed]}
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={`${chip.label}: ${chip.detail}`}
    >
      <View style={hubStyles.chipIcon}>
        <AlertTriangle size={15} color={color} strokeWidth={2.1} />
      </View>
      <Text style={hubStyles.commentText}>{chip.label}</Text>
      <Text style={[hubStyles.chipMutedText, { color }]} numberOfLines={1}>
        {chip.detail}
      </Text>
      <ChevronRight size={16} color={colors.textMuted} strokeWidth={2.1} />
    </Pressable>
  )
}
