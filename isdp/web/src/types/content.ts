// src/types/content.ts

/**
 * 内容块类型
 */
export type ContentType =
  | 'text'           // 纯文本
  | 'code'           // 代码块
  | 'diagram'        // 架构图（Mermaid/PlantUML）
  | 'chart'          // 数据图表
  | 'image'          // 图片
  | 'table'          // 表格
  | 'document'       // 长文档
  | 'json'           // JSON/YAML 数据
  | 'error-log';     // 错误日志/堆栈

/**
 * 内容块接口
 */
export interface ContentBlock {
  type: ContentType;
  content: string;
  language?: string;
  filename?: string;
  additions?: number;
  deletions?: number;
  isNew?: boolean;
}

/**
 * 文件变更接口
 */
export interface FileChange {
  id: string;
  filename: string;
  originalContent: string | null;
  modifiedContent: string;
  additions: number;
  deletions: number;
  isNew: boolean;
}

/**
 * 代码面板状态
 */
export interface CodePanelState {
  isOpen: boolean;
  isCollapsed: boolean;
  expandedFiles: Set<string>;
  files: FileChange[];
  totalAdditions: number;
  totalDeletions: number;
}