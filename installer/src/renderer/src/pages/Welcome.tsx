import { InstallConfig, InstalledVersion } from '../types'

interface WelcomeProps {
  config: InstallConfig
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

// ISDP Logo SVG - 熄灯工厂主题
const ISDPLogo = () => (
  <svg width="80" height="80" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#374151' }} />
        <stop offset="100%" style={{ stopColor: '#1f2937' }} />
      </linearGradient>
      <linearGradient id="bulbGrad" x1="0%" y1="0%" x2="0%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#fbbf24' }} />
        <stop offset="100%" style={{ stopColor: '#f59e0b' }} />
      </linearGradient>
    </defs>
    <rect x="2" y="2" width="28" height="28" rx="6" fill="url(#bgGrad)" />
    <ellipse cx="16" cy="12" rx="6" ry="7" fill="url(#bulbGrad)" />
    <rect x="13" y="18" width="6" height="2" rx="0.5" fill="#9ca3af" />
    <rect x="13.5" y="20.5" width="5" height="1.5" rx="0.5" fill="#6b7280" />
    <rect x="14" y="22.5" width="4" height="1.5" rx="0.5" fill="#4b5563" />
    <line x1="13" y1="19" x2="19" y2="19" stroke="#6b7280" strokeWidth="0.5" />
    <line x1="13.5" y1="21.25" x2="18.5" y2="21.25" stroke="#4b5563" strokeWidth="0.5" />
    <path d="M14 14 Q16 11 18 14" stroke="#fcd34d" strokeWidth="1.5" fill="none" strokeLinecap="round" />
  </svg>
)

export default function Welcome({ isUpgrade, installedVersion }: WelcomeProps) {
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

      {isUpgrade ? (
        <>
          <h2 style={{ fontSize: 24, marginBottom: 12, color: '#333' }}>
            检测到已安装的 Lights-Out
          </h2>
          <p style={{ color: '#666', lineHeight: 2, marginBottom: 20 }}>
            安装位置：<code style={{ background: '#f5f5f5', padding: '2px 8px', borderRadius: 4 }}>{installedVersion?.installDir}</code>
          </p>
          <p style={{ color: '#666', lineHeight: 2, marginBottom: 40 }}>
            本向导将帮助您完成：<br />
            · 检测并安装运行依赖<br />
            · 升级 Lights-Out 核心程序<br />
            · 保留现有配置和数据
          </p>
        </>
      ) : (
        <>
          <h2 style={{ fontSize: 24, marginBottom: 12, color: '#333' }}>
            欢迎使用 Lights-Out 熄灯工厂
          </h2>
          <p style={{ color: '#666', lineHeight: 2, marginBottom: 40 }}>
            本向导将帮助您完成：<br />
            · 安装 Lights-Out 核心程序<br />
            · 检测并安装运行依赖<br />
            · 配置数据库连接
          </p>
        </>
      )}
    </div>
  )
}