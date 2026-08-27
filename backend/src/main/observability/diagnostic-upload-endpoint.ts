// Adapted from desktop/src/main/observability/diagnostic-upload-endpoint.ts
// (diagnostics.* bundle RPC port). Desktop resolves the token endpoint from a
// CI-substituted build-time global (ORCA_DIAGNOSTICS_TOKEN_URL/
// ORCA_BUILD_IDENTITY, defined via electron-vite's `define` — see
// desktop/electron.vite.config.ts and src/types/build-constants.d.ts) so
// official releases can't be redirected by user env. Backend has no
// equivalent packaged-release build identity or `define` step, so this
// always resolves from process.env — same fallback desktop uses for its own
// unofficial/dev builds — and always reports the 'dev' channel.

export function resolveDiagnosticTokenEndpoint(): string | null {
  const fromEnv = process.env.ORCA_DIAGNOSTICS_TOKEN_URL
  return fromEnv && fromEnv.length > 0 ? fromEnv : null
}

export function resolveDiagnosticOrcaChannel(): 'stable' | 'rc' | 'dev' {
  return 'dev'
}
