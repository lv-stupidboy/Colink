import React from 'react';
import CollapsiblePanel from './CollapsiblePanel';
import { useCollapsibleState } from '@/hooks/useCollapsibleState';
import './CollapsiblePanels.css';

interface ThinkingPanelProps {
  content: string;
  label?: string;
  defaultExpanded?: boolean;
  expandInExport?: boolean;
  accentColor?: string;
}

/**
 * Thinking process display panel.
 *
 * Features:
 * - Preview text display when collapsed (first 60 chars)
 * - Configurable defaultExpanded via store
 * - Customizable label prop
 */
const ThinkingPanel: React.FC<ThinkingPanelProps> = ({
  content,
  label = 'Thinking',
  defaultExpanded = false,
  expandInExport = true,
  accentColor = '#13c2c2',
}) => {
  // Use smart collapsible state
  const { expanded, toggle } = useCollapsibleState({
    defaultExpanded,
    expandInExport,
  });

  // Generate preview text (first 60 chars)
  const preview = content.length > 60
    ? `${content.slice(0, 60)}…`
    : content;

  // Build header with preview when collapsed
  const header = (
    <>
      <span>{label}</span>
      {!expanded && preview && (
        <span className="thinking-panel-preview">: {preview}</span>
      )}
    </>
  );

  return (
    <div className="thinking-panel">
      <CollapsiblePanel
        header={header}
        expanded={expanded}
        onToggle={toggle}
        accentColor={accentColor}
        className="thinking-panel"
      >
        <pre style={{
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          fontSize: 13,
          lineHeight: 1.5,
        }}>
          {content}
        </pre>
      </CollapsiblePanel>
    </div>
  );
};

export default React.memo(ThinkingPanel);