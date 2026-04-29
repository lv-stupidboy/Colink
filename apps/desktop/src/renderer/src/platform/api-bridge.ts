/**
 * Get API base URL based on mode and daemon status.
 * Local mode: use localhost with daemon port
 * Remote mode: use configured remote URL
 */
export function getApiBaseUrl(mode: "local" | "remote", daemonUrl?: string): string {
  if (mode === "remote") {
    // Remote mode: URL is configured at build time or runtime
    return import.meta.env.VITE_API_URL || "http://localhost:26305";
  }

  // Local mode: use daemon URL or fallback to default
  return daemonUrl || "http://localhost:26305";
}

/**
 * Get WebSocket URL based on mode and daemon status.
 */
export function getWsUrl(mode: "local" | "remote", daemonUrl?: string): string {
  if (mode === "remote") {
    const apiUrl = import.meta.env.VITE_API_URL || "http://localhost:26305";
    // Convert http/https to ws/wss
    return apiUrl.replace(/^http/, "ws") + "/ws";
  }

  const baseUrl = daemonUrl || "http://localhost:26305";
  return baseUrl.replace(/^http/, "ws") + "/ws";
}