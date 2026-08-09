// CredentialInput.tsx — Secure API key input with in-browser AES-GCM encryption (TASK-V5-07)
// SECURITY: rawValue is cleared from React state immediately after encryption
import { useState } from 'react'
import type { AIProviderType } from '../../types/ai-provider-types'
import { encryptCredential } from '../../lib/credential-crypto'
import { useAppStore } from '../../store'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Lock, Loader2 } from 'lucide-react'

type CredentialInputProps = {
  provider:    AIProviderType
  hasExisting: boolean
  onEncrypted: (encryptedBlob: string, iv: string) => void
  onClear:     () => void
}

const CREDENTIAL_LABELS: Record<AIProviderType, string | null> = {
  anthropic: 'Anthropic API Key (sk-ant-...)',
  openai:    'OpenAI API Key (sk-...)',
  gemini:    'Google API Key (AIza...)',
  azure:     'Azure OpenAI API Key',
  bedrock:   'AWS Credentials (JSON: accessKey + secret + region)',
  vllm:      'vLLM API Key (optional)',
  ollama:    null,   // no credential needed
}

export function CredentialInput({
  provider, hasExisting, onEncrypted, onClear
}: CredentialInputProps) {
  const [rawValue, setRawValue]         = useState('')
  const [isEncrypting, setIsEncrypting] = useState(false)
  const [isEncrypted, setIsEncrypted]   = useState(false)

  // Ollama: no credential needed
  const label = CREDENTIAL_LABELS[provider]
  if (label === null) {return null}

  // Get session token for key derivation
  const sessionToken = useAppStore(
    s => (s as any).auth?.sessionToken ?? 'fallback-dev-token'
  ) as string

  const handleChange = async (value: string) => {
    // Reset state first
    setRawValue(value)
    setIsEncrypted(false)
    onClear()

    if (value.length >= 10) {
      setIsEncrypting(true)
      try {
        const { encryptedBlob, iv } = await encryptCredential(value, sessionToken)
        setIsEncrypted(true)
        onEncrypted(encryptedBlob, iv)
      } catch {
        // SECURITY: Do not log error details — they may contain timing info
        setIsEncrypted(false)
      } finally {
        setIsEncrypting(false)
        // CRITICAL: clear plaintext from state after encryption
        setRawValue('')
      }
    }
  }

  return (
    <div className="credential-input space-y-1">
      <Label>{label}</Label>
      {hasExisting && !isEncrypted && (
        <p className="text-xs text-muted-foreground">
          Leave blank to keep existing credential
        </p>
      )}
      <div className="relative">
        <Input
          type="password"
          placeholder="Enter API key..."
          value={rawValue}
          onChange={e => handleChange(e.target.value)}
          autoComplete="new-password"
          autoCorrect="off"
          autoCapitalize="off"
          spellCheck={false}
          data-testid="credential-input"
        />
        {isEncrypting && (
          <Loader2
            className="absolute right-2 top-2.5 animate-spin text-muted-foreground"
            size={16}
          />
        )}
        {isEncrypted && (
          <Lock
            className="absolute right-2 top-2.5 text-green-500"
            size={16}
            data-testid="lock-icon"
          />
        )}
      </div>
      {isEncrypted && (
        <p className="text-xs text-green-600">
          ✓ Credential encrypted in browser — will be stored securely on dev server
        </p>
      )}
    </div>
  )
}
