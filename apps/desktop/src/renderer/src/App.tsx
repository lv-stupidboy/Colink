import { useEffect, useState, useRef } from "react";
import { ConfigProvider, Spin } from "antd";
import zhCN from "antd/locale/zh_CN";
import type { DaemonStatus } from "../../shared/daemon-types";
import { getApiBaseUrl } from "./platform/api-bridge";

function App() {
  const [status, setStatus] = useState<DaemonStatus>({ state: "installing_cli" });
  const [ready, setReady] = useState(false);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  // Safely get mode with fallback to "local"
  const mode = window.desktopAPI?.appInfo?.mode ?? "local";

  useEffect(() => {
    console.log("[App] Starting with mode:", mode);

    // Global timeout: set ready after 10 seconds no matter what
    const globalTimeout = setTimeout(() => {
      console.log("[App] global timeout reached, setting ready");
      setReady(true);
    }, 10000);

    // Check if preload API is available
    if (!window.daemonAPI) {
      console.error("[App] daemonAPI not available - preload script may not be loaded");
      clearTimeout(globalTimeout);
      setReady(true);
      return;
    }

    // Subscribe to daemon status changes
    const unsubscribe = window.daemonAPI.onStatusChange((s) => {
      console.log("[App] status changed:", s.state, "serverUrl:", s.serverUrl);
      setStatus(s);
      if (s.state === "running" || s.state === "remote") {
        clearTimeout(globalTimeout);
        setReady(true);
      }
    });

    // Initial status fetch
    window.daemonAPI.getStatus().then((s) => {
      console.log("[App] initial status:", s.state, "serverUrl:", s.serverUrl);
      setStatus(s);
      if (s.state === "running" || s.state === "remote") {
        clearTimeout(globalTimeout);
        setReady(true);
      }
    }).catch((err) => {
      console.error("[App] getStatus failed:", err);
    });

    // Auto-start daemon in local mode
    if (mode === "local") {
      window.daemonAPI.autoStart().then(() => {
        console.log("[App] autoStart completed");
      }).catch((err) => {
        console.error("[App] autoStart failed:", err);
        // Try manual start
        window.daemonAPI.start().then(() => {
          console.log("[App] manual start completed");
        }).catch((err2) => {
          console.error("[App] manual start failed:", err2);
        });
      });
    } else {
      // Remote mode: ready immediately
      clearTimeout(globalTimeout);
      setReady(true);
    }

    return () => {
      clearTimeout(globalTimeout);
      unsubscribe();
    };
  }, [mode]);

  // Determine API URL based on mode
  const apiUrl = getApiBaseUrl(mode, status.serverUrl);

  // Effect to set iframe src when ready
  useEffect(() => {
    if (ready && iframeRef.current && apiUrl) {
      console.log("[App] Setting iframe src to:", apiUrl);
      iframeRef.current.src = apiUrl;
    }
  }, [ready, apiUrl]);

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

  console.log("[App] rendering iframe, apiUrl:", apiUrl, "ready:", ready);

  return (
    <div style={{ width: "100%", height: "100vh", overflow: "hidden", backgroundColor: "#f5f5f5" }}>
      <ConfigProvider locale={zhCN}>
        <iframe
          ref={iframeRef}
          src={apiUrl}
          style={{ width: "100%", height: "100%", border: "none" }}
          title="Colink"
          allow="clipboard-read; clipboard-write"
        />
      </ConfigProvider>
    </div>
  );
}

export default App;