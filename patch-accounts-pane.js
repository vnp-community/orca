const fs = require('fs');
const path = '/Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/AccountsPane.tsx';
let content = fs.readFileSync(path, 'utf8');

// Replace activeRuntimeEnvironmentId definition
content = content.replace(
  "const activeRuntimeEnvironmentId = settings.activeRuntimeEnvironmentId?.trim() || null",
  `const rawActiveRuntimeEnvironmentId = settings.activeRuntimeEnvironmentId?.trim() || null\n  const isRemoteOrcaServer = !!(rawActiveRuntimeEnvironmentId && runtimeEnvironments.some(e => e.id === rawActiveRuntimeEnvironmentId))\n  const activeRuntimeEnvironmentId = isRemoteOrcaServer ? rawActiveRuntimeEnvironmentId : null`
);

// We also need to replace the `settings` passed to API calls with a modified one that uses activeRuntimeEnvironmentId.
// We can just define `effectiveSettings` right after.
content = content.replace(
  "const accountRuntimeSentenceLabel =",
  `const effectiveSettings = { ...settings, activeRuntimeEnvironmentId }\n  const accountRuntimeSentenceLabel =`
);

// Now we need to replace all `selectClaudeProviderAccount(settings,` with `selectClaudeProviderAccount(effectiveSettings,`
content = content.replace(/selectClaudeProviderAccount\(settings,/g, 'selectClaudeProviderAccount(effectiveSettings,');
content = content.replace(/selectCodexProviderAccount\(settings,/g, 'selectCodexProviderAccount(effectiveSettings,');
content = content.replace(/removeClaudeProviderAccount\(settings,/g, 'removeClaudeProviderAccount(effectiveSettings,');
content = content.replace(/removeCodexProviderAccount\(settings,/g, 'removeCodexProviderAccount(effectiveSettings,');
content = content.replace(/hasRemoteProviderAccountOwner\(settings\)/g, 'hasRemoteProviderAccountOwner(effectiveSettings)');

fs.writeFileSync(path, content);
console.log("Updated AccountsPane.tsx");
