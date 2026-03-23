// src/utils/contentDetector.ts
import type { ContentType, ContentBlock, FileChange } from '@/types/content';

/**
 * 检测代码块的内容类型
 */
export function detectContentType(codeBlock: string, language: string): ContentType {
  // 架构图语言
  if (['mermaid', 'plantuml', 'graphviz', 'dot'].includes(language.toLowerCase())) {
    return 'diagram';
  }

  // JSON/YAML 数据
  if (['json', 'yaml', 'yml'].includes(language.toLowerCase())) {
    return 'json';
  }

  // 错误日志
  if (language.toLowerCase() === 'log' || /Error:|Exception|Stack trace|Traceback/i.test(codeBlock)) {
    return 'error-log';
  }

  // 代码
  if (language) {
    return 'code';
  }

  return 'text';
}

/**
 * 解析 Markdown 内容为内容块列表
 */
export function parseContentBlocks(content: string): ContentBlock[] {
  const blocks: ContentBlock[] = [];
  const codeBlockRegex = /```(\w+)?(?:\s+([^\n]+))?\n([\s\S]*?)```/g;

  let lastIndex = 0;
  let match;

  while ((match = codeBlockRegex.exec(content)) !== null) {
    // 添加代码块之前的文本
    if (match.index > lastIndex) {
      const textContent = content.slice(lastIndex, match.index).trim();
      if (textContent) {
        blocks.push({
          type: 'text',
          content: textContent,
        });
      }
    }

    const language = match[1] || '';
    const filename = match[2] || undefined;
    const code = match[3];

    blocks.push({
      type: detectContentType(code, language),
      content: code,
      language,
      filename,
    });

    lastIndex = match.index + match[0].length;
  }

  // 添加最后的文本
  if (lastIndex < content.length) {
    const textContent = content.slice(lastIndex).trim();
    if (textContent) {
      blocks.push({
        type: 'text',
        content: textContent,
      });
    }
  }

  return blocks;
}

/**
 * 判断内容块是否应该在右侧面板展示
 */
export function shouldShowInPanel(type: ContentType): boolean {
  return ['code', 'json', 'table', 'document'].includes(type);
}

/**
 * 判断内容块是否应该在气泡内展示
 */
export function shouldShowInBubble(type: ContentType): boolean {
  return ['diagram', 'chart', 'image', 'error-log'].includes(type);
}

/**
 * 解析代码块为文件变更列表
 * 从 Markdown 内容块中提取文件信息
 */
export function parseCodeFiles(block: ContentBlock): FileChange[] {
  const files: FileChange[] = [];

  // 如果有明确的文件名，创建单个文件变更
  if (block.filename) {
    const lines = block.content.split('\n');
    files.push({
      id: `file-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
      filename: block.filename,
      originalContent: null, // 原始代码需要从文件系统或 API 获取
      modifiedContent: block.content,
      additions: lines.length,
      deletions: 0,
      isNew: true, // 默认视为新文件，后续可获取原始代码后更新
    });
  } else {
    // 尝试从代码内容中解析文件名（如注释中的文件路径）
    const filePattern = /^(?:\/\/|#|<!--)\s*(.+?\.(?:ts|tsx|js|jsx|py|go|java|rs))\s*$/m;
    const match = block.content.match(filePattern);

    if (match) {
      files.push({
        id: `file-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
        filename: match[1].trim(),
        originalContent: null,
        modifiedContent: block.content,
        additions: block.content.split('\n').length,
        deletions: 0,
        isNew: true,
      });
    } else {
      // 无法解析文件名，使用默认名称
      files.push({
        id: `file-${Date.now()}`,
        filename: `code.${block.language || 'txt'}`,
        originalContent: null,
        modifiedContent: block.content,
        additions: block.content.split('\n').length,
        deletions: 0,
        isNew: true,
      });
    }
  }

  return files;
}

/**
 * 从多个内容块解析所有文件变更
 */
export function parseAllCodeFiles(blocks: ContentBlock[]): FileChange[] {
  const allFiles: FileChange[] = [];
  let fileIndex = 0;

  blocks.forEach((block) => {
    if (shouldShowInPanel(block.type)) {
      const files = parseCodeFiles(block);
      files.forEach((file) => {
        file.id = `file-${fileIndex++}`;
        allFiles.push(file);
      });
    }
  });

  return allFiles;
}