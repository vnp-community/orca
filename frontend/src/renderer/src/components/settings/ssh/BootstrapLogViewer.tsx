// BootstrapLogViewer — scrollable monospace log output with auto-scroll (CR-004, TASK-004-D)
import { useEffect, useRef } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'
import { translate } from '@/i18n/i18n'

export function BootstrapLogViewer({ lines }: { lines: string[] }): React.JSX.Element {
  const bottomRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom whenever new lines arrive
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines.length])

  return (
    <div className="mt-2 rounded-md border bg-background font-mono text-xs">
      {/* Header with title + line count */}
      <div className="flex items-center justify-between border-b px-3 py-1.5">
        <span className="text-xs text-muted-foreground">
          {translate('fleet.bootstrap.logTitle', 'Bootstrap output')}
        </span>
        <span className="text-xs tabular-nums text-muted-foreground/50">
          {lines.length} lines
        </span>
      </div>

      {/* Log body */}
      <ScrollArea className="h-[200px]">
        <div className="space-y-0.5 p-3">
          {lines.length === 0 ? (
            <p className="italic text-muted-foreground/50">
              {translate('fleet.bootstrap.logEmpty', 'Waiting for output...')}
            </p>
          ) : (
            lines.map((line, i) => (
              <div
                // Why: index key is acceptable here — lines are append-only and
                // never reordered, so index is stable within a session.
                key={i}
                className="whitespace-pre-wrap break-all leading-5 text-muted-foreground"
              >
                {line}
              </div>
            ))
          )}
          {/* Anchor element for auto-scroll */}
          <div ref={bottomRef} />
        </div>
      </ScrollArea>
    </div>
  )
}
