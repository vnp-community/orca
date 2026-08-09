// OfflineBanner.tsx — Warning bar shown when dev server is unreachable
import { WifiOff } from 'lucide-react'
import { Button } from '../ui/button'

type OfflineBannerProps = {
  message:  string
  onRetry?: () => void
}

export function OfflineBanner({ message, onRetry }: OfflineBannerProps) {
  return (
    <div
      data-testid="offline-banner"
      className="bg-yellow-50 border-b border-yellow-200 px-4 py-2 flex items-center gap-2"
    >
      <WifiOff size={14} className="text-yellow-600 shrink-0" />
      <span className="text-sm text-yellow-800 flex-1">{message}</span>
      {onRetry && (
        <Button
          size="sm"
          variant="outline"
          onClick={onRetry}
          className="shrink-0"
        >
          Retry Connection
        </Button>
      )}
    </div>
  )
}
