import { useEffect, useState } from "react";
import { ConfigProvider, Spin } from "antd";
import zhCN from "antd/locale/zh_CN";
import type { DaemonStatus } from "../../shared/daemon-types";
import { getApiBaseUrl } from "./platform/api-bridge";

function App() {
  const [status, setStatus] = useState<DaemonStatus>({ state: "installing_cli" });
  const [ready, setReady] = useState(false);
  const { mode } = window.desktopAPI.appInfo;

  useEffect(() => {
    // Subscribe to daemon status
    const unsubscribe = window.daemonAPI.onStatusChange(setStatus);

    // Initial status fetch
    window.daemonAPI.getStatus().then(setStatus);

    // Auto-start daemon in local mode
    if (mode === "local") {
      window.daemonAPI.autoStart().then(() => {
        // Wait for daemon to be running
        const checkReady = () => {
          window.daemonAPI.getStatus().then((s) => {
            if (s.state === "running" || s.state === "remote") {
              setReady(true);
            } else if (s.state === "stopped") {
              // Retry start
              window.daemonAPI.start().then(() => {
                setTimeout(checkReady, 2000);
              });
            } else {
              setTimeout(checkReady, 1000);
            }
          });
        };
        checkReady();
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
    return (
      <div style={{ display: "flex", height: "100vh", alignItems: "center", justifyContent: "center" }}>
        <Spin size="large" tip={status.state === "installing_cli" ? "初始化中..." : "启动服务中..."} />
      </div>
    );
  }

  return (
    <ConfigProvider locale={zhCN}>
      <iframe
        src={apiUrl}
        style={{ width: "100%", height: "100%", border: "none" }}
        title="Colink"
      />
    </ConfigProvider>
  );
}

export default App;