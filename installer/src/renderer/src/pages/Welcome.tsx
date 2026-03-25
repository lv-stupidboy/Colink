import { InstallConfig } from '../types'

interface WelcomeProps {
  config: InstallConfig
}

export default function Welcome({}: WelcomeProps) {
  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      textAlign: 'center'
    }}>
      <div style={{ fontSize: 80, marginBottom: 20 }}>🚀</div>
      <h2 style={{ fontSize: 24, marginBottom: 12, color: '#333' }}>
        欢迎使用 ISDP 智能开发平台
      </h2>
      <p style={{ color: '#666', lineHeight: 2, marginBottom: 40 }}>
        本向导将帮助您完成：<br />
        · 安装 ISDP 核心程序<br />
        · 检测并安装运行依赖<br />
        · 配置数据库连接
      </p>
    </div>
  )
}