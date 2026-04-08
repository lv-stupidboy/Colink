import React from 'react';
import CollapsiblePanel from './CollapsiblePanel';
import { useCollapsibleState } from '@/hooks/useCollapsibleState';
import type { ToolEvent } from '@/types';
import './CollapsiblePanels.css';

interface ToolOutputPanelProps {
  events: ToolEvent[];
  status: 'streaming' | 'done' | 'failed';
  defaultExpanded?: boolean;
  accentColor?: string;
}

/**
 * Tool output panel for displaying tool events.
 *
 * Features:
 * - Auto-expand during streaming, auto-collapse after completion
 * - Render tool events list
 * - Status summary display
 */
const ToolOutputPanel: React.FC<ToolOutputPanelProps> = ({
  events,
  status,
  defaultExpanded = false,
  accentColor = '#722ed1',
}) => {
  // Use smart collapsible state with streaming awareness
  const { expanded, toggle } = useCollapsibleState({
    defaultExpanded,
    forceExpanded: status === 'streaming',
    expandInExport: true,
  });

  // Count events by status (ISDP uses 'running' | 'success' | 'failed')
  const successCount = events.filter(e => e.status === 'success').length;
  const failedCount = events.filter(e => e.status === 'failed').length;

  // Build header with status badge
  const header = (
    <>
      <span>Tool Output</span>
      <span className={`panel-status-badge ${status}`}>
        {status === 'streaming' && `Running (${events.length})`}
        {status === 'done' && `Done (${successCount + failedCount})`}
        {status === 'failed' && `Failed (${failedCount})`}
      </span>
    </>
  );

  return (
    <div className={`tool-output-panel ${status === 'streaming' ? 'streaming' : ''}`}>
      <CollapsiblePanel
        header={header}
        expanded={expanded}
        onToggle={toggle}
        accentColor={accentColor}
        className="tool-output-panel"
      >
        {events.length === 0 ? (
          <div style={{ color: 'rgba(0, 0, 0, 0.45)', fontSize: 13 }}>
            No tool events
          </div>
        ) : (
          events.map((event) => (
            <div key={event.id} className="tool-output-event">
              <span className="tool-output-event-name">{event.name}</span>
              <span className={`tool-output-event-status ${event.status}`}>
                {event.status === 'running' && '⏳'}
                {event.status === 'success' && '✓'}
                {event.status === 'failed' && '✗'}
                {event.duration && ` ${event.duration}ms`}
              </span>
            </div>
          ))
        )}
      </CollapsiblePanel>
    </div>
  );
};

export default React.memo(ToolOutputPanel);