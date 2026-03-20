#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// 读取 package.json 获取基础版本号
const packageJsonPath = path.join(__dirname, '../package.json');
const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
const baseVersion = packageJson.version;

// 生成时间戳
const now = new Date();
const dateStr = now.toISOString().slice(0, 10).replace(/-/g, ''); // 20260320
const timeStr = now.toTimeString().slice(0, 8).replace(/:/g, ''); // 143052
const buildTime = now.toISOString().slice(0, 19).replace('T', ' '); // 2026-03-20 14:30:52

// 完整版本号
const fullVersion = `v${baseVersion}-${dateStr}-${timeStr}`;

// 版本文件路径
const versionFilePath = path.join(__dirname, '../src/config/version.ts');

// 版本文件内容
const content = `/**
 * ISDP 版本配置
 * 此文件由构建脚本自动生成，请勿手动修改
 *
 * 版本号格式: v主版本.次版本.修订号-日期-时分秒
 * 示例: v0.3.0-20260320-143052
 */

// 基础版本号（在 package.json 中维护）
export const BASE_VERSION = '${baseVersion}';

// 完整版本号（构建时自动注入）
export const VERSION = '${fullVersion}';

// 构建时间（构建时自动注入）
export const BUILD_TIME = '${buildTime}';

// 内测标识
export const BETA_LABEL = '内测中';

/**
 * 版本历史（手动维护）
 * - v0.3.0: 云端部署支持，修复API Token传递问题，添加邀请码验证
 * - v0.2.0: 添加MySQL数据库支持
 * - v0.1.0: 初始版本
 */
`;

// 写入文件
fs.writeFileSync(versionFilePath, content, 'utf-8');

console.log(`✅ 版本号已生成: ${fullVersion}`);
console.log(`   构建时间: ${buildTime}`);