import { useState } from 'react'
import { LockIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { McpExternalServerSecretField } from './McpExternalServerSecretField'

export type RefRowsProps = {
  /** Used for aria-labels, e.g. "environment variable" / "header". */
  noun: string
  names: string[]
  /** Names that already existed on the server (read-only; rename = remove + add). */
  savedNames: ReadonlySet<string>
  /** Server-reported hasSecret per saved name. */
  hasSecret: ReadonlyMap<string, boolean>
  /** Names with a value staged for the next Save (no value is exposed here). */
  staged: ReadonlySet<string>
  rowErrors: ReadonlyMap<string, string>
  error?: string
  onAdd: (name: string) => void
  onRemove: (name: string) => void
  onStage: (name: string, value: string) => void
  onUnstage: (name: string) => void
}

export function McpExternalServerRefRows(p: RefRowsProps): React.JSX.Element {
  const [newName, setNewName] = useState('')
  const [editing, setEditing] = useState<string | null>(null)
  const add = (): void => {
    const n = newName.trim()
    if (n && !p.names.includes(n)) {
      p.onAdd(n)
    }
    setNewName('')
  }
  return (
    <div className="space-y-2">
      {p.names.map((name) => {
        const set = p.staged.has(name) || p.hasSecret.get(name) === true
        const err = p.rowErrors.get(name)
        return (
          <div key={name} className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-mono text-sm">{name}</span>
              {set ? (
                <Badge variant="secondary">
                  <LockIcon aria-hidden />
                  {p.staged.has(name)
                    ? translate('auto.mcp.external.secretStaged', 'Secret ready to save')
                    : translate('auto.mcp.external.secretIsSet', 'Secret set')}
                </Badge>
              ) : (
                <Badge variant="outline">
                  {translate('auto.mcp.external.secretNone', 'No value yet')}
                </Badge>
              )}
              <Button
                type="button"
                size="xs"
                variant="outline"
                aria-label={translate('auto.mcp.external.secretSetAria', 'Set value for {{name}}', {
                  name
                })}
                onClick={() => setEditing(name)}
              >
                {set
                  ? translate('auto.mcp.external.secretReplace', 'Replace')
                  : translate('auto.mcp.external.secretSet', 'Set')}
              </Button>
              {p.staged.has(name) ? (
                <Button type="button" size="xs" variant="ghost" onClick={() => p.onUnstage(name)}>
                  {translate('auto.mcp.external.secretDiscard', 'Discard value')}
                </Button>
              ) : null}
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label={translate('auto.mcp.external.refRemove', 'Remove {{name}}', { name })}
                onClick={() => p.onRemove(name)}
              >
                <Trash2Icon aria-hidden />
              </Button>
            </div>
            {editing === name ? (
              <McpExternalServerSecretField
                label={translate('auto.mcp.external.secretFor', 'Value for {{name}}', { name })}
                onCommit={(v) => {
                  p.onStage(name, v)
                  setEditing(null)
                }}
                onCancel={() => setEditing(null)}
              />
            ) : null}
            {err ? (
              <p role="alert" className="text-xs text-destructive">
                {err}
              </p>
            ) : null}
          </div>
        )
      })}
      <div className="flex items-center gap-2">
        <Input
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              add()
            }
          }}
          aria-label={translate('auto.mcp.external.refNewName', 'New {{noun}} name', {
            noun: p.noun
          })}
          placeholder={translate('auto.mcp.external.refNamePlaceholder', 'NAME')}
          className="h-8 max-w-xs font-mono"
        />
        <Button type="button" variant="outline" size="xs" onClick={add}>
          <PlusIcon aria-hidden />
          {translate('auto.mcp.external.refAdd', 'Add')}
        </Button>
      </div>
      {p.error ? (
        <p role="alert" className="text-xs text-destructive">
          {p.error}
        </p>
      ) : null}
    </div>
  )
}
