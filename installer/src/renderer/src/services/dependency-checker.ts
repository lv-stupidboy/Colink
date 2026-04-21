import { Dependency } from '../types'

// 只检测智能体 CLI，不检测 Node.js 和 Git
const DEPENDENCIES_CONFIG = [
  { name: 'Claude CLI', key: 'claude', required: false, command: 'claude --version' },
  { name: 'OpenCode', key: 'opencode', required: false, command: 'opencode --version' },
]

export async function checkAllDependencies(): Promise<Dependency[]> {
  const results: Dependency[] = []

  for (const dep of DEPENDENCIES_CONFIG) {
    try {
      const result = await window.electronAPI.checkDependency(dep.key)
      results.push({
        name: dep.name,
        key: dep.key,
        required: dep.required,
        installed: result.installed,
        version: result.version,
      })
    } catch {
      results.push({
        name: dep.name,
        key: dep.key,
        required: dep.required,
        installed: false,
      })
    }
  }

  return results
}