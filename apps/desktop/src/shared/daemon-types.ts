export interface DaemonStatus {
  state: "running" | "stopped" | "starting" | "stopping" | "installing_cli" | "cli_not_found" | "remote";
  version?: string;
  gitCommit?: string;
  buildTime?: string;
  serverUrl?: string;
}

export interface DaemonPrefs {
  autoStart: boolean;
  autoStop: boolean;
}