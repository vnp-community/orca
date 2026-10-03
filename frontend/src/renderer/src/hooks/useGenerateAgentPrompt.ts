import { useCallback, useRef, useState } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { translate } from '@/i18n/i18n'

export type GenerateAgentPromptResult = { prompt: string } | { error: string }

function describeFailure(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err ?? '')
  if (/permission/i.test(raw)) {
    return translate(
      'auto.hooks.useGenerateAgentPrompt.permissionDenied',
      'You do not have permission to generate a prompt for this task.'
    )
  }
  if (/failed.?precondition/i.test(raw)) {
    return translate(
      'auto.hooks.useGenerateAgentPrompt.noDevServer',
      'Connect a dev server for this project to generate a prompt.'
    )
  }
  if (/unimplemented|unsupported|not supported|unknown method/i.test(raw)) {
    return translate(
      'auto.hooks.useGenerateAgentPrompt.unsupported',
      'Prompt generation is not supported by the connected runtime.'
    )
  }
  return (
    raw ||
    translate('auto.hooks.useGenerateAgentPrompt.failed', 'Failed to generate the agent prompt.')
  )
}

// Always save=false: the user reviews and saves through the normal edit flow.
export function useGenerateAgentPrompt() {
  const [isGenerating, setIsGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Ref guard: state updates are async, so a fast double-click could slip past isGenerating.
  const inFlight = useRef(false)

  const generate = useCallback(async (taskId: string): Promise<GenerateAgentPromptResult> => {
    if (inFlight.current) {
      return { error: translate('auto.hooks.useGenerateAgentPrompt.busy', 'Already generating.') }
    }
    inFlight.current = true
    setIsGenerating(true)
    setError(null)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const res = await callRuntimeRpc<{ prompt: string }>(target, 'task.generateAgentPrompt', {
        taskId,
        save: false
      })
      return { prompt: res?.prompt ?? '' }
    } catch (err) {
      const message = describeFailure(err)
      setError(message)
      return { error: message }
    } finally {
      inFlight.current = false
      setIsGenerating(false)
    }
  }, [])

  return { generate, isGenerating, error }
}
