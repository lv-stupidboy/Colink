import { InstallConfig } from '../types'

interface WelcomeProps {
  config: InstallConfig
}

// ISDP Logo SVG
const ISDPLogo = () => (
  <svg width="80" height="80" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#10b981' }} />
        <stop offset="100%" style={{ stopColor: '#059669' }} />
      </linearGradient>
      <linearGradient id="flameGrad" x1="0%" y1="0%" x2="0%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#fbbf24' }} />
        <stop offset="50%" style={{ stopColor: '#f59e0b' }} />
        <stop offset="100%" style={{ stopColor: '#ef4444' }} />
      </linearGradient>
    </defs>
    <rect x="2" y="2" width="28" height="28" rx="6" fill="url(#bgGrad)" />
    <path d="M16 5C16 5 11 9 11 15C11 17 11.5 19 12 20L13 22H19L20 20C20.5 19 21 17 21 15C21 9 16 5 16 5Z" fill="white" />
    <circle cx="16" cy="12" r="2" fill="#10b981" />
    <path d="M11 18L9 22L11 23L12.5 20.5Z" fill="#e5e7eb" />
    <path d="M21 18L23 22L21 23L19.5 20.5Z" fill="#e5e7eb" />
    <path d="M13.5 22L14.5 27L16 25L17.5 27L18.5 22Z" fill="url(#flameGrad)" />
  </svg>
)

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
      <div style={{ marginBottom: 20 }}><ISDPLogo /></div>
      <h2 style={{ fontSize: 24, marginBottom: 12, color: '#333' }}>
        欢迎使用 ISDP 智能软件开发平台
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