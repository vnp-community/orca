// Zustand slice: cache KG enrichment per task (CR-ORCA-002, CR-ORCA-003)
// Tách slice riêng vì enrichment là derived data với lifecycle riêng

import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { TaskEnrichment } from '../../types/task-types'

export interface TaskEnrichmentSlice {
  taskEnrichments: Record<string, TaskEnrichment>

  // Loading state khi fetch bắt đầu
  setTaskEnrichmentLoading: (taskId: string) => void

  // Lưu kết quả fetch (best-effort — cả hai có thể undefined nếu fetch fail)
  setTaskEnrichment: (
    taskId: string,
    crsContext?: unknown,
    featureContext?: unknown
  ) => void

  // Lưu error khi cả hai fetch fail
  setTaskEnrichmentError: (taskId: string, error: string) => void

  // CR-ORCA-003: cập nhật CRS status trong crsContext sau quality gate
  // Gọi từ onCRSStatusUpdated IPC event (PISTool.UpdateCRSStatus → "done")
  updateEnrichmentCRSStatus: (taskId: string, newStatus: string) => void
}

export const createTaskEnrichmentSlice: StateCreator<
  AppState,
  [],
  [],
  TaskEnrichmentSlice
> = (set) => ({
  taskEnrichments: {},

  setTaskEnrichmentLoading: (taskId) =>
    set((s) => ({
      taskEnrichments: {
        ...s.taskEnrichments,
        [taskId]: {
          ...(s.taskEnrichments[taskId] ?? { taskId, fetchedAt: 0, loading: false }),
          loading: true,
        },
      },
    })),

  setTaskEnrichment: (taskId, crsContext, featureContext) =>
    set((s) => ({
      taskEnrichments: {
        ...s.taskEnrichments,
        [taskId]: {
          taskId,
          crsContext,
          featureContext,
          fetchedAt: Date.now(),
          loading: false,
        },
      },
    })),

  setTaskEnrichmentError: (taskId, error) =>
    set((s) => ({
      taskEnrichments: {
        ...s.taskEnrichments,
        [taskId]: {
          ...(s.taskEnrichments[taskId] ?? { taskId, fetchedAt: 0, loading: false }),
          loading: false,
          error,
        },
      },
    })),

  // Best-effort CRS status patch (SOL-ORCA-003 §2.4)
  // Chỉ patch field 'status' trong crsContext raw JSON — không replace whole object
  updateEnrichmentCRSStatus: (taskId, newStatus) =>
    set((s) => {
      const entry = s.taskEnrichments[taskId]
      if (entry?.crsContext == null) return s
      const updated = {
        ...(entry.crsContext as Record<string, unknown>),
        status: newStatus,
      }
      return {
        taskEnrichments: {
          ...s.taskEnrichments,
          [taskId]: { ...entry, crsContext: updated },
        },
      }
    }),
})
