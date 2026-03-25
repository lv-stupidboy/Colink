import { ReactNode } from 'react'

interface LayoutProps {
  currentStep: number
  stepLabels: string[]
  children: ReactNode
}

export default function Layout({ currentStep, stepLabels, children }: LayoutProps) {
  const handleMinimize = () => {
    window.electronAPI?.minimizeWindow()
  }

  const handleClose = () => {
    window.electronAPI?.closeWindow()
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* 标题栏 */}
      <div className="title-bar">
        <h1>ISDP 安装向导</h1>
        <div className="title-bar-controls">
          <button className="title-bar-btn minimize" onClick={handleMinimize} />
          <button className="title-bar-btn close" onClick={handleClose} />
        </div>
      </div>

      {/* 主内容区 */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* 步骤导航 */}
        <nav className="step-nav">
          {stepLabels.map((label, index) => {
            const stepNum = index + 1
            const isActive = stepNum === currentStep
            const isCompleted = stepNum < currentStep

            return (
              <div
                key={stepNum}
                className={`step-item ${isActive ? 'active' : ''} ${isCompleted ? 'completed' : ''}`}
              >
                <div className="step-number">
                  {isCompleted ? '✓' : stepNum}
                </div>
                <span className="step-label">{label}</span>
              </div>
            )
          })}
        </nav>

        {/* 内容区域 */}
        <main className="content-area">
          {children}
        </main>
      </div>
    </div>
  )
}