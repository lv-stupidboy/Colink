const sharp = require('sharp');
const pngToIco = require('png-to-ico').default || require('png-to-ico');
const fs = require('fs');
const path = require('path');

async function createIcon() {
  const svgPath = 'D:/00-codes/isdp/isdp/isdp/web/public/favicon.svg';
  const pngPath = 'D:/00-codes/isdp/isdp/installer/resources/icon-256.png';
  const icoPath = 'D:/00-codes/isdp/isdp/installer/build/icon.ico';
  const resourcesIcoPath = 'D:/00-codes/isdp/isdp/installer/resources/icon.ico';

  console.log('Converting SVG to PNG...');

  // Convert SVG to PNG (256x256)
  await sharp(svgPath)
    .resize(256, 256)
    .png()
    .toFile(pngPath);

  console.log('Converting PNG to ICO...');

  // Convert PNG to ICO with multiple sizes
  const icoBuffer = await pngToIco([
    await sharp(pngPath).resize(16, 16).png().toBuffer(),
    await sharp(pngPath).resize(32, 32).png().toBuffer(),
    await sharp(pngPath).resize(48, 48).png().toBuffer(),
    await sharp(pngPath).resize(256, 256).png().toBuffer(),
  ]);

  // Write ICO files
  fs.writeFileSync(icoPath, icoBuffer);
  fs.writeFileSync(resourcesIcoPath, icoBuffer);

  console.log('Icon created successfully!');
  console.log(`  - ${icoPath}`);
  console.log(`  - ${resourcesIcoPath}`);
}

createIcon().catch(console.error);