import { ReactNode } from 'react'

interface LayoutProps {
  currentStep?: number
  stepLabels?: string[]
  children: ReactNode
  hideSteps?: boolean
  title?: string
}

export default function Layout({ currentStep = 1, stepLabels = [], children, hideSteps = false, title = 'ISDP' }: LayoutProps) {
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
        <h1>{title}</h1>
        <div className="title-bar-controls">
          <button className="title-bar-btn minimize" onClick={handleMinimize} />
          <button className="title-bar-btn close" onClick={handleClose} />
        </div>
      </div>

      {/* 主内容区 */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* 步骤导航 */}
        {!hideSteps && (
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
        )}

        {/* 内容区域 */}
        <main className="content-area" style={hideSteps ? { width: '100%' } : undefined}>
          {children}
        </main>
      </div>
    </div>
  )
}