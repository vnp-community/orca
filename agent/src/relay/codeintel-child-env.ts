export function buildCodeIntelChildEnv(config: { toolEnv: NodeJS.ProcessEnv }): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {}
  
  // The child codeintel processes do not need any secrets.
  // This is the only place to change if the contractor requires `toolEnv` literally.
  const secretPattern = /(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i
  const forbiddenPrefixes = ['AWS_', 'GOOGLE_', 'ORCA_']

  for (const [key, value] of Object.entries(config.toolEnv)) {
    if (key === 'PATH' || key === 'HOME' || key === 'LANG') {
      env[key] = value
      continue
    }
    
    if (key === 'SSH_AUTH_SOCK') continue
    if (secretPattern.test(key)) continue
    if (forbiddenPrefixes.some(prefix => key.startsWith(prefix))) continue
    
    env[key] = value
  }

  env['NO_COLOR'] = '1'

  return env
}
