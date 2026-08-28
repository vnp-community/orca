# TASK-SSH-02-01: Embed `AGENT_VERSION` as a bundle-exported constant

**From Solution:** SOL-SSH-02
**Priority:** P0 — TASK-SSH-02-02's remote version probe depends on this
**Service:** agent/ (Dev Server Agent)
**File:** `agent/build.mjs`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`sshrelay`'s new `remoteVersionAndPresence` probe (TASK-SSH-02-02) runs
`node -e "console.log(require('./agent.js').AGENT_VERSION||'unknown')"`
against the deployed bundle. `build.mjs` already computes a version string
(`AGENT_VERSION` const + content hash, written to `out/.agent-version`) but
never injects it into the bundle itself — `agent-entry.ts` exports nothing,
so `require('./agent.js')` returns `{}` today, not a version. This task
makes the version actually readable from the built artifact.

## Changes to make

`agent/build.mjs` — inject the version as a `define` so it becomes a real
compile-time constant, not a runtime env lookup (works identically whether
launched via `node agent.js --stdio` or `--detach`, no new env var to plumb
through `sshrelay/launch.go`'s command line):

```js
const buildOptions = {
  entryPoints: [AGENT_ENTRY],
  outfile: AGENT_OUT,
  bundle: true,
  platform: 'node',
  target: 'node22',
  format: 'cjs',
  external: ['node-pty', 'better-sqlite3', 'keytar', '@parcel/watcher', 'electron'],
  sourcemap: false,
  minify: false,
  define: {
    'process.env.NODE_ENV': '"production"',
    __AGENT_VERSION__: JSON.stringify(AGENT_VERSION)
  },
  logLevel: 'info'
}
```

`agent/src/relay/agent-entry.ts` — add a top-level export (before `main()`'s
self-invocation at the bottom; CJS output turns this into
`module.exports.AGENT_VERSION`):

```ts
// AGENT_VERSION is injected at build time by build.mjs's esbuild `define`
// (__AGENT_VERSION__) — infra-fleet-service's sshrelay.remoteVersionAndPresence
// reads this via `require('./agent.js').AGENT_VERSION` to skip a redundant
// redeploy when the remote bundle is already current (BR-SSH-07).
declare const __AGENT_VERSION__: string
export const AGENT_VERSION: string =
  typeof __AGENT_VERSION__ !== 'undefined' ? __AGENT_VERSION__ : '0.0.0-dev'
```

Add the same `declare const __AGENT_VERSION__: string` to whichever `.d.ts`
ambient-globals file this repo already uses for other esbuild `define`
globals (check `agent/src/*.d.ts` / `agent/tsconfig.json`'s `types` — follow
the existing pattern rather than inventing a new ambient-declarations file).

## Verify

```bash
cd /opt/repos/orca/agent
pnpm run build
node -e "console.log(require('./out/agent.js').AGENT_VERSION)"
```

Expected: prints the `AGENT_VERSION` const from `build.mjs` (e.g. `2.1.0`),
not `undefined`; `pnpm tsc --noEmit` (or the project's existing typecheck
script) reports no error for the new `declare const`.
