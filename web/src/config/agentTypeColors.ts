// Agent 类型颜色列表（按顺序分配）
export const AGENT_TYPE_COLORS = ['blue', 'green', 'purple', 'orange', 'cyan', 'magenta', 'red', 'geekblue'];

// 根据类型在列表中的位置获取颜色
export const getTypeColorByIndex = (agentTypes: { type: string }[], type: string): string => {
  const index = agentTypes.findIndex(t => t.type === type);
  if (index < 0) return 'default';
  return AGENT_TYPE_COLORS[index % AGENT_TYPE_COLORS.length];
};