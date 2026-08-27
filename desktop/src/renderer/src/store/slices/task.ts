import type { OrcaTask, TaskStatus } from '../../../../shared/task-types'

export type TaskSlice = {
  tasks:        OrcaTask[]
  activeTaskId: string | null
  taskLoading:  boolean
  
  setTasks(tasks: OrcaTask[]): void
  addTask(task: OrcaTask): void
  updateTask(id: string, patch: Partial<OrcaTask>): void
  removeTask(id: string): void
  setActiveTask(id: string | null): void
  setTaskLoading(v: boolean): void
}

export function createTaskSlice(set): TaskSlice {
  return {
    tasks: [], activeTaskId: null, taskLoading: false,
    setTasks:       (tasks) => set(s => { s.tasks = tasks }),
    addTask:        (task)  => set(s => { s.tasks.push(task) }),
    updateTask:     (id, patch) => set(s => {
      const idx = s.tasks.findIndex((t: OrcaTask) => t.id === id)
      if (idx !== -1) {Object.assign(s.tasks[idx], patch)}
    }),
    removeTask:     (id) => set(s => { s.tasks = s.tasks.filter((t: OrcaTask) => t.id !== id) }),
    setActiveTask:  (id) => set(s => { s.activeTaskId = id }),
    setTaskLoading: (v)  => set(s => { s.taskLoading = v }),
  }
}
