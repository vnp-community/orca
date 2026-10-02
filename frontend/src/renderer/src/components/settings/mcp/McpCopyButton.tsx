import { useEffect, useRef, useState } from 'react'
import { CheckIcon, CopyIcon } from 'lucide-react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'

type McpCopyButtonProps = {
  text: string
  /** Accessible name, e.g. "Copy server address". */
  ariaLabel?: string
  /** Secrets must not echo into toasts. */
  quiet?: boolean
  onFailed?: () => void
}

export function McpCopyButton({
  text,
  ariaLabel,
  quiet = false,
  onFailed
}: McpCopyButtonProps): React.JSX.Element {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(
    () => () => {
      if (timer.current) {
        clearTimeout(timer.current)
      }
    },
    []
  )

  const copy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      onFailed?.()
      return
    }
    setCopied(true)
    if (!quiet) {
      toast.success(translate('auto.mcp.common.copied', 'Copied'))
    }
    if (timer.current) {
      clearTimeout(timer.current)
    }
    timer.current = setTimeout(() => setCopied(false), 1500)
  }

  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      aria-label={ariaLabel ?? translate('auto.mcp.common.copy', 'Copy')}
      onClick={() => void copy()}
    >
      {copied ? <CheckIcon aria-hidden /> : <CopyIcon aria-hidden />}
      <span aria-live="polite">
        {copied
          ? translate('auto.mcp.common.copied', 'Copied')
          : translate('auto.mcp.common.copy', 'Copy')}
      </span>
    </Button>
  )
}
