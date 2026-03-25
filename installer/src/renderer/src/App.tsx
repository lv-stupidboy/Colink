import { useState } from 'react'
import { Button, Space } from 'antd'
import { StepId, InstallConfig, Dependency } from './types'
import Layout from './components/Layout'
import Welcome from './pages/Welcome'
import DirectorySelect from './pages/DirectorySelect'
import DependencyCheck from './pages/DependencyCheck'
import ModeSelect from './pages/ModeSelect'
import SystemConfig from './pages/SystemConfig'
import Installing from './pages/Installing'
import Complete from './pages/Complete'

const PAGES = {
  1: Welcome,
  2: DirectorySelect,
  3: DependencyCheck,
  4: ModeSelect,
  5: SystemConfig,
  6: Installing,
  7: Complete,
}

const STEP_LABELS = ['欢迎', '目录选择', '依赖检测', '模式选择', '系统配置', '安装', '完成']

export default function App() {
  const [currentStep, setCurrentStep] = useState<StepId>(1)
  const [config, setConfig] = useState<InstallConfig>({
    installDir: 'C:\\Program Files\\ISDP',
    installMode: 'auto',
    dependencies: [],
    database: {
      host: '',
      port: 3306,
      database: 'isdp',
      username: 'root',
      password: '',
    },
    createShortcut: true,
    launchNow: true,
  })

  const [hasMissingDeps, setHasMissingDeps] = useState(false)

  const PageComponent = PAGES[currentStep]

  const handleNext = () => {
    // 如果在依赖检测步骤没有缺失依赖，跳过模式选择
    if (currentStep === 3 && !hasMissingDeps) {
      setCurrentStep(5 as StepId)
    } else if (currentStep < 7) {
      setCurrentStep((currentStep + 1) as StepId)
    }
  }

  const handlePrev = () => {
    // 如果从系统配置返回且没有缺失依赖，跳回依赖检测
    if (currentStep === 5 && !hasMissingDeps) {
      setCurrentStep(3 as StepId)
    } else if (currentStep > 1) {
      setCurrentStep((currentStep - 1) as StepId)
    }
  }

  const handleConfigUpdate = (updates: Partial<InstallConfig>) => {
    setConfig(prev => ({ ...prev, ...updates }))
  }

  const handleDependenciesUpdate = (deps: Dependency[]) => {
    handleConfigUpdate({ dependencies: deps })
    setHasMissingDeps(deps.some(d => !d.installed))
  }

  return (
    <Layout
      currentStep={currentStep}
      stepLabels={STEP_LABELS}
    >
      <PageComponent
        config={config}
        onConfigUpdate={handleConfigUpdate}
        onDependenciesUpdate={handleDependenciesUpdate}
        onComplete={() => setCurrentStep(7)}
      />

      {currentStep !== 6 && currentStep !== 7 && (
        <div className="footer-buttons">
          <Space>
            {currentStep > 1 && (
              <Button onClick={handlePrev}>上一步</Button>
            )}
            {currentStep === 1 && (
              <Button type="primary" onClick={handleNext}>开始安装</Button>
            )}
            {currentStep > 1 && currentStep < 6 && (
              <Button type="primary" onClick={handleNext}>
                {currentStep === 5 ? '安装' : '下一步'}
              </Button>
            )}
          </Space>
        </div>
      )}
    </Layout>
  )
}