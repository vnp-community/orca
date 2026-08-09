// ModelSelector.tsx — AI model picker with approval filtering (TDD-FE-11)
import { useAppStore } from '../../store'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'

type ModelSelectorProps = {
  value?:          string
  onChange:        (model: string) => void
  disabled?:       boolean
  approvedModels?: string[]  // pass-through from resolved profile; falls back to store
}

const KNOWN_MODELS = [
  { id: 'claude-opus-4-5',  provider: 'anthropic', label: 'Claude Opus 4.5',  context: '200K' },
  { id: 'claude-sonnet-4',  provider: 'anthropic', label: 'Claude Sonnet 4',  context: '200K' },
  { id: 'gpt-4o',           provider: 'openai',    label: 'GPT-4o',           context: '128K' },
  { id: 'gemini-2.5-pro',   provider: 'google',    label: 'Gemini 2.5 Pro',   context: '1M'   },
  { id: 'gemini-2.5-flash', provider: 'google',    label: 'Gemini 2.5 Flash', context: '1M'   },
]

export function ModelSelector({ value, onChange, disabled, approvedModels: propApprovedModels }: ModelSelectorProps) {
  const storeApprovedModels = useAppStore(
    s => (s as any).resolvedProfile?.security?.approvedModels ?? []
  ) as string[]

  const approvedModels = propApprovedModels ?? storeApprovedModels

  const available = approvedModels.length > 0
    ? KNOWN_MODELS.filter(m =>
        approvedModels.some((ap: string) => m.id.startsWith(ap.replace(/\*/g, '')))
      )
    : KNOWN_MODELS

  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger data-testid="model-selector">
        <SelectValue placeholder="Select model..." />
      </SelectTrigger>
      <SelectContent>
        {available.map(m => (
          <SelectItem key={m.id} value={m.id}>
            <span>{m.label}</span>
            <span className="text-xs text-muted-foreground ml-2">({m.context})</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
