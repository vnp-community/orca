/**
 * User Process Entry Point
 *
 * ⚠️ This file is the entry point of a forked child process.
 * It MUST NOT be imported by the supervisor process.
 * It is executed via child_process.fork() with ORCA_USER_ID,
 * ORCA_USER_DATA_PATH, and ORCA_SOCKET_PATH set in env.
 *
 * Initialization sequence:
 * 1. Validate required env vars
 * 2. Set platform adapter with per-user userData path
 * 3. Boot OrcaRuntime services on a Unix domain socket
 * 4. Signal supervisor via IPC: process.send({ type: 'ready' })
 * 5. Handle SIGTERM/SIGINT for graceful shutdown
 *
 * @module main/session/user-process-entry
 */

const userId   = process.env['ORCA_USER_ID']
const dataPath = process.env['ORCA_USER_DATA_PATH']
const sockPath = process.env['ORCA_SOCKET_PATH']

// Step 1: Validate required env vars — fail fast
if (!userId || !dataPath || !sockPath) {
  console.error('[UserProcess] ERROR: Missing required env vars:', {
    ORCA_USER_ID:        Boolean(userId),
    ORCA_USER_DATA_PATH: Boolean(dataPath),
    ORCA_SOCKET_PATH:    Boolean(sockPath)
  })
  process.exit(1)
}

console.log(`[UserProcess] Starting: userId=${userId}, sockPath=${sockPath}`)

async function main(): Promise<void> {
  // Step 2: Set up platform adapter with per-user data path
  // Dynamic imports prevent circular dependencies with supervisor code
  const { createNodeAdapter } = await import('../../platform/adapters/node')
  const { setPlatform }       = await import('../../platform/context')

  const adapter = createNodeAdapter({ userDataPath: dataPath! })
  setPlatform(adapter)

  // Step 3: Boot Orca backend services — listen on Unix socket (not TCP)
  const { initializeOrcaServices } = await import('../server-bootstrap')
  const { shutdown, rpcAuthToken } = await initializeOrcaServices({
    platform: adapter,
    // Why: ORCA_SOCKET_PATH is the Unix domain socket this user process must
    // listen on. WsSessionRouter (supervisor) connects to this socket to proxy
    // browser WebSocket traffic. Without it, handleConnection() closes the
    // client connection immediately with 1011 because the socket file is missing.
    socketPath: sockPath ?? undefined,
    // wsPort=0: disable the TCP WebSocket server in user-process mode; all
    // traffic arrives via the Unix socket proxied by WsSessionRouter.
    port: 0,
    isUserProcess: true
  })

  // Step 4: Signal supervisor that we are ready — include rpcAuthToken so
  // SessionManager stores it in UserProcess for WsSessionRouter proxy injection.
  if (process.send) {
    process.send({ type: 'ready', socketPath: sockPath, rpcAuthToken: rpcAuthToken })
  }
  console.log(`[UserProcess] Ready: userId=${userId}`)

  // Step 5: Graceful shutdown handlers
  const handleExit = async (signal: string) => {
    console.log(`[UserProcess] Shutting down (${signal}): userId=${userId}`)
    try {
      await shutdown()
    } catch (err) {
      console.error(`[UserProcess] Shutdown error: userId=${userId}`, err)
    } finally {
      process.exit(0)
    }
  }

  process.on('SIGTERM', () => { void handleExit('SIGTERM') })
  process.on('SIGINT',  () => { void handleExit('SIGINT') })
}

main().catch((err: unknown) => {
  console.error(`[UserProcess] Fatal error: userId=${userId}`, err)
  process.exit(1)
})
