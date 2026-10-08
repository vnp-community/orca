import { useLocalSearchParams, useRouter } from 'expo-router'
import { MobileReviewSummaryScreenView } from '../../../../src/components/MobileReviewSummaryScreenView'
import { firstReviewParam } from '../../../../src/session/mobile-diff-review-screen-model'
import { useMobileReviewSummaryController } from '../../../../src/session/use-mobile-review-summary-controller'
import { useForceReconnect, useHostClient } from '../../../../src/transport/client-context'

export default function MobileReviewSummaryScreen() {
  const params = useLocalSearchParams<{
    hostId?: string | string[]
    worktreeId?: string | string[]
    name?: string | string[]
  }>()
  const hostId = firstReviewParam(params.hostId)
  const worktreeId = firstReviewParam(params.worktreeId)
  const name = firstReviewParam(params.name)
  const router = useRouter()
  const { client, state: connState } = useHostClient(hostId)
  const forceReconnect = useForceReconnect()

  const controller = useMobileReviewSummaryController({
    client,
    connState,
    hostId,
    worktreeId,
    name,
    onNavigate: (route) => router.push(route as never),
    onReconnect: forceReconnect
  })

  return <MobileReviewSummaryScreenView controller={controller} onBack={() => router.back()} />
}
