// src/relay/agent-build-version.ts
// Exposes the build-time AGENT_VERSION injected by esbuild's define option.
// Does NOT import agent-entry.ts (it runs main() on load, causing a circular dep).

declare const __AGENT_VERSION__: string | undefined

export const AGENT_BUILD_VERSION: string =
  typeof __AGENT_VERSION__ !== 'undefined' ? __AGENT_VERSION__ : '0.0.0-dev'
