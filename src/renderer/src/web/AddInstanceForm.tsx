// AddInstanceForm — form to add a new Orca server instance (CR-006, TASK-006-A)
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { translate } from '@/i18n/i18n'
import type { OrcaInstance } from '@/hooks/useSavedOrcaInstances'

type AddInstanceFormProps = {
  onAdd: (instance: OrcaInstance) => void
  onCancel: () => void
}

export function AddInstanceForm({ onAdd, onCancel }: AddInstanceFormProps): React.JSX.Element {
  const [label, setLabel] = useState('')
  const [url, setUrl] = useState('https://')
  const [team, setTeam] = useState('')

  const handleSubmit = (e: React.FormEvent): void => {
    e.preventDefault()
    if (!label.trim() || !url.trim()) return
    onAdd({
      id: crypto.randomUUID(),
      label: label.trim(),
      url: url.trim().replace(/\/$/, ''),
      team: team.trim() || undefined
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      <div className="space-y-1.5">
        <Label htmlFor="instance-label">
          {translate('instanceForm.label', 'Display name')}
        </Label>
        <Input
          id="instance-label"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          placeholder={translate('instanceForm.labelPlaceholder', 'e.g. Team Backend')}
          required
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="instance-url">
          {translate('instanceForm.url', 'Orca server URL')}
        </Label>
        <Input
          id="instance-url"
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://orca.yourteam.internal"
          required
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="instance-team">
          {translate('instanceForm.team', 'Team (optional)')}
        </Label>
        <Input
          id="instance-team"
          value={team}
          onChange={(e) => setTeam(e.target.value)}
          placeholder={translate('instanceForm.teamPlaceholder', 'e.g. backend, frontend')}
        />
      </div>

      <div className="flex gap-2 pt-1">
        <Button type="button" variant="ghost" onClick={onCancel} className="flex-1">
          {translate('common.cancel', 'Cancel')}
        </Button>
        <Button type="submit" className="flex-1" disabled={!label.trim() || !url.trim()}>
          {translate('instanceForm.add', 'Add Server')}
        </Button>
      </div>
    </form>
  )
}
