import { useActiveDevServer } from '../store/slices/dev-servers-selectors'

/** Trả về platform của active dev server, hoặc null nếu chưa kết nối */
export function useActiveDevServerPlatform(): NodeJS.Platform | null {
  const ds = useActiveDevServer()
  return ds?.platform ?? null
}

/** true khi active dev server là Windows */
export function useShowWindowsTerminalStep(): boolean {
  return useActiveDevServerPlatform() === 'win32'
}

/** true khi active dev server là macOS */
export function useShowGhosttyImport(): boolean {
  return useActiveDevServerPlatform() === 'darwin'
}

/** true khi active dev server là Linux */
export function useIsLinuxDevServer(): boolean {
  return useActiveDevServerPlatform() === 'linux'
}
