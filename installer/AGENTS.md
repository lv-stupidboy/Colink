# ISDP Installer

Electron-based dual-app architecture. **Setup** (install/upgrade/uninstall) and **Launcher** (runtime service control) share renderer UI but have separate main processes and packaging configs.

## Architecture

```
src/
├── main/
│   ├── index.ts              # Setup main process (505 lines). IPC handlers for install flow.
│   ├── launcher-entry.ts     # Launcher main process. IPC for service control + logs.
│   ├── installer.ts          # Installation pipeline (555 lines). Copy, config, shortcuts, registry.
│   ├── service-manager.ts    # Spawns/stops isdp-server.exe via child_process.
│   └── shared/
│       ├── app-mode.ts       # Detects setup vs launcher via exe name + resourcesPath
│       ├── install-utils.ts  # Windows registry checks for installed version
│       └── window-utils.ts   # Close confirmation, ensures service stopped before exit
├── preload/
│   └── index.ts              # IPC bridge: exposes ipcRenderer APIs to renderer
└── renderer/
    └── src/                  # Shared React UI (both modes load same renderer)
        ├── App.tsx           # Router, mode detection via 'is-launcher-mode' IPC
        ├── pages/            # 9 page components for install/launcher flows
        ├── services/         # Frontend service layer
        └── styles/           # CSS
```

## Mode Detection

```typescript
// Setup main (index.ts):
ipcMain.handle('is-launcher-mode', () => false)

// Launcher main (launcher-entry.ts):
ipcMain.handle('is-launcher-mode', () => true)
```

Renderer calls `is-launcher-mode` on startup to determine which UI flow to show.

## Key IPC Channels

| Channel | Mode | Purpose |
|---------|------|---------|
| `is-launcher-mode` | Both | Mode detection |
| `get-startup-action` | Setup | Returns 'install' or 'upgrade' |
| `start-installation` | Setup | Triggers full install pipeline |
| `generate-config` | Setup | Generates config.yaml |
| `test-database-connection` | Setup | Validates MySQL connectivity |
| `check-dependency` | Setup | Checks Node.js, Git, Claude CLI |
| `start-service` / `stop-service` | Both | Controls isdp-server.exe |
| `get-service-status` | Both | Reports running/stopped |
| `open-console` | Launcher | Opens browser to backend URL |
| `open-logs` / `open-config` | Launcher | Opens log/config directories |
| `uninstall` | Setup | Removes app (optionally preserves `data/`) |

## Service Management

`ServiceManager` spawns `isdp-server.exe` as child process (not a Windows service). Config path passed as argument. `killAllProcesses()` uses `taskkill` for clean shutdown before install/exit.

## Packaging

Two separate electron-builder configs:

| Config | Product | Entry | Output |
|--------|---------|-------|--------|
| `electron-builder.yml` | ISDP Setup | `out/main/index.js` | `release/` — bundles server, web, launcher |
| `electron-builder.launcher.yml` | ISDP | `out/main/launcher-entry.js` | `release/launcher/` — standalone launcher |

Setup's `extraResources` bundles: `runtime/isdp-server.exe`, `runtime/web/`, `release/launcher/win-unpacked/`.

**Build order**: `npm run build` → `npm run package:launcher` → `npm run package:setup` → `node scripts/create-zip.js`

## Where to Change

| Task | File |
|------|------|
| Add install step | `installer.ts` — `runInstallation()` |
| Add IPC channel | `index.ts` or `launcher-entry.ts` + `preload/index.ts` |
| Modify service control | `service-manager.ts` |
| Change mode detection | `shared/app-mode.ts` |
| Add installer page | `renderer/src/pages/` + update `App.tsx` routing |

## Notes

- **Windows-only**: Packaging targets Windows (`--win`). Cross-platform not implemented.
- **Shared renderer**: Both Setup and Launcher load identical `renderer/index.html`.
- **Registry**: Install location stored in Windows registry. Checked via `getInstalledVersion()`.
- **Shortcuts**: Created via VBScript. Desktop + Start Menu.
