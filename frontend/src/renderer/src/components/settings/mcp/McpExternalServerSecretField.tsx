import { useEffect, useRef } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

/**
 * Write-only secret entry. Why: the value lives only in the DOM input until commit (uncontrolled, no
 * React state), is handed to the parent once, and is wiped immediately and on unmount.
 */
export function McpExternalServerSecretField({
  label,
  onCommit,
  onCancel
}: {
  label: string
  onCommit: (value: string) => void
  onCancel: () => void
}): React.JSX.Element {
  const inputRef = useRef<HTMLInputElement>(null)
  const wipe = (): void => {
    if (inputRef.current) {
      inputRef.current.value = ''
    }
  }
  useEffect(() => {
    const el = inputRef.current
    return () => {
      if (el) {
        el.value = ''
      }
    }
  }, [])

  const commit = (): void => {
    const value = inputRef.current?.value ?? ''
    if (!value) {
      return
    }
    try {
      onCommit(value)
    } finally {
      wipe()
    }
  }

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <Input
          ref={inputRef}
          type="password"
          autoComplete="new-password"
          spellCheck={false}
          autoFocus
          data-testid="mcp-secret-input"
          aria-label={label}
          placeholder={translate('auto.mcp.external.secretPlaceholder', 'Secret value')}
          className="h-8 max-w-xs"
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              commit()
            }
          }}
        />
        <Button type="button" size="xs" onClick={commit}>
          {translate('auto.mcp.external.secretSet', 'Set')}
        </Button>
        <Button
          type="button"
          size="xs"
          variant="ghost"
          onClick={() => {
            wipe()
            onCancel()
          }}
        >
          {translate('auto.mcp.external.cancel', 'Cancel')}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        {translate(
          'auto.mcp.external.secretNote',
          'Sent over TLS and encrypted at rest by the server.'
        )}
      </p>
    </div>
  )
}
