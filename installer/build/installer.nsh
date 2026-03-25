; ISDP 自定义安装脚本

; 安装完成后复制启动器到安装目录
!macro customInstall
  CopyFiles "$INSTDIR\resources\ISDP-Launcher.exe" "$INSTDIR\ISDP-Launcher.exe"
!macroend

; 卸载时清理
!macro customUnInstall
  ; 询问是否删除配置文件
  MessageBox MB_YESNO "Delete configuration and user data?$\nYes = Delete all data$\nNo = Keep for reinstall" IDYES deleteAll IDNO keepData

  deleteAll:
    RMDir /r "$INSTDIR\config.yaml"
    RMDir /r "$INSTDIR\logs"
    RMDir /r "$INSTDIR\agent-assets"
    RMDir /r "$INSTDIR\repos"
    Goto done

  keepData:
    ; 保留配置和数据文件

  done:
!macroend