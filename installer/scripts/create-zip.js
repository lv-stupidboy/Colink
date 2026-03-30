const { createWriteStream, existsSync, readdirSync, statSync, readFileSync } = require('fs')
const { join } = require('path')
const archiver = require('archiver')

// Get full version: priority from environment variable (set by build script)
let fullVersion = process.env.ISDP_FULL_VERSION
let os = process.env.ISDP_OS
let arch = process.env.ISDP_ARCH

if (!fullVersion) {
  // Fallback: generate from VERSION file + timestamp
  const rootVersionPath = join(__dirname, '../../isdp/VERSION')
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

archive.finalize()