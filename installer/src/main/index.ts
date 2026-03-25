import { app, BrowserWindow, ipcMain } from 'electron'
import { join } from 'path'

// 判断是否为开发模式
const isDev = !app.isPackaged

let mainWindow: BrowserWindow | null = null

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 900,
    height: 600,
    minWidth: 800,
    minHeight: 500,
    frame: false,  // 无边框窗口
    resizable: true,
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  // 开发模式加载本地服务器，生产模式加载打包文件
  if (isDev) {
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }
}

app.whenReady().then(() => {
  createWindow()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow()
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

// IPC 处理：窗口控制
ipcMain.on('window-minimize', () => mainWindow?.minimize())
ipcMain.on('window-close', () => mainWindow?.close())

// IPC 处理：获取应用路径
ipcMain.handle('get-app-path', () => app.getAppPath())

// IPC 处理：获取资源路径
ipcMain.handle('get-resource-path', () => {
  return isDev
    ? join(__dirname, '../../resources')
    : process.resourcesPath
})