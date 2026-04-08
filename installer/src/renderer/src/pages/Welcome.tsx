import { InstallConfig, InstalledVersion } from '../types'

interface WelcomeProps {
  config: InstallConfig
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

// Colink Logo SVG - 六边形网络设计（缩小版）
const ColinkLogo = () => (
  <svg width="80" height="80" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="colinkGrad" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#10b981' }} />
        <stop offset="100%" style={{ stopColor: '#3b82f6' }} />
      </linearGradient>
    </defs>
    {/* 背景 */}
    <rect x="2" y="2" width="28" height="28" rx="6" fill="#0f172a" />
    {/* 六边形轮廓线 - 缩小尺寸 */}
    <polygon
      points="16,6 24,10.5 24,21.5 16,26 8,21.5 8,10.5"
      fill="none"
      stroke="#10b981"
      strokeWidth="1.2"
      strokeOpacity="0.35"
      strokeLinejoin="round"
    />
    {/* 从外环到中心的连接线 */}
    <g stroke="#10b981" strokeWidth="0.8" strokeOpacity="0.35">
      <line x1="16" y1="6" x2="16" y2="16" />
      <line x1="24" y1="10.5" x2="16" y2="16" />
      <line x1="24" y1="21.5" x2="16" y2="16" />
      <line x1="16" y1="26" x2="16" y2="16" />
      <line x1="8" y1="21.5" x2="16" y2="16" />
      <line x1="8" y1="10.5" x2="16" y2="16" />
    </g>
    {/* 外环节点 (6个) */}
    <circle cx="16" cy="6" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="24" cy="10.5" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="24" cy="21.5" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="16" cy="26" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="8" cy="21.5" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="8" cy="10.5" r="1.8" fill="url(#colinkGrad)" />
    {/* 中心节点 */}
    <circle cx="16" cy="16" r="3" fill="url(#colinkGrad)" />
    {/* 节点高光 */}
    <circle cx="16" cy="6" r="0.7" fill="white" opacity="0.3" />
    <circle cx="24" cy="10.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="24" cy="21.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="16" cy="26" r="0.7" fill="white" opacity="0.3" />
    <circle cx="8" cy="21.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="8" cy="10.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="16" cy="16" r="1.2" fill="white" opacity="0.4" />
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
      <div style={{ marginBottom: 20 }}><ColinkLogo /></div>

      {isUpgrade ? (
        <>
          <h2 style={{ fontSize: 24, marginBottom: 12, color: '#333' }}>
            检测到已安装的 Colink
          </h2>
          <p style={{ color: '#666', lineHeight: 2, marginBottom: 20 }}>
            安装位置：<code style={{ background: '#f5f5f5', padding: '2px 8px', borderRadius: 4 }}>{installedVersion?.installDir}</code>
          </p>
          <p style={{ color: '#666', lineHeight: 2, marginBottom: 40 }}>
            本向导将帮助您完成：<br />
            · 检测并安装运行依赖<br />
            · 升级 Colink 核心程序<br />
            · 保留现有配置和数据
          </p>
        </>
      ) : (
        <>
          <h2 style={{ fontSize: 24, marginBottom: 12, color: '#333' }}>
            欢迎使用 Colink 多智能体协作平台
          </h2>
          <p style={{ color: '#666', lineHeight: 2, marginBottom: 40 }}>
            本向导将帮助您完成：<br />
            · 安装 Colink 核心程序<br />
            · 检测并安装运行依赖<br />
            · 配置数据库连接
          </p>
        </>
      )}
    </div>
  )
}