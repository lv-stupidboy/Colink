import React, { useMemo } from 'react';
import { Collapse } from 'antd';
import { RightOutlined } from '@ant-design/icons';
import './CollapsiblePanels.css';

interface CollapsiblePanelProps {
  header: React.ReactNode;
  children: React.ReactNode;
  defaultExpanded?: boolean;
  expanded?: boolean;
  onToggle?: (expanded: boolean) => void;
  accentColor?: string;
  expandInExport?: boolean;
  className?: string;
}

/**
 * Generic collapsible panel using Ant Design Collapse + CSS variables for dynamic theming.
 *
 * Features:
 * - Controlled/uncontrolled mode switching
 * - Export mode detection and auto-expand
 * - Layout change event notification (`isdp:chat-layout-changed`)
 * - CSS variable for accentColor
 * - React.memo optimization
 * - Custom arrow for animation control
 */
const CollapsiblePanel: React.FC<CollapsiblePanelProps> = ({
  header,
  children,
  defaultExpanded = false,
  expanded: controlledExpanded,
  onToggle,
  accentColor,
  expandInExport = true,
  className = '',
}) => {
  // Detect export mode
  const isExport = typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('export') === 'true';

  // Determine if we're in controlled or uncontrolled mode
  const isControlled = controlledExpanded !== undefined;

  // Calculate initial expanded state
  const shouldExpandByDefault = (isExport && expandInExport) || defaultExpanded;

  // Use controlled state if provided, otherwise use export-aware default
  const expanded = isControlled ? controlledExpanded : shouldExpandByDefault;

  // Handle toggle - if controlled, just call onToggle; if uncontrolled, toggle internally
  const handleChange = () => {
    onToggle?.(!expanded);
  };

  // Custom header with arrow
  const customHeader = useMemo(() => (
    <div className="collapsible-panel-header" onClick={handleChange}>
      <span
        className={`collapsible-panel-arrow ${expanded ? 'expanded' : ''}`}
        style={{ color: accentColor }}
      >
        <RightOutlined />
      </span>
      <span className="collapsible-panel-title">{header}</span>
    </div>
  ), [header, expanded, accentColor, handleChange]);

  // CSS variable for dynamic theming
  const style = {
    '--accent-color': accentColor || '#1890ff',
  } as React.CSSProperties;

  return (
    <Collapse
      activeKey={expanded ? ['1'] : []}
      onChange={() => {}} // Disabled built-in toggle, use custom handler
      className={`collapsible-panel ${className}`}
      style={style}
      ghost
    >
      <Collapse.Panel
        header={customHeader}
        key="1"
        showArrow={false}
      >
        {children}
      </Collapse.Panel>
    </Collapse>
  );
};

export default React.memo(CollapsiblePanel);