import { DndContext, closestCenter, type DragEndEvent } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy, useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { WorkflowStep } from '@shared/workflow-types'
import { Plus } from 'lucide-react'

function SortableStep({ step, isSelected, onSelect }: { step: WorkflowStep, isSelected: boolean, onSelect: (id: string) => void }) {
  const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id: step.id })
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={`flex items-center gap-2 px-2 py-1.5 cursor-pointer hover:bg-accent/50 rounded ${isSelected ? 'bg-accent' : ''}`}
      onClick={() => onSelect(step.id)}
      data-testid={`step-item-${step.id}`}
    >
      <span {...attributes} {...listeners} className="cursor-grab text-muted-foreground">⠿</span>
      <span className="text-xs font-mono text-muted-foreground">{step.type}</span>
      <span className="text-sm truncate">{step.name}</span>
    </div>
  )
}

export function StepList({ steps, selectedStepId, onSelect, onAdd, onReorder }: any) {
  const handleDragEnd = (e: DragEndEvent) => {
    const { active, over } = e
    if (!over || active.id === over.id) return
    const from = steps.findIndex((s: any) => s.id === active.id)
    const to   = steps.findIndex((s: any) => s.id === over.id)
    if (from !== -1 && to !== -1) onReorder(from, to)
  }

  return (
    <div className="step-list" data-testid="step-list">
      <DndContext collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={steps.map((s: any) => s.id)} strategy={verticalListSortingStrategy}>
          {steps.map((step: any) => (
            <SortableStep key={step.id} step={step} isSelected={step.id === selectedStepId} onSelect={onSelect} />
          ))}
        </SortableContext>
      </DndContext>
      <button
        className="flex items-center gap-1 px-2 py-1 text-sm text-muted-foreground hover:text-foreground mt-1"
        onClick={onAdd}
        data-testid="add-step-btn"
      >
        <Plus size={12} /> Add Step
      </button>
    </div>
  )
}
