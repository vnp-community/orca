import { Pressable, Text, View } from 'react-native'
import { ChevronLeft, RefreshCw } from 'lucide-react-native'
import { colors } from '../theme/mobile-theme'
import { mobileReviewSummaryStyles as styles } from './mobile-review-summary-styles'

type Props = {
  worktreeLabel: string
  onBack: () => void
  onRefresh: () => void
}

export function MobileReviewSummaryHeader({ worktreeLabel, onBack, onRefresh }: Props) {
  return (
    <View style={styles.header}>
      <Pressable
        style={({ pressed }) => [styles.iconButton, pressed && styles.iconButtonPressed]}
        onPress={onBack}
        accessibilityRole="button"
        accessibilityLabel="Back"
      >
        <ChevronLeft size={19} color={colors.textPrimary} strokeWidth={2.2} />
      </Pressable>
      <View style={styles.titleBlock}>
        <Text style={styles.title} numberOfLines={1}>
          Review summary
        </Text>
        <Text style={styles.subtitle} numberOfLines={1}>
          {worktreeLabel}
        </Text>
      </View>
      <Pressable
        style={({ pressed }) => [styles.iconButton, pressed && styles.iconButtonPressed]}
        onPress={onRefresh}
        accessibilityRole="button"
        accessibilityLabel="Refresh review summary"
      >
        <RefreshCw size={18} color={colors.textPrimary} strokeWidth={2.2} />
      </Pressable>
    </View>
  )
}
