// CredentialInput.tsx — write-only API key field for AI provider accounts.
// SECURITY: the key lives only in the uncontrolled <input> DOM node. It is never held in
// React state; the parent pulls it exactly once on save via the `take()` handle, which
// also clears the field.
import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react'
import type { AIProviderType } from '../../types/ai-provider-types'
import { Input } from '../ui/input'
import { Label } from '../ui/label'

export type CredentialInputHandle = {
  /** Returns the trimmed key and clears the field; null when nothing was entered. */
  take: () => string | null
}

type CredentialInputProps = {
  provider: AIProviderType
  hasExisting: boolean
  /** Fires only when "has a value" flips, never with the value itself. */
  onChange?: (hasValue: boolean) => void
}

const CREDENTIAL_LABELS: Record<AIProviderType, string | null> = {
  anthropic: 'Anthropic API Key (sk-ant-...)',
  openai: 'OpenAI API Key (sk-...)',
  gemini: 'Google API Key (AIza...)',
  azure: 'Azure OpenAI API Key',
  bedrock: 'AWS Credentials (JSON: accessKey + secret + region)',
  vllm: 'vLLM API Key (optional)',
  ollama: null // no credential needed
}

export const CredentialInput = forwardRef<CredentialInputHandle, CredentialInputProps>(
  function CredentialInput({ provider, hasExisting, onChange }, ref) {
    const inputRef = useRef<HTMLInputElement>(null)
    const [hasValue, setHasValue] = useState(false)
    const label = CREDENTIAL_LABELS[provider]

    const update = (next: boolean): void => {
      setHasValue(next)
      onChange?.(next)
    }

    useImperativeHandle(ref, () => ({
      take: () => {
        const el = inputRef.current
        const value = (el?.value ?? '').trim()
        if (el) {
          el.value = ''
        }
        setHasValue(false)
        return value === '' ? null : value
      }
    }))

    // Why: switching to a provider without a credential unmounts the field, so a typed
    // value is gone — tell the parent instead of leaving it believing one is pending.
    useEffect(() => {
      if (label === null) {
        setHasValue(false)
        onChange?.(false)
      }
    }, [label, onChange])

    useEffect(
      () => () => {
        if (inputRef.current) {
          inputRef.current.value = ''
        }
      },
      []
    )

    // Why after the hooks: returning before them changed the hook order when the provider
    // switched to/from one with no credential field.
    if (label === null) {
      return null
    }

    return (
      <div className="credential-input space-y-1">
        <Label htmlFor="ai-provider-credential">{label}</Label>
        {hasExisting && !hasValue && (
          <p className="text-xs text-muted-foreground">Leave blank to keep existing credential</p>
        )}
        <Input
          id="ai-provider-credential"
          ref={inputRef}
          type="password"
          placeholder="Enter API key..."
          defaultValue=""
          onChange={(e) => {
            const next = e.target.value.length > 0
            if (next !== hasValue) {
              update(next)
            }
          }}
          autoComplete="new-password"
          autoCorrect="off"
          autoCapitalize="off"
          spellCheck={false}
          data-testid="credential-input"
        />
        {hasValue && (
          <p className="text-xs text-muted-foreground" data-testid="credential-transport-note">
            Sent over TLS when you save and encrypted at rest by the server.
          </p>
        )}
      </div>
    )
  }
)
