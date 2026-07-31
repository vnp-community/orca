import { useState, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import { toast } from 'sonner'
import type { WorkflowDefinition, WorkflowStep } from '@shared/workflow-types'

export function useWorkflow(templateId?: string) {
  const { templates, executions } = useAppStore(s => ({
    templates:  s.templates,
    executions: s.executions,
  }))
  const template = templateId ? templates.find(t => t.id === templateId) ?? null : null
  const [local, setLocal] = useState<Partial<WorkflowDefinition>>(template ?? {})

  const updateTemplate = useCallback((patch: Partial<WorkflowDefinition>) => {
    setLocal(prev => ({ ...prev, ...patch }))
  }, [])

  const addStep = useCallback(() => {
    const newStep: WorkflowStep = {
      id: `step-${Date.now()}`, type: 'agent', name: `Step ${(local.steps?.length ?? 0) + 1}`,
      serverSpec: 'project:current', config: { type: 'agent', prompt: '', worktreePath: '.' },
      dependsOn: [], continueOnError: false, timeout: 1800,
    }
    setLocal(prev => ({ ...prev, steps: [...(prev.steps ?? []), newStep] }))
    return newStep.id
  }, [local.steps])

  const removeStep = useCallback((stepId: string) => {
    setLocal(prev => ({
      ...prev,
      steps: (prev.steps ?? [])
        .filter(s => s.id !== stepId)
        .map(s => ({ ...s, dependsOn: s.dependsOn.filter(d => d !== stepId) }))
    }))
  }, [])

  const updateStep = useCallback((stepId: string, patch: Partial<WorkflowStep>) => {
    setLocal(prev => ({
      ...prev,
      steps: (prev.steps ?? []).map(s => s.id === stepId ? { ...s, ...patch } : s)
    }))
  }, [])

  const saveTemplate = useCallback(async () => {
    if (templateId) {
      await callRuntimeRpc('workflow.template.update', { templateId, ...local })
    } else {
      const created = await callRuntimeRpc('workflow.template.create', local) as WorkflowDefinition
      useAppStore.getState().addTemplate(created)
    }
    toast.success('Workflow saved')
  }, [templateId, local])

  const runWorkflow = useCallback(async (inputs?: Record<string, unknown>) => {
    if (!templateId) { toast.error('Save workflow first'); return null }
    const result = await callRuntimeRpc('workflow.execute', { templateId, inputs }) as { id: string }
    return result.id
  }, [templateId])

  return { template: local, templates, executions, addStep, removeStep, updateStep, updateTemplate, saveTemplate, runWorkflow }
}
