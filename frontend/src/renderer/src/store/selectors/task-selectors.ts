// Selectors cho task store (CR-ORCA-003: batch update sau onCRSStatusUpdated)

import type { AppState } from '../types'

// Tìm taskIds có kg_context.crs_id = crsId
// Dùng trong onCRSStatusUpdated handler để batch update KG badge
export const selectTaskIdsByCRSId = (state: AppState, crsId: string): string[] => {
  // tasks lưu trong TaskSlice — adapt theo actual store shape
  const taskSlice = state as unknown as {
    tasks?: Record<string, { id: string; kg_context?: { crs_id?: string } }>
  }
  const tasks = Object.values(taskSlice.tasks ?? {})
  return tasks
    .filter((task) => task.kg_context?.crs_id === crsId)
    .map((task) => task.id)
}
