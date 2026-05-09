// auto-test/e2e/fixtures/test-config.ts
// 测试配置加载模块 - 从 config.yaml 读取端口配置

import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// 配置文件查找顺序（与后端一致）
const CONFIG_SEARCH_PATHS = [
  // 1. 命令行指定（暂不支持）
  // 2. 环境变量 ISDP_CONFIG
  process.env.ISDP_CONFIG,
  // 3. data/configs/config.yaml（用户数据目录）
  path.resolve(__dirname, '../../../data/configs/config.yaml'),
  // 4. configs/config.yaml（项目默认配置）
  path.resolve(__dirname, '../../../configs/config.yaml'),
  // 5. configs/config.yaml.example（配置模板，作为备选）
  path.resolve(__dirname, '../../../configs/config.yaml.example'),
];

// 默认端口值（与 pkg/config/config.go 中 setDefaults 一致）
const DEFAULT_SERVER_PORT = 26305;
const DEFAULT_WEB_PORT = 26306;

export interface TestConfig {
  serverPort: number;
  webPort: number;
  apiBaseUrl: string;
  webBaseUrl: string;
}

/**
 * 简单解析 YAML 配置文件，提取端口信息
 * 仅支持基本格式，复杂配置请使用 js-yaml 库
 */
function parseYamlConfig(content: string): { serverPort?: number; webPort?: number } {
  const result: { serverPort?: number; webPort?: number } = {};

  // 匹配 server.port
  const serverPortMatch = content.match(/^server:\s*\n\s*port:\s*(\d+)/m);
  if (serverPortMatch) {
    result.serverPort = parseInt(serverPortMatch[1], 10);
  } else {
    // 尝试匹配扁平格式 server.port
    const flatServerPortMatch = content.match(/^server\.port:\s*(\d+)/m);
    if (flatServerPortMatch) {
      result.serverPort = parseInt(flatServerPortMatch[1], 10);
    }
  }

  // 匹配 web.port
  const webPortMatch = content.match(/web:\s*\n(?:[^\n]*\n)*?\s*port:\s*(\d+)/);
  if (webPortMatch) {
    result.webPort = parseInt(webPortMatch[1], 10);
  } else {
    // 尝试匹配扁平格式 web.port
    const flatWebPortMatch = content.match(/^web\.port:\s*(\d+)/m);
    if (flatWebPortMatch) {
      result.webPort = parseInt(flatWebPortMatch[1], 10);
    }
  }

  return result;
}

/**
 * 查找并加载配置文件
 */
function loadConfigFile(): { serverPort?: number; webPort?: number } | null {
  for (const configPath of CONFIG_SEARCH_PATHS) {
    if (!configPath) continue;

    try {
      if (fs.existsSync(configPath)) {
        const content = fs.readFileSync(configPath, 'utf-8');
        const config = parseYamlConfig(content);

        // 如果至少找到一个端口配置，则返回
        if (config.serverPort !== undefined || config.webPort !== undefined) {
          console.log(`[TestConfig] Loaded from: ${configPath}`);
          return config;
        }
      }
    } catch (err) {
      // 文件读取失败，继续尝试下一个路径
      console.warn(`[TestConfig] Failed to read ${configPath}: ${err}`);
    }
  }

  console.log('[TestConfig] No config file found, using defaults');
  return null;
}

/**
 * 获取测试配置
 * 配置查找顺序：环境变量 → data/configs/config.yaml → configs/config.yaml → 默认值
 */
export function getTestConfig(): TestConfig {
  const fileConfig = loadConfigFile();

  // 优先使用文件配置，其次使用默认值
  const serverPort = fileConfig?.serverPort ?? DEFAULT_SERVER_PORT;
  const webPort = fileConfig?.webPort ?? DEFAULT_WEB_PORT;

  return {
    serverPort,
    webPort,
    apiBaseUrl: `http://localhost:${serverPort}/api/v1`,
    webBaseUrl: `http://localhost:${webPort}`,
  };
}

// 导出默认配置实例（便于直接使用）
export const testConfig = getTestConfig();