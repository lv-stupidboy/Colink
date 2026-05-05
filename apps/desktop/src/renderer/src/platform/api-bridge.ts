/**
 * Get API base URL based on mode and daemon status.
 * Local mode: use localhost with daemon port
 * Remote mode: use configured remote URL
 */
const DEFAULT_PORT = 26305;

export function getApiBaseUrl(mode: "local" | "remote", daemonUrl?: string): string {
  if (mode === "remote") {
    // Remote mode: URL is configured at build time or runtime
    return import.meta.env.VITE_API_URL || `http://localhost:${DEFAULT_PORT}`;
  }

  // Local mode: use daemon URL or fallback to default
  return daemonUrl || `http://localhost:${DEFAULT_PORT}`;
}

/**
 * Get WebSocket URL based on mode and daemon status.
 */
export function getWsUrl(mode: "local" | "remote", daemonUrl?: string): string {
  if (mode === "remote") {
    const apiUrl = import.meta.env.VITE_API_URL || `http://localhost:${DEFAULT_PORT}`;
    // Convert http/https to ws/wss
    return apiUrl.replace(/^http/, "ws") + "/ws";
  }

  const baseUrl = daemonUrl || `http://localhost:${DEFAULT_PORT}`;
  return baseUrl.replace(/^http/, "ws") + "/ws";
}