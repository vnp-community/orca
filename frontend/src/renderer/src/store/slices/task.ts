import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { OrcaTask } from '../../../../shared/task-types'

export type TaskSlice = {
  tasks: OrcaTask[]
  activeTaskId: string | null
  taskLoading: boolean

  setTasks(tasks: OrcaTask[]): void
  addTask(task: OrcaTask): void
  updateTask(id: string, patch: Partial<OrcaTask>): void
  removeTask(id: string): void
  setActiveTask(id: string | null): void
  setTaskLoading(v: boolean): void
}

// Why every action returns a partial object instead of mutating `s` and
// returning nothing: this store has no immer middleware, so plain zustand's
// `set` treats a non-object return value (i.e. `undefined`, from a bare
// `set(s => { s.tasks = tasks })`) as a full-state REPLACE — wiping the
// entire AppState to `undefined`. Live-reproduced as a whole-app crash
// ("Cannot read properties of undefined (reading 'settings')" everywhere)
// the moment any Project Workspace Tasks-tab action ran for real data.
export const createTaskSlice: StateCreator<AppState, [], [], TaskSlice> = (set) => ({
  tasks: [],
  activeTaskId: null,
  taskLoading: false,

  setTasks: (tasks) => set(() => ({ tasks })),
  addTask: (task) => set((s) => ({ tasks: [...s.tasks, task] })),
  updateTask: (id, patch) =>
    set((s) => ({
      tasks: s.tasks.map((t) => (t.id === id ? { ...t, ...patch } : t))
    })),
  removeTask: (id) => set((s) => ({ tasks: s.tasks.filter((t) => t.id !== id) })),
  setActiveTask: (id) => set(() => ({ activeTaskId: id })),
  setTaskLoading: (v) => set(() => ({ taskLoading: v }))
})
