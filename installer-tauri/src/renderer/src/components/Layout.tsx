import React, { ReactNode } from 'react';
import { windowApi } from '../../../lib/api';

interface LayoutProps {
  children: ReactNode;
  isLauncherMode?: boolean;
  currentStep?: number;
  stepLabels?: string[];
  hideSteps?: boolean;
  title?: string;
}

const Layout: React.FC<LayoutProps> = ({
  children,
  isLauncherMode,
  currentStep = 1,
  stepLabels = [],
  hideSteps = false,
  title = 'Colink'
}) => {
  const handleMinimize = async () => {
    try {
      await windowApi.minimize();
    } catch (e) {
      console.error('Failed to minimize:', e);
    }
  };

  const handleMaximize = async () => {
    try {
      await windowApi.maximize();
    } catch (e) {
      console.error('Failed to maximize:', e);
    }
  };

  const handleClose = async () => {
    try {
      await windowApi.close();
    } catch (e) {
      console.error('Failed to close:', e);
    }
  };

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* 标题栏 */}
      <div className="title-bar" data-tauri-drag-region>
        <h1 data-tauri-drag-region>{isLauncherMode ? 'Colink' : title}</h1>
        <div className="title-bar-controls">
          <button className="title-bar-btn minimize" onClick={handleMinimize} />
          <button className="title-bar-btn maximize" onClick={handleMaximize} />
          <button className="title-bar-btn close" onClick={handleClose} />
        </div>
      </div>

      {/* 主内容区 */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* 步骤导航 */}
        {!hideSteps && (
          <nav className="step-nav">
            {stepLabels.map((label, index) => {
              const stepNum = index + 1;
              const isActive = stepNum === currentStep;
              const isCompleted = stepNum < currentStep;

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
              );
            })}
          </nav>
        )}

        {/* 内容区域 */}
        <main className="content-area" style={hideSteps ? { width: '100%' } : undefined}>
          {children}
        </main>
      </div>
    </div>
  );
};

export default Layout;