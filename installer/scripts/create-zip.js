const { createWriteStream, existsSync, mkdirSync } = require('fs')
const { join } = require('path')
const archiver = require('archiver')

// 获取版本号
const packageJson = require('../package.json')
const version = packageJson.version

// 源目录和输出文件
const distDir = join(__dirname, '../release/win-unpacked')
const outputFile = join(__dirname, `../release/ISDP-${version}.zip`)

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

// 添加文件
console.log('Packaging files...')
archive.directory(distDir, 'ISDP')

archive.finalize()