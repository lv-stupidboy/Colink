import { BrowserWindow, Menu, WebContents } from "electron";

export function installContextMenu(webContents: WebContents): void {
  webContents.on("context-menu", (_event, params) => {
    const menu = Menu.buildFromTemplate([
      { label: "Paste", role: "paste", enabled: params.editFlags.canPaste },
      { label: "Copy", role: "copy", enabled: params.editFlags.canCopy },
      { label: "Cut", role: "cut", enabled: params.editFlags.canCut },
      { type: "separator" },
      { label: "Select All", role: "selectAll" },
    ]);
    const win = BrowserWindow.fromWebContents(webContents);
    if (win) {
      menu.popup({ window: win });
    }
  });
}