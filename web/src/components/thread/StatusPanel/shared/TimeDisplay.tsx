import React from 'react';

interface Props {
  isoString?: string;
}

/**
 * 格式化时间显示（只显示时分秒）
 */
export const TimeDisplay: React.FC<Props> = ({ isoString }) => {
  if (!isoString) return <span className="time-display">—</span>;

  const date = new Date(isoString);
  return (
    <span className="time-display">
      {date.toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      })}
    </span>
  );
};

export default TimeDisplay;