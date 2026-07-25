// Why: shared interface decoupling ConnectionStatusProvider and web-preload-api
// from the concrete WebSocketRpcClient transport implementation.
export interface IRpcClient {
  connect(): Promise<void>
  disconnect(): void
  isConnected(): boolean
  invoke(channel: string, ...args: unknown[]): Promise<unknown>
  send(channel: string, data?: unknown): void
  on(channel: string, handler: (...args: unknown[]) => void): () => void
  off(channel: string, handler: (...args: unknown[]) => void): void
  once(channel: string, handler: (...args: unknown[]) => void): void
}
