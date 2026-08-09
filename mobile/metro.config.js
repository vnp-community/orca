const path = require('node:path')
const { getDefaultConfig } = require('expo/metro-config')

const projectRoot = __dirname
// Was '../src/shared' (repo-root shared source); that tree no longer exists
// after the monorepo split — mobile now keeps its own vendored copy instead.
const sharedRoot = path.resolve(projectRoot, 'src', 'vendor-shared', 'shared')

const config = getDefaultConfig(projectRoot)

// Why: mobile source-control prompts use the same pure builders as desktop.
// Metro only watches mobile/ by default, so make the vendored shared modules visible.
config.watchFolders = Array.from(new Set([...(config.watchFolders ?? []), sharedRoot]))

module.exports = config
