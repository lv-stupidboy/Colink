@echo off
chcp 65001 >nul
echo ============================================
echo     Colink 僵尸文件清理工具
echo ============================================
echo.

set INSTALL_DIR=C:\Program Files\Colink
set ZOMBIE_FILE=%INSTALL_DIR%\Colink.exe

echo [1] 检查僵尸文件是否存在...
if not exist "%ZOMBIE_FILE%" (
    echo Colink.exe 不存在，无需清理
    pause
    exit /b 0
)

echo [2] 检查文件大小...
for %%A in ("%ZOMBIE_FILE%") do set FILE_SIZE=%%~zA
echo 文件大小: %FILE_SIZE% bytes
if %FILE_SIZE% LSS 104857600 (
    echo 文件大小小于 100MB，可能是僵尸文件
)

echo.
echo [3] 强制结束所有相关进程...
taskkill /f /im Colink.exe 2>nul
taskkill /f /im colink-server.exe 2>nul
taskkill /f /im ISDP.exe 2>nul
taskkill /f /im isdp-server.exe 2>nul

echo 等待进程退出...
timeout /t 3 /nobreak >nul

echo.
echo [4] 尝试方法1: 直接删除...
del /f /q "%ZOMBIE_FILE%" 2>nul
if not exist "%ZOMBIE_FILE%" (
    echo 成功删除！
    pause
    exit /b 0
)

echo 方法1失败，尝试方法2...

echo.
echo [5] 尝试方法2: 获取权限后删除...
takeown /f "%ZOMBIE_FILE%" >nul 2>&1
icacls "%ZOMBIE_FILE%" /grant administrators:F >nul 2>&1
del /f /q "%ZOMBIE_FILE%" 2>nul
if not exist "%ZOMBIE_FILE%" (
    echo 成功删除！
    pause
    exit /b 0
)

echo 方法2失败，尝试方法3...

echo.
echo [6] 尝试方法3: 停止可能锁定文件的服务...
net stop "Windows Search" >nul 2>&1
timeout /t 2 /nobreak >nul
del /f /q "%ZOMBIE_FILE%" 2>nul
net start "Windows Search" >nul 2>&1
if not exist "%ZOMBIE_FILE%" (
    echo 成功删除！
    pause
    exit /b 0
)

echo 方法3失败，尝试方法4...

echo.
echo [7] 尝试方法4: PowerShell 强制删除...
powershell -Command "Remove-Item -Path '%ZOMBIE_FILE%' -Force -ErrorAction SilentlyContinue"
if not exist "%ZOMBIE_FILE%" (
    echo 成功删除！
    pause
    exit /b 0
)

echo 方法4失败，尝试方法5...

echo.
echo [8] 尝试方法5: 创建同名空文件覆盖...
echo. > "%ZOMBIE_FILE%.tmp"
move /y "%ZOMBIE_FILE%.tmp" "%ZOMBIE_FILE%" >nul 2>&1
del /f /q "%ZOMBIE_FILE%" 2>nul
if not exist "%ZOMBIE_FILE%" (
    echo 成功删除！
    pause
    exit /b 0
)

echo.
echo ============================================
echo 所有方法都失败了！
echo.
echo 可能的原因：
echo   1. 杀毒软件正在锁定该文件
echo   2. Windows 系统服务正在使用该文件
echo   3. 文件系统损坏
echo.
echo 建议：
echo   1. 关闭杀毒软件后重新运行此脚本
echo   2. 重启电脑后再次运行安装程序
echo ============================================
pause
exit /b 1