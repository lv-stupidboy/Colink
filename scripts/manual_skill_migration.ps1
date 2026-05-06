# Skill 存储路径手动割接脚本 (Windows PowerShell)
# 使用方法: .\scripts\manual_skill_migration.ps1 -DbPath <db_path> -SkillsDir <skills_dir>

param(
    [string]$DbPath = ".\data\sqlite\colink.db",
    [string]$SkillsDir = ".\data\agent-assets\skills"
)

if (-not (Test-Path $DbPath)) {
    Write-Host "数据库不存在: $DbPath" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $SkillsDir)) {
    Write-Host "Skills 目录不存在: $SkillsDir" -ForegroundColor Red
    exit 1
}

Write-Host "开始割接..." -ForegroundColor Green
Write-Host "数据库: $DbPath"
Write-Host "Skills 目录: $SkillsDir"

# 查询所有 skill (id, name)
$skills = & sqlite3 $DbPath "SELECT id, name FROM skills;"

$migrated = 0
$skipped = 0

foreach ($line in $skills) {
    $parts = $line.Split('|')
    if ($parts.Length -lt 2) { continue }
    
    $id = $parts[0]
    $name = $parts[1]
    
    $srcDir = Join-Path $SkillsDir $name
    $dstDir = Join-Path $SkillsDir $id
    
    if (Test-Path $srcDir) {
        if (Test-Path $dstDir) {
            Remove-Item $dstDir -Recurse -Force
        }
        Move-Item $srcDir $dstDir -Force
        Write-Host "迁移: $name -> $id" -ForegroundColor Green
        $migrated++
    } else {
        Write-Host "跳过: $name (目录不存在)" -ForegroundColor Yellow
        $skipped++
    }
}

Write-Host "割接完成: $migrated 个迁移, $skipped 个跳过" -ForegroundColor Green
