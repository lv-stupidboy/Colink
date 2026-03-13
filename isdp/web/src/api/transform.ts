/**
 * API 响应转换工具
 * 将后端 snake_case 字段转换为前端 camelCase
 */

// 递归转换对象中的所有 snake_case 键为 camelCase
export function snakeToCamel(obj: any): any {
  if (obj === null || obj === undefined) {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(item => snakeToCamel(item));
  }

  if (typeof obj !== 'object') {
    return obj;
  }

  const result: any = {};
  for (const key in obj) {
    if (Object.prototype.hasOwnProperty.call(obj, key)) {
      const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
      result[camelKey] = snakeToCamel(obj[key]);
    }
  }
  return result;
}

// 递归转换 camelCase 为 snake_case（用于发送请求）
export function camelToSnake(obj: any): any {
  if (obj === null || obj === undefined) {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(item => camelToSnake(item));
  }

  if (typeof obj !== 'object') {
    return obj;
  }

  const result: any = {};
  for (const key in obj) {
    if (Object.prototype.hasOwnProperty.call(obj, key)) {
      const snakeKey = key.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);
      result[snakeKey] = camelToSnake(obj[key]);
    }
  }
  return result;
}

// 转换 Project 数据
export function transformProject(data: any): any {
  return snakeToCamel(data);
}

// 转换 Project 列表
export function transformProjects(data: any[]): any[] {
  return data.map(transformProject);
}

// 转换 Thread 数据
export function transformThread(data: any): any {
  return snakeToCamel(data);
}

// 转换 Thread 列表
export function transformThreads(data: any[]): any[] {
  if (!data || !Array.isArray(data)) {
    return [];
  }
  return data.map(transformThread);
}

// 转换 Message 数据
export function transformMessage(data: any): any {
  return snakeToCamel(data);
}

// 转换 Message 列表
export function transformMessages(data: any[]): any[] {
  if (!data || !Array.isArray(data)) {
    return [];
  }
  return data.map(transformMessage);
}

// 转换 AgentConfig 数据
export function transformAgentConfig(data: any): any {
  return snakeToCamel(data);
}

// 转换 AgentConfig 列表
export function transformAgentConfigs(data: any[]): any[] {
  if (!data || !Array.isArray(data)) {
    return [];
  }
  return data.map(transformAgentConfig);
}

// 转换 AgentInvocation 数据
export function transformAgentInvocation(data: any): any {
  return snakeToCamel(data);
}

// 转换 AgentInvocation 列表
export function transformAgentInvocations(data: any[]): any[] {
  if (!data || !Array.isArray(data)) {
    return [];
  }
  return data.map(transformAgentInvocation);
}

// 转换 Artifact 数据
export function transformArtifact(data: any): any {
  return snakeToCamel(data);
}

// 转换 Artifact 列表
export function transformArtifacts(data: any[]): any[] {
  if (!data || !Array.isArray(data)) {
    return [];
  }
  return data.map(transformArtifact);
}
