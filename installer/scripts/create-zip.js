const { createWriteStream, existsSync, readdirSync, statSync, readFileSync, mkdirSync, writeFileSync, copyFileSync } = require('fs')
const { join, basename } = require('path')
const archiver = require('archiver')

// Get full version: priority from environment variable (set by build script)
let fullVersion = process.env.COLINK_FULL_VERSION
let os = process.env.COLINK_OS
let arch = process.env.COLINK_ARCH

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
const packageName = `Colink-${fullVersion}-${os}-${arch}`

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

// 添加文件到 Colink 目录
console.log('Packaging files...')

const files = readdirSync(distDir)
for (const file of files) {
  const filePath = join(distDir, file)
  const stat = statSync(filePath)
  if (stat.isDirectory()) {
    archive.directory(filePath, `Colink/${file}`)
  } else {
    archive.file(filePath, { name: `Colink/${file}` })
  }
}

// 添加数据库变更目录
console.log('Adding database changes...')
const sqlChangeDir = join(__dirname, '../sql-change')

// 添加版本迁移目录（v1.0.0, v1.1.0 等）
const versions = readdirSync(sqlChangeDir)
  .filter(f => {
    const stat = statSync(join(sqlChangeDir, f))
    return stat.isDirectory() && f.startsWith('v')
  })
  .sort()

for (const version of versions) {
  const versionSrc = join(sqlChangeDir, version)

  // 检查 mysql 和 sqlite 子目录
  const dbTypes = ['mysql', 'sqlite']
  for (const dbType of dbTypes) {
    const dbTypeSrc = join(versionSrc, dbType)
    if (existsSync(dbTypeSrc)) {
      const sqlFiles = readdirSync(dbTypeSrc)
        .filter(f => f.endsWith('.sql'))
        .sort()

      if (sqlFiles.length > 0) {
        const versionDest = `Colink/runtime/data/sql-change/${version}/${dbType}`

        for (const sqlFile of sqlFiles) {
          const sqlPath = join(dbTypeSrc, sqlFile)
          archive.file(sqlPath, { name: `${versionDest}/${sqlFile}` })
        }

        console.log(`  Added ${sqlFiles.length} SQL files to ${version}/${dbType}`)
      }
    }
  }
}

// 添加配置模板文件
const templatePath = join(__dirname, '../configs/config.yaml.example')
if (existsSync(templatePath)) {
  archive.file(templatePath, { name: 'Colink/runtime/data/configs/config.yaml.example' })
  console.log('  Added config template')
}

archive.finalize()