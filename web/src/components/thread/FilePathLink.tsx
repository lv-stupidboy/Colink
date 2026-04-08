// isdp/web/src/components/thread/FilePathLink.tsx
import React, { memo } from 'react';

/**
 * 文件路径链接组件
 * 识别文本中的文件路径并提供 VSCode 打开功能
 */

interface FilePathLinkProps {
  text: string;
  projectPath?: string; // 项目根目录
}

/**
 * 文件路径正则表达式
 * 支持格式：
 * - 相对路径: src/file.ts
 * - 带行号: src/file.ts:10 或 src/file.ts:10:20
 * - Windows 路径: C:\path\file.ts
 * - Unix 路径: /path/to/file.ts
 */
const FILE_PATH_REGEX = /(?:^|[\s"'`(])(?!vscode:\/\/)((?:[A-Za-z]:\\|\/)?(?:[\w.-]+[\/\\])*[\w.-]+\.[\w]+(?:\:\d+(?:\:\d+)?)?)(?:$|[\s"'`),;.])/g;

/**
 * 从路径中提取文件名和行号
 */
function parseFilePath(pathWithLine: string): { path: string; line?: number; column?: number } {
  // 匹配行号和列号
  const lineMatch = pathWithLine.match(/:(\d+)(?::(\d+))?$/);

  if (lineMatch) {
    const path = pathWithLine.slice(0, lineMatch.index);
    const line = parseInt(lineMatch[1], 10);
    const column = lineMatch[2] ? parseInt(lineMatch[2], 10) : undefined;
    return { path, line, column };
  }

  return { path: pathWithLine };
}

/**
 * 生成 VSCode 协议链接
 * @param filePath 文件路径
 * @param projectPath 项目根目录（用于相对路径）
 * @param line 行号（可选）
 * @param column 列号（可选）
 */
function generateVSCodeLink(
  filePath: string,
  projectPath?: string,
  line?: number,
  column?: number
): string {
  let absolutePath = filePath;

  // 如果是相对路径，结合项目根目录
  if (projectPath && !filePath.match(/^[A-Za-z]:|^\/|^vscode:/)) {
    // 标准化路径分隔符
    const normalizedProjectPath = projectPath.replace(/\\/g, '/');
    const normalizedFilePath = filePath.replace(/\\/g, '/');
    absolutePath = `${normalizedProjectPath}/${normalizedFilePath}`;
  }

  // 构建 VSCode 链接
  let link = `vscode://file/${absolutePath}`;

  if (line !== undefined) {
    link += `:${line}`;
    if (column !== undefined) {
      link += `:${column}`;
    }
  }

  return link;
}

/**
 * 判断文本是否可能是文件路径
 * 简单检查：包含扩展名且不是 URL
 */
function isLikelyFilePath(text: string): boolean {
  // 排除 URL
  if (text.includes('://')) return false;

  // 排除常见的非文件扩展名
  const nonFileExtensions = ['.com', '.org', '.net', '.io', '.cn', '.jp'];
  for (const ext of nonFileExtensions) {
    if (text.toLowerCase().endsWith(ext)) return false;
  }

  // 必须包含点和扩展名
  const dotIndex = text.lastIndexOf('.');
  if (dotIndex === -1 || dotIndex === text.length - 1) return false;

  // 扩展名长度合理（1-10个字符）
  const extension = text.slice(dotIndex + 1);
  return extension.length >= 1 && extension.length <= 10;
}

/**
 * 渲染带文件路径链接的文本
 */
export const FilePathLink: React.FC<FilePathLinkProps> = memo(({ text, projectPath }) => {
  const parts: React.ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let keyIndex = 0;

  // 重置正则表达式状态
  const regex = new RegExp(FILE_PATH_REGEX.source, FILE_PATH_REGEX.flags);

  while ((match = regex.exec(text)) !== null) {
    // 添加匹配前的文本
    if (match.index > lastIndex) {
      // 检查匹配前是否有分隔符，如果有则添加
      const prefixChar = text[match.index];
      if (prefixChar && /[\s"'`(]/.test(prefixChar)) {
        parts.push(text.slice(lastIndex, match.index + 1));
        lastIndex = match.index + 1;
      } else {
        parts.push(text.slice(lastIndex, match.index));
        lastIndex = match.index;
      }
    }

    // 处理文件路径
    const filePathRaw = match[1];

    if (isLikelyFilePath(filePathRaw)) {
      const { path, line, column } = parseFilePath(filePathRaw);
      const vscodeLink = generateVSCodeLink(path, projectPath, line, column);

      parts.push(
        <a
          key={`filepath-${keyIndex++}`}
          href={vscodeLink}
          className="filepath-link"
          onClick={(e) => {
            // Cmd/Ctrl + Click 直接打开 VSCode
            // 普通 Click 也打开 VSCode（简化交互）
            e.preventDefault();
            window.open(vscodeLink, '_blank');
          }}
          title={`点击在 VSCode 中打开: ${path}${line ? `:${line}` : ''}`}
          style={{
            color: '#1890ff',
            textDecoration: 'underline',
            cursor: 'pointer',
          }}
        >
          {filePathRaw}
        </a>
      );
    } else {
      // 不是有效的文件路径，保持原样
      parts.push(filePathRaw);
    }

    // 处理匹配后的分隔符
    const afterPath = text.slice(regex.lastIndex);
    const nextChar = afterPath[0];
    if (nextChar && /[\s"'`),;.]/.test(nextChar)) {
      parts.push(nextChar);
      lastIndex = regex.lastIndex + 1;
    } else {
      lastIndex = regex.lastIndex;
    }
  }

  // 添加剩余文本
  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex));
  }

  // 如果没有匹配，返回原始文本
  if (parts.length === 0) {
    return <>{text}</>;
  }

  return <>{parts}</>;
});

FilePathLink.displayName = 'FilePathLink';

/**
 * 检查文本是否包含文件路径
 */
export function hasFilePath(text: string): boolean {
  const regex = new RegExp(FILE_PATH_REGEX.source, FILE_PATH_REGEX.flags);
  return regex.test(text);
}