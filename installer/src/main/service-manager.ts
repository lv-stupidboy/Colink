import { ChildProcess, spawn } from 'child_process'
import { join } from 'path'
import { existsSync } from 'fs'

export class ServiceManager {
  private process: ChildProcess | null = null
  private installDir: string

  constructor(installDir: string) {
    this.installDir = installDir
  }

  async start(): Promise<{ success: boolean; error?: string }> {
    if (this.process) {
      return { success: true }
    }

    const serverPath = join(this.installDir, 'isdp-server.exe')
    const configPath = join(this.installDir, 'data', 'configs', 'config.yaml')

    if (!existsSync(serverPath)) {
      return { success: false, error: `服务程序不存在` }
    }

    if (!existsSync(configPath)) {
      return { success: false, error: `配置文件不存在` }
    }

    return new Promise((resolve) => {
      let resolved = false
      let errorOutput = ''

      try {
        this.process = spawn(serverPath, ['-config', configPath], {
          cwd: this.installDir,
          detached: false,
          stdio: ['ignore', 'pipe', 'pipe'],
          windowsHide: true,
        })

        this.process.stdout?.on('data', (data) => {
          console.log(`[Server] ${data.toString().trim()}`)
        })

        this.process.stderr?.on('data', (data) => {
          const msg = data.toString()
          console.error(`[Server Error] ${msg.trim()}`)
          errorOutput += msg
        })

        this.process.on('error', (err) => {
          console.error('[Service] Process error:', err)
          this.process = null
          if (!resolved) {
            resolved = true
            resolve({ success: false, error: `启动失败: ${err.message}` })
          }
        })

        this.process.on('close', (code) => {
          this.process = null
          if (!resolved) {
            resolved = true
            resolve({ success: false, error: errorOutput || `服务退出，代码: ${code}` })
          }
        })

        // 等待服务启动
        setTimeout(() => {
          if (!resolved) {
            if (this.process && !this.process.killed) {
              resolved = true
              resolve({ success: true })
            } else {
              resolved = true
              resolve({ success: false, error: errorOutput || '服务启动超时' })
            }
          }
        }, 5000)

      } catch (err) {
        console.error('[Service] Start error:', err)
        if (!resolved) {
          resolved = true
          resolve({ success: false, error: err instanceof Error ? err.message : '启动异常' })
        }
      }
    })
  }

  async stop(): Promise<void> {
    if (this.process) {
      this.process.kill('SIGTERM')
      this.process = null
    }
  }

  async restart(): Promise<{ success: boolean; error?: string }> {
    await this.stop()
    return this.start()
  }

  getStatus(): 'running' | 'stopped' {
    return this.process ? 'running' : 'stopped'
  }
}