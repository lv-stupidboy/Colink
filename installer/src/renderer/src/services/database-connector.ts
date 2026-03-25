import { DatabaseConfig } from '../types'

export interface TestResult {
  success: boolean
  error?: string
}

export async function testConnection(config: DatabaseConfig): Promise<TestResult> {
  try {
    const result = await window.electronAPI.testDatabaseConnection(config)
    return result
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '未知错误',
    }
  }
}