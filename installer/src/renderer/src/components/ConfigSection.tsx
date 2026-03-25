import { ReactNode, useState } from 'react'

interface ConfigSectionProps {
  title: string
  defaultCollapsed?: boolean
  children: ReactNode
}

export default function ConfigSection({
  title,
  defaultCollapsed = false,
  children
}: ConfigSectionProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed)

  return (
    <div style={{
      border: '1px solid #e8e8e8',
      borderRadius: 8,
      marginBottom: 16,
      overflow: 'hidden'
    }}>
      <div
        onClick={() => setCollapsed(!collapsed)}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '14px 16px',
          background: '#fafafa',
          cursor: 'pointer',
          fontWeight: 500,
        }}
      >
        <span>{title}</span>
        <span style={{ transform: collapsed ? 'rotate(-90deg)' : 'none', transition: 'transform 0.3s' }}>
          ▼
        </span>
      </div>
      {!collapsed && (
        <div style={{ padding: 20, borderTop: '1px solid #e8e8e8' }}>
          {children}
        </div>
      )}
    </div>
  )
}