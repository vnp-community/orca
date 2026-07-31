# TASK-V5-13: DiffViewer + BranchManager

**Order:** 13 | **Prerequisite:** TASK-V5-12 | **Tests:** 9

---

## Files Cần Tạo

### 1. `src/renderer/src/components/workspace/git/DiffViewer.tsx`

Parse unified diff format → color-coded lines:
- `+` lines → `bg-green-50 text-green-800`
- `-` lines → `bg-red-50 text-red-800`
- `@@` hunk headers → `bg-gray-100 text-gray-500 font-mono text-xs`
- unchanged lines → transparent

```typescript
interface DiffViewerProps {
  filePath: string
  staged?:  boolean
}

export function DiffViewer({ filePath, staged }: DiffViewerProps) {
  const { project } = useWorkspace()
  const diffContent = useAppStore(s => s.diffContent)
  
  useEffect(() => {
    if (!project || !filePath) return
    callRuntimeRpc('git.getDiff', { projectId: project.id, path: filePath, staged })
      .then(diff => useAppStore.getState().setDiffContent(diff as string))
  }, [project, filePath, staged])

  if (!diffContent) return <div className="p-4 text-sm text-muted-foreground">No changes</div>

  return (
    <div className="diff-viewer font-mono text-xs overflow-auto" data-testid="diff-viewer">
      {diffContent.split('\n').map((line, i) => (
        <div
          key={i}
          className={
            line.startsWith('+') ? 'bg-green-50 text-green-800' :
            line.startsWith('-') ? 'bg-red-50 text-red-800' :
            line.startsWith('@@') ? 'bg-gray-100 text-gray-500' :
            ''
          }
          data-testid={`diff-line-${i}`}
        >
          <span className="select-none text-gray-300 w-8 inline-block">{line[0] ?? ' '}</span>
          {line.slice(1)}
        </div>
      ))}
    </div>
  )
}
```

### 2. `src/renderer/src/components/workspace/git/BranchManager.tsx`

```typescript
// Lists branches, current branch indicator, create/checkout/delete actions

export function BranchManager() {
  const { project } = useWorkspace()
  const branches    = useAppStore(s => s.branches)
  const [newBranch, setNewBranch] = useState('')

  useEffect(() => {
    if (!project) return
    callRuntimeRpc('git.listBranches', { projectId: project.id })
      .then(bs => useAppStore.getState().setBranches(bs as any[]))
  }, [project])

  const checkout = async (branch: string) => {
    await callRuntimeRpc('git.checkout', { projectId: project!.id, branch })
    // Refresh branches
    const bs = await callRuntimeRpc('git.listBranches', { projectId: project!.id })
    useAppStore.getState().setBranches(bs as any[])
  }

  const create = async () => {
    if (!newBranch.trim()) return
    await callRuntimeRpc('git.createBranch', { projectId: project!.id, name: newBranch.trim() })
    setNewBranch('')
    checkout(newBranch.trim())
  }

  return (
    <div className="branch-manager p-2 space-y-3" data-testid="branch-manager">
      {/* Create branch */}
      <div className="flex gap-2">
        <Input value={newBranch} onChange={e => setNewBranch(e.target.value)} placeholder="New branch name..." className="text-sm" />
        <Button size="sm" onClick={create} disabled={!newBranch.trim()}>Create</Button>
      </div>

      {/* Branch list */}
      <div className="space-y-0.5">
        {branches.map(b => (
          <div
            key={b.name}
            className={`flex items-center gap-2 px-2 py-1.5 rounded text-sm ${b.isCurrent ? 'bg-accent' : 'hover:bg-accent/50'}`}
            data-testid={`branch-${b.name}`}
          >
            <GitBranch size={12} className="text-muted-foreground shrink-0" />
            <span className="flex-1 truncate">{b.name}</span>
            {b.isCurrent && <Badge className="text-xs">current</Badge>}
            {!b.isCurrent && (
              <Button size="sm" variant="ghost" className="h-5 text-xs" onClick={() => checkout(b.name)}>
                Checkout
              </Button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
```

---

## Tests (9 total)

```
__tests__/workspace/git/DiffViewer.test.tsx   (4 tests)
  + lines → bg-green-50 class
  - lines → bg-red-50 class
  @@ lines → bg-gray-100 class
  empty diff → "No changes" text

__tests__/workspace/git/BranchManager.test.tsx  (5 tests)
  fetches branches on mount
  current branch has 'current' badge
  non-current branch has Checkout button
  Checkout button calls git.checkout
  Create branch: calls git.createBranch + checkout
```

---

## Acceptance Criteria

- [x] DiffViewer color coding correct for +/-/@@/unchanged lines
- [x] BranchManager fetches `git.listBranches` on mount
- [x] Checkout calls `git.checkout` RPC
- [x] Create branch: input + button, calls `git.createBranch`
- [x] 9/9 tests pass
