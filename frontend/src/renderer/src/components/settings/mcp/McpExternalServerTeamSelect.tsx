import { useEffect, useState } from 'react'
import { translate } from '@/i18n/i18n'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'

type TeamOption = { id: string; name: string }

/** Admin-only team picker; the team id becomes scopeId, the name is only a label. */
export function McpExternalServerTeamSelect({
  value,
  disabled,
  onChange
}: {
  value: string | undefined
  disabled?: boolean
  onChange: (teamId: string) => void
}): React.JSX.Element {
  const [teams, setTeams] = useState<TeamOption[] | null>(null)
  useEffect(() => {
    let live = true
    Promise.resolve()
      .then(() => window.api.admin.listTeams())
      .then(
        (list) => live && setTeams(list.map((t) => ({ id: t.id, name: t.name }))),
        () => live && setTeams([])
      )
    return () => {
      live = false
    }
  }, [])
  const options = teams ?? []
  const withCurrent =
    value && !options.some((t) => t.id === value)
      ? [...options, { id: value, name: value }]
      : options
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled || teams === null}>
      <SelectTrigger
        size="sm"
        aria-label={translate('auto.mcp.external.teamSelect', 'Team')}
        className="min-w-48"
      >
        <SelectValue
          placeholder={translate('auto.mcp.external.teamPlaceholder', 'Choose a team')}
        />
      </SelectTrigger>
      <SelectContent>
        {withCurrent.map((t) => (
          <SelectItem key={t.id} value={t.id}>
            {t.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
