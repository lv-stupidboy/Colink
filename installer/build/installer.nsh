; ISDP 自定义安装脚本
; 引导模式：NSIS只做解压，所有交互由Electron处理

; 安装完成后无需额外操作（不复制启动器，直接使用isdp-server.exe）
!macro customInstall
  ; 无需复制启动器
!macroend

; 卸载时清理
!macro customUnInstall
  ; 基础清理由NSIS自动处理
!macroend