# TC-TG-001 — Task Graph CRUD & Structural Management

**BL Reference:** BL-TG-01  
**Flow Reference:** docs/flows/logic/task-graph.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Developer, Lead, Admin

---

## TC-TG-001-01: Tạo Epic task

**Priority:** P0

### Steps
1. `task.create { type: 'epic', title: 'Authentication System', projectId, description: '...' }`

### Expected Results
- Task created với type='epic', status='backlog'
- DB: INSERT `orca_tasks { id, title, type: 'epic', status: 'backlog', owner_id }`

### Assertions
```
task = await rpc.call('task.create', {
  type: 'epic',
  title: 'Authentication System',
  projectId: 'proj-123'
})
assert task.type === 'epic'
assert task.status === 'backlog'
dbTask = db.tasks.find({ id: task.id })
assert dbTask !== undefined
assert dbTask.owner_id === currentUser.id
```

---

## TC-TG-001-02: Tạo Story — Child của Epic

**Priority:** P0

### Steps
1. `task.create { type: 'story', title: 'Login Flow', parentId: epicId }`

### Expected Results
- Story created với parentId=epicId
- Epic's `total_subtasks` incremented to 1
- Epic progress auto-updated

### Assertions
```
story = await rpc.call('task.create', { type: 'story', title: 'Login Flow', parentId: epic.id })
assert story.parentId === epic.id
epic = await rpc.call('task.get', { taskId: epic.id })
assert epic.totalSubtasks === 1
```

---

## TC-TG-001-03: Dependency edge — A depends on B

**Priority:** P0

### Steps
1. Tạo Task A và Task B
2. `task.addDependency { fromId: A, toId: B, edgeType: 'depends_on' }`

### Expected Results
- Dependency edge: A → B (A depends on B completing first)
- DB: `orca_task_edges { from_task_id: A.id, to_task_id: B.id, edge_type: 'depends_on' }`
- IF B is not done: A.status → 'blocked'

### Assertions
```
await rpc.call('task.addDependency', { fromId: A.id, toId: B.id, edgeType: 'depends_on' })
edge = db.taskEdges.find({ from_task_id: A.id, to_task_id: B.id })
assert edge !== undefined
assert edge.edge_type === 'depends_on'
taskA = await rpc.call('task.get', { taskId: A.id })
assert taskA.status === 'blocked'  // B is not done yet
```

---

## TC-TG-001-04: Cycle detection — A → B → A (direct cycle, reject)

**Priority:** P0

### Steps
1. Task A depends on Task B: `A → B`
2. Try: `task.addDependency { fromId: B, toId: A }` (would create cycle)

### Expected Results
- Error: `{ code: 'CYCLE_DETECTED', path: ['B', 'A', 'B'] }`
- Dependency NOT added
- DB unchanged

### Assertions
```
await rpc.call('task.addDependency', { fromId: A.id, toId: B.id, edgeType: 'depends_on' })
result = await rpc.call('task.addDependency', { fromId: B.id, toId: A.id }).catch(e => e)
assert result.code === 'CYCLE_DETECTED'
assert db.taskEdges.count({ from_task_id: B.id, to_task_id: A.id }) === 0
```

---

## TC-TG-001-05: Cycle detection — Transitive (A → B → C → A)

**Priority:** P0

### Steps
1. A → B, B → C already exist
2. Try: `C → A`

### Expected Results
- Error: `{ code: 'CYCLE_DETECTED', path: ['C', 'A', 'B', 'C'] }`
- C → A NOT added

---

## TC-TG-001-06: Task types — All valid types accepted

**Priority:** P1

### Steps
1. Test create with each type: `epic, story, task, subtask, bug, spike`

### Expected Results
- All 6 types accepted and stored correctly
- Invalid type 'unknown' → 400 INVALID_TYPE

### Assertions
```
const types = ['epic', 'story', 'task', 'subtask', 'bug', 'spike']
for (const type of types) {
  task = await rpc.call('task.create', { type, title: `Test ${type}` })
  assert task.type === type
}
// Invalid type
result = await rpc.call('task.create', { type: 'unknown', title: 'Bad' }).catch(e => e)
assert result.code === 'INVALID_TYPE'
```

---

## TC-TG-001-07: Task status transition — Happy path

**Priority:** P0

### Steps
1. Task created → status='backlog'
2. Assign → status='todo'
3. Start → status='in_progress'
4. Agent completes → status='review'
5. Approve → status='done'

### Expected Results
- Each transition succeeds
- DB updated per transition

### Assertions
```
task = createTask()
assert task.status === 'backlog'

await rpc.call('task.update', { taskId: task.id, status: 'todo' })
assert (await getTask(task.id)).status === 'todo'

await rpc.call('task.update', { taskId: task.id, status: 'in_progress' })
await rpc.call('task.update', { taskId: task.id, status: 'review' })
await rpc.call('task.update', { taskId: task.id, status: 'done' })
assert (await getTask(task.id)).status === 'done'
```

---

## TC-TG-001-08: Auto-unblock — Task B unblocked when dependency A done

**Priority:** P0

### Preconditions
- Task A, Task B: B depends_on A
- B.status = 'blocked'

### Steps
1. `task.update { taskId: A.id, status: 'done' }`
2. Verify B auto-unblocked

### Expected Results
- B.status → 'todo' (auto, triggered by A's status change)
- DB updated within same transaction

### Assertions
```
// Setup
await rpc.call('task.addDependency', { fromId: B.id, toId: A.id, edgeType: 'depends_on' })
taskB = await getTask(B.id)
assert taskB.status === 'blocked'

// Mark A done
await rpc.call('task.update', { taskId: A.id, status: 'done' })

// B auto-unblocked
await delay(100)  // allow async processing
taskB = await getTask(B.id)
assert taskB.status === 'todo'
```

---

## TC-TG-001-09: Progress tracking — done_subtasks / total_subtasks

**Priority:** P0

### Steps
1. Epic với 4 stories
2. Mark 2 stories as 'done'
3. Check progress

### Expected Results
- `epic.doneSubtasks === 2`
- `epic.totalSubtasks === 4`
- `epic.progress === 50%`

### Assertions
```
markTaskDone(story1.id)
markTaskDone(story2.id)
epic = db.tasks.find({ id: epicId })
assert epic.doneSubtasks === 2
assert epic.totalSubtasks === 4
assert Math.round(epic.doneSubtasks / epic.totalSubtasks * 100) === 50
```

---

## TC-TG-001-10: Progress tracking — Recursive (subtask của story)

**Priority:** P0

### Steps
1. Epic → Story → Subtask × 3
2. Mark 2 subtasks done
3. Check epic progress (recursive)

### Expected Results
- Story progress: 2/3 subtasks done
- Epic progress reflects story progress recursively

---

## TC-TG-001-11: Delete task có children — Cascade

**Priority:** P1

### Steps
1. Epic với 3 stories (each với 2 subtasks)
2. Delete epic

### Expected Results
- Epic deleted
- All 3 stories deleted (cascade)
- All 6 subtasks deleted (cascade)
- All dependency edges cleaned up
- DB: `orca_tasks.count` reduced by 10 (1 epic + 3 stories + 6 subtasks)

### Assertions
```
epicId = epic.id
await rpc.call('task.delete', { taskId: epicId })
assert db.tasks.find({ id: epicId }) === undefined
assert db.tasks.count({ parentId: epicId }) === 0
assert db.taskEdges.count({ from_task_id: epicId }) === 0
```

---

## TC-TG-001-12: depends_on edge types — blocks, relates_to

**Priority:** P1

### Steps
1. A `blocks` B: `task.addDependency { fromId: A, toId: B, edgeType: 'blocks' }`
2. A `relates_to` B: `task.addDependency { fromId: A, toId: B, edgeType: 'relates_to' }`

### Expected Results
- 'blocks': B.status → 'blocked', A does not become blocked
- 'relates_to': informational only, no status change

---

*TC-TG-001 — Orca v5.0 — Updated 2026-08-01*
