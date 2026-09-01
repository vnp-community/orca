// dev-servers-selectors.ts holds the useAppStore-reading convenience hooks
// for DevServerSlice (dev-servers.ts) — kept in a SEPARATE file, not
// re-exported from dev-servers.ts itself, because store/index.ts imports
// dev-servers.ts directly to build createDevServerSlice into the aggregate
// store. A hook here needs a normal top-level `import { useAppStore } from
// '../index'`, which store/index.ts's own import of dev-servers.ts would
// turn into a genuine circular module dependency if the two lived together
// — observed live: createDevServerSlice resolving to undefined
// ("createDevServerSlice is not a function") in Vitest's module graph, even
// after trying a namespace import to defer the property access (the cycle
// blocks at module-evaluation time, not property-access time, so deferring
// the access alone doesn't help — only removing the import edge from
// dev-servers.ts entirely does). store/index.ts never needs to import this
// file, so it never re-creates the cycle.
import { useShallow } from 'zustand/react/shallow'
import type { DevServer } from '../../../../shared/dev-server-types'
import { useAppStore } from '../index'

/** All dev servers (stable reference via shallow equality) */
export function useDevServers(): DevServer[] {
  return useAppStore(useShallow((s) => s.devServers))
}

/** Currently active dev server, or null */
export function useActiveDevServer(): DevServer | null {
  return useAppStore(
    useShallow((s) => {
      const id = s.activeDevServerId
      return id ? (s.devServers.find((ds) => ds.id === id) ?? null) : null
    })
  )
}

/** Only servers that are actively connected */
export function useConnectedDevServers(): DevServer[] {
  return useAppStore(useShallow((s) => s.devServers.filter((ds) => ds.status === 'connected')))
}

/** Look up a single server by id */
export function useDevServerById(id: string | null): DevServer | null {
  return useAppStore((s) => (id ? (s.devServers.find((ds) => ds.id === id) ?? null) : null))
}
