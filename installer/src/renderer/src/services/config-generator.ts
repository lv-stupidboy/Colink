import YAML from 'yaml'
import { InstallConfig } from '../types'

export async function generateConfigFile(config: InstallConfig, targetPath: string): Promise<{ success: boolean; error?: string }> {
  try {
    const yamlContent = YAML.stringify({
      server: {
        port: 8080,
        mode: 'release',
      },
      database: {
        type: 'mysql',
        mysql: {
          host: config.database.host,
          port: config.database.port,
          database: config.database.database,
          username: config.database.username,
          password: config.database.password,
          charset: 'utf8mb4',
        },
      },
      claude: {
        path: 'claude',
        default_model: 'claude-sonnet-4-6',
        timeout: '30m',
      },
      logging: {
        level: 'info',
        format: 'json',
      },
    })

    const result = await window.electronAPI.generateConfig({ path: targetPath, content: yamlContent })
    return result
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '生成配置失败',
    }
  }
}