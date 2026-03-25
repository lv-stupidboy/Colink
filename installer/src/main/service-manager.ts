import { ChildProcess, spawn } from 'child_process'
import { join } from 'path'

export class ServiceManager {
  private process: ChildProcess | null = null
  private installDir: string

  constructor(installDir: string) {
    this.installDir = installDir
  }

  async start(): Promise<void> {
    if (this.process) return

    this.process = spawn(join(this.installDir, 'isdp-server.exe'), [], {
      cwd: this.installDir,
      detached: false,
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
    })

    this.process.stdout?.on('data', (data) => {
      console.log(`[ISDP] ${data}`)
    })

    this.process.stderr?.on('data', (data) => {
      console.error(`[ISDP Error] ${data}`)
    })

    this.process.on('close', () => {
      this.process = null
    })

    // 等待服务就绪
    await this.waitForReady()
  }

  async stop(): Promise<void> {
    if (this.process) {
      this.process.kill('SIGTERM')
      this.process = null
    }
  }

  async restart(): Promise<void> {
    await this.stop()
    await this.start()
  }

  getStatus(): 'running' | 'stopped' {
    return this.process ? 'running' : 'stopped'
  }

  private async waitForReady(): Promise<void> {
    // 简单的等待策略，实际可以检查 HTTP 端点
    return new Promise(resolve => setTimeout(resolve, 2000))
  }
}