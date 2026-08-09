// OrcaInstanceSwitcher — web mode server picker with localStorage persistence (CR-006, TASK-006-A)
import { useState } from 'react'
import { Server, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useSavedOrcaInstances, type OrcaInstance } from '@/hooks/useSavedOrcaInstances'
import { AddInstanceForm } from './AddInstanceForm'

function formatRelativeTime(ts?: number): string {
  if (!ts) {return ''}
  const diff = Math.round((Date.now() - ts) / 1000)
  if (diff < 60) {return 'just now'}
  if (diff < 3600) {return `${Math.round(diff / 60)}m ago`}
  if (diff < 86400) {return `${Math.round(diff / 3600)}h ago`}
  return `${Math.round(diff / 86400)}d ago`
}

export function OrcaInstanceSwitcher({
  onSelect
}: {
  onSelect: (instance: OrcaInstance) => void
}): React.JSX.Element {
  const { instances, addInstance, removeInstance, updateLastConnected } = useSavedOrcaInstances()
  const [showAddForm, setShowAddForm] = useState(false)

  const handleSelect = (instance: OrcaInstance): void => {
    updateLastConnected(instance.id)
    onSelect(instance)
  }

  return (
    <div className="w-[400px] space-y-5">
      {/* Header */}
      <div>
        <h1 className="text-xl font-semibold">
          {translate('instanceSwitcher.title', 'Connect to Orca')}
        </h1>
        <p className="text-sm text-muted-foreground">
          {translate(
            'instanceSwitcher.subtitle',
            'Select your team server or add a new one.'
          )}
        </p>
      </div>

      {/* Saved instance list — hidden when add form is open */}
      {instances.length > 0 && !showAddForm && (
        <div className="space-y-1.5">
          {instances
            .slice()
            .sort((a, b) => (b.lastConnectedAt ?? 0) - (a.lastConnectedAt ?? 0))
            .map((instance) => (
              <div key={instance.id} className="group relative">
                <button
                  type="button"
                  className="flex w-full items-center gap-3 rounded-md border px-4 py-3 text-left transition-colors hover:bg-muted/50"
                  onClick={() => handleSelect(instance)}
                >
                  <div className="flex size-8 flex-shrink-0 items-center justify-center rounded-full bg-primary/10">
                    <Server className="size-4 text-primary" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{instance.label}</p>
                    <p className="truncate text-xs text-muted-foreground">{instance.url}</p>
                  </div>
                  {instance.lastConnectedAt && (
                    <span className="flex-shrink-0 text-xs text-muted-foreground">
                      {formatRelativeTime(instance.lastConnectedAt)}
                    </span>
                  )}
                </button>
                {/* Delete button — visible on hover via group-hover */}
                <button
                  type="button"
                  className="absolute right-2 top-1/2 hidden -translate-y-1/2 rounded p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground group-hover:flex"
                  onClick={(e) => {
                    e.stopPropagation()
                    removeInstance(instance.id)
                  }}
                >
                  <Trash2 className="size-3.5" />
                  <span className="sr-only">Remove instance</span>
                </button>
              </div>
            ))}
        </div>
      )}

      {/* Add form or Add button */}
      {showAddForm ? (
        <AddInstanceForm
          onAdd={(instance) => {
            addInstance(instance)
            setShowAddForm(false)
          }}
          onCancel={() => setShowAddForm(false)}
        />
      ) : (
        <Button
          variant="outline"
          className="w-full gap-2"
          onClick={() => setShowAddForm(true)}
        >
          <Plus className="size-4" />
          {translate('instanceSwitcher.addServer', 'Add Orca server')}
        </Button>
      )}
    </div>
  )
}
