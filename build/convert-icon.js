const sharp = require('sharp');
const fs = require('fs');
const path = require('path');

const svgPath = path.join(__dirname, 'appicon.svg');
const pngPath = path.join(__dirname, 'appicon.png');
const icoPath = path.join(__dirname, 'windows', 'icon.ico');

const svgContent = fs.readFileSync(svgPath, 'utf8');

async function convertIcons() {
  // 生成 1024x1024 PNG
  await sharp(Buffer.from(svgContent))
    .resize(1024, 1024)
    .png()
    .toFile(pngPath);
  console.log('Created: appicon.png');

  // 生成 256x256 PNG 用於 ICO 轉換
  const png256Path = path.join(__dirname, 'temp_256.png');
  await sharp(Buffer.from(svgContent))
    .resize(256, 256)
    .png()
    .toFile(png256Path);

  // 使用動態 import 載入 png-to-ico
  const pngToIco = (await import('png-to-ico')).default;
  
  // 轉換為 ICO
  const icoBuffer = await pngToIco(png256Path);
  fs.writeFileSync(icoPath, icoBuffer);
  console.log('Created: windows/icon.ico');

  // 清理臨時檔案
  fs.unlinkSync(png256Path);
}

convertIcons().catch(console.error);
