const { createWriteStream, existsSync, readdirSync, statSync, readFileSync, mkdirSync, writeFileSync, copyFileSync } = require('fs')
const { join, basename } = require('path')
const archiver = require('archiver')

// Get full version: priority from environment variable (set by build script)
let fullVersion = process.env.ISDP_FULL_VERSION
let os = process.env.ISDP_OS
let arch = process.env.ISDP_ARCH

if (!fullVersion) {
  // Fallback: generate from VERSION file + timestamp
  const rootVersionPath = join(__dirname, '../VERSION')
  let baseVersion
  if (existsSync(rootVersionPath)) {
    baseVersion = readFileSync(rootVersionPath, 'utf-8').trim()
  } else {
    const packageJson = require('../package.json')
    baseVersion = packageJson.version
  }

  // Generate timestamp
  const now = new Date()
  const dateStr = now.toISOString().slice(0, 10).replace(/-/g, '')
  const timeStr = now.toTimeString().slice(0, 8).replace(/:/g, '')
  fullVersion = `v${baseVersion}-${dateStr}-${timeStr}`
}

// Extract base version for db-changes directory
// fullVersion format: v0.3.0-20260408-123456
const baseVersionMatch = fullVersion.match(/^v?(\d+\.\d+\.\d+)/)
const baseVersion = baseVersionMatch ? baseVersionMatch[1] : '0.3.0'

// Detect platform if not provided
if (!os || !arch) {
  const platform = process.platform
  const nodeArch = process.arch

  os = platform === 'win32' ? 'windows' : platform === 'darwin' ? 'darwin' : 'linux'
  arch = nodeArch === 'x64' ? 'amd64' : nodeArch === 'arm64' ? 'arm64' : nodeArch
}

// Package name with platform
const packageName = `ISDP-${fullVersion}-${os}-${arch}`

// Source directory and output file
const releaseDir = join(__dirname, '../release')
const distDir = join(releaseDir, 'win-unpacked')
const outputFile = join(releaseDir, `${packageName}.zip`)

console.log('Source:', distDir)
console.log('Output:', outputFile)

// 检查源目录
if (!existsSync(distDir)) {
  console.error('Error: Source directory not found:', distDir)
  process.exit(1)
}

// 创建 zip 文件
const output = createWriteStream(outputFile)
const archive = archiver('zip', { zlib: { level: 9 } })

output.on('close', () => {
  const size = (archive.pointer() / 1024 / 1024).toFixed(2)
  console.log(`✓ Created: ${outputFile}`)
  console.log(`  Size: ${size} MB`)
})

archive.on('error', (err) => {
  console.error('Archive error:', err)
  throw err
})

archive.on('progress', (progress) => {
  if (progress.entries.total > 0) {
    const percent = Math.round((progress.entries.processed / progress.entries.total) * 100)
    process.stdout.write(`\rPackaging: ${percent}% (${progress.entries.processed}/${progress.entries.total} files)`)
  }
})

archive.pipe(output)

// 添加文件到 ISDP 目录
console.log('Packaging files...')

const files = readdirSync(distDir)
for (const file of files) {
  const filePath = join(distDir, file)
  const stat = statSync(filePath)
  if (stat.isDirectory()) {
    archive.directory(filePath, `ISDP/${file}`)
  } else {
    archive.file(filePath, { name: `ISDP/${file}` })
  }
}

// 添加数据库变更目录
console.log('Adding database changes...')
const sqlChangeDir = join(__dirname, '../sql-change')

// 1. 添加初始化 SQL
const initSqlPath = join(sqlChangeDir, 'init.sql')
if (existsSync(initSqlPath)) {
  archive.file(initSqlPath, { name: 'ISDP/runtime/data/sql-change/init.sql' })
  console.log('  Added init.sql')
}

// 2. 添加增量迁移目录（按版本号组织）
const migrationsDir = join(sqlChangeDir, 'migrations')
if (existsSync(migrationsDir)) {
  const versions = readdirSync(migrationsDir)
    .filter(f => {
      const stat = statSync(join(migrationsDir, f))
      return stat.isDirectory() && f.startsWith('v')
    })
    .sort()

  for (const version of versions) {
    const versionSrc = join(migrationsDir, version)
    const sqlFiles = readdirSync(versionSrc)
      .filter(f => f.endsWith('.sql'))
      .sort()

    if (sqlFiles.length > 0) {
      const versionDest = `ISDP/runtime/data/sql-change/migrations/${version}`

      for (const sqlFile of sqlFiles) {
        const sqlPath = join(versionSrc, sqlFile)
        archive.file(sqlPath, { name: `${versionDest}/${sqlFile}` })
      }

      // 生成 README.txt
      const readmeContent = generateDbChangeReadme(version, sqlFiles)
      archive.append(readmeContent, { name: `${versionDest}/README.txt` })

      console.log(`  Added ${sqlFiles.length} SQL files to ${version}`)
    }
  }
} else {
  console.log('  No migrations directory found, skipping')
}

// 添加配置模板文件
const templatePath = join(__dirname, '../configs/config.yaml.example')
if (existsSync(templatePath)) {
  archive.file(templatePath, { name: 'ISDP/runtime/data/configs/config.yaml.example' })
  console.log('  Added config template')
}

archive.finalize()

// 生成数据库变更 README
function generateDbChangeReadme(version, sqlFiles) {
  let content = `=== 版本 ${version} 数据库变更 ===\n\n`
  content += `包含以下 SQL 文件：\n\n`

  for (const file of sqlFiles) {
    content += `  - ${file}\n`
  }

  content += `\n执行顺序：按文件名前缀（日期序号）依次执行\n`
  content += `\n执行方法：\n`
  content += `  mysqlsh --sql -h <host> -P 3306 -u <user> -p<password> -D <database> -f <文件路径>\n`
  content += `\n或使用 MySQL 客户端：\n`
  content += `  mysql -h <host> -P 3306 -u <user> -p<password> <database> < <文件路径>\n`

  return content
}