import { useState } from 'react'
import { Check } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  devServerId: string | null
  hasUserName: boolean
  hasUserEmail: boolean
  onSaved: () => void
}

// ─── Component ───────────────────────────────────────────────────────────────

/**
 * Card for setting or confirming `git config --global user.name / user.email`
 * on a remote dev server. Shows a read-only success state when both fields are
 * already configured.
 */
export function GitIdentityCard({ devServerId, hasUserName, hasUserEmail, onSaved }: Props) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const isConfigured = hasUserName && hasUserEmail

  const handleSave = async () => {
    if (!devServerId) {return}
    setSaving(true)
    setError(null)
    try {
      await window.api.onboarding.setGitIdentity({ devServerId, name, email })
      onSaved()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  if (isConfigured) {
    return (
      <div className="git-identity-card git-identity-card--configured">
        <Check className="size-4 text-green-500" aria-hidden />
        <span>Git identity configured</span>
      </div>
    )
  }

  return (
    <div className="git-identity-card">
      <p className="git-identity-card__label">
        Set your Git identity on this dev server
      </p>

      {!hasUserName && (
        <>
          <label htmlFor="git-user-name" className="git-identity-card__field-label">
            Name
          </label>
          <Input
            id="git-user-name"
            placeholder="Your Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </>
      )}

      {!hasUserEmail && (
        <>
          <label htmlFor="git-user-email" className="git-identity-card__field-label">
            Email
          </label>
          <Input
            id="git-user-email"
            type="email"
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </>
      )}

      {error && <p className="git-identity-card__error">{error}</p>}

      <Button
        id="git-identity-save-btn"
        onClick={() => void handleSave()}
        disabled={saving || (!name && !hasUserName) || (!email && !hasUserEmail)}
      >
        {saving ? 'Saving…' : 'Save Git identity'}
      </Button>
    </div>
  )
}
