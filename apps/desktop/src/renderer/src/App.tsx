import { useEffect, useState } from "react";
import { ConfigProvider, Spin } from "antd";
import zhCN from "antd/locale/zh_CN";
import type { DaemonStatus } from "../../shared/daemon-types";
import { getApiBaseUrl } from "./platform/api-bridge";

function App() {
  const [status, setStatus] = useState<DaemonStatus>({ state: "installing_cli" });
  const [ready, setReady] = useState(false);

  // Safely get mode with fallback to "local"
  const mode = window.desktopAPI?.appInfo?.mode ?? "local";

  useEffect(() => {
    // Check if preload API is available
    if (!window.daemonAPI) {
      console.error("[App] daemonAPI not available - preload script may not be loaded");
      setReady(true); // Fallback: allow iframe to load anyway
      return;
    }

    // Subscribe to daemon status
    const unsubscribe = window.daemonAPI.onStatusChange(setStatus);

    // Initial status fetch
    window.daemonAPI.getStatus().then(setStatus).catch((err) => {
      console.error("[App] getStatus failed:", err);
      setStatus({ state: "stopped" });
    });

    // Auto-start daemon in local mode
    if (mode === "local") {
      window.daemonAPI.autoStart().then(() => {
        // Wait for daemon to be running
        const checkReady = () => {
          window.daemonAPI.getStatus().then((s) => {
            console.log("[App] daemon status:", s.state);
            if (s.state === "running" || s.state === "remote") {
              setReady(true);
            } else if (s.state === "stopped") {
              // Retry start
              window.daemonAPI.start().then(() => {
                setTimeout(checkReady, 2000);
              }).catch((err) => {
                console.error("[App] start failed:", err);
                // Fallback: after 3 failed attempts, show iframe anyway
                setReady(true);
              });
            } else {
              setTimeout(checkReady, 1000);
            }
          }).catch((err) => {
            console.error("[App] getStatus in checkReady failed:", err);
            setTimeout(checkReady, 1000);
          });
        };
        checkReady();
      }).catch((err) => {
        console.error("[App] autoStart failed:", err);
        // Fallback: try manual start
        window.daemonAPI.start().catch(() => setReady(true));
      });
    } else {
      // Remote mode: ready immediately
      setReady(true);
    }

    return unsubscribe;
  }, [mode]);

  // Determine API URL based on mode
  const apiUrl = getApiBaseUrl(mode, status.serverUrl);

  if (!ready) {
    const tip = status.state === "installing_cli"
      ? "初始化中..."
      : status.state === "cli_not_found"
        ? "服务未安装"
        : status.state === "starting"
          ? "启动服务中..."
          : "连接服务中...";
    return (
      <div style={{ display: "flex", height: "100vh", alignItems: "center", justifyContent: "center" }}>
        <Spin size="large" tip={tip} />
      </div>
    );
  }

  return (
    <div style={{ width: "100%", height: "100vh", overflow: "hidden" }}>
      <ConfigProvider locale={zhCN}>
        <iframe
          src={apiUrl}
          style={{ width: "100%", height: "100%", border: "none" }}
          title="Colink"
        />
      </ConfigProvider>
    </div>
  );
}

export default App;