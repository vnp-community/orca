import { Loader2, Sparkles } from 'lucide-react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { useGenerateAgentPrompt } from '../../hooks/useGenerateAgentPrompt'
import { Button } from '../ui/button'

type Props = {
  taskId: string
  currentPrompt: string
  onGenerated: (prompt: string) => void
}

export function GenerateAgentPromptButton({ taskId, currentPrompt, onGenerated }: Props) {
  const { generate, isGenerating } = useGenerateAgentPrompt()

  const onClick = async () => {
    const result = await generate(taskId)
    if ('error' in result) {
      toast.error(result.error)
      return
    }
    // Never silently overwrite text the user typed.
    if (
      currentPrompt.trim() &&
      !window.confirm(
        translate(
          'auto.components.task.GenerateAgentPromptButton.confirmReplace',
          'Replace the current prompt with the generated one?'
        )
      )
    ) {
      return
    }
    onGenerated(result.prompt)
  }

  const label = translate(
    'auto.components.task.GenerateAgentPromptButton.label',
    'Generate with AI'
  )
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={onClick}
      disabled={isGenerating}
      aria-label={label}
      data-testid="generate-agent-prompt-btn"
    >
      {isGenerating ? (
        <Loader2 size={12} className="animate-spin mr-1" />
      ) : (
        <Sparkles size={12} className="mr-1" />
      )}
      {label}
    </Button>
  )
}
