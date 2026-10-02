import { useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { visualizeControlChars } from '@/components/mcp/mcp-approval-display'

export const UNTRUSTED_TEXT_LIMIT = 500

/**
 * Text from an external server. Why: tool names/descriptions are attacker-controlled, so they are
 * rendered as plain text nodes only (no HTML, markdown or links) with invisible/bidi chars made visible.
 */
export function UntrustedToolText({
  text,
  className
}: {
  text: string
  className?: string
}): React.JSX.Element {
  const [expanded, setExpanded] = useState(false)
  const shown = visualizeControlChars(text)
  const long = shown.length > UNTRUSTED_TEXT_LIMIT
  const body = long && !expanded ? `${shown.slice(0, UNTRUSTED_TEXT_LIMIT)}…` : shown
  return (
    <div className={className}>
      <pre className="font-sans text-sm break-words whitespace-pre-wrap">{body}</pre>
      {long ? (
        <Button type="button" variant="ghost" size="xs" onClick={() => setExpanded((v) => !v)}>
          {expanded
            ? translate('auto.mcp.external.showLess', 'Show less')
            : translate('auto.mcp.external.showMore', 'Show more')}
        </Button>
      ) : null}
    </div>
  )
}
