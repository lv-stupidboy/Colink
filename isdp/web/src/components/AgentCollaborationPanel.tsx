import React from 'react';
import CollapsiblePanel from './CollapsiblePanel';
import { useCollapsibleState } from '@/hooks/useCollapsibleState';
import type { Message } from '@/types';
import './CollapsiblePanels.css';

interface AgentCollaborationPanelProps {
  groupId: string;
  messages: Message[];
  renderMessage: (msg: Message) => React.ReactNode;
  getAgentColor?: (agentId: string) => string | undefined;
}

/**
 * Agent collaboration container for A2A messages.
 *
 * Features:
 * - Left colored border via CSS variable
 * - Summary line showing participants
 * - Auto-expand in export mode
 */
const AgentCollaborationPanel: React.FC<AgentCollaborationPanelProps> = ({
  groupId: _groupId,
  messages,
  renderMessage,
  getAgentColor,
}) => {
  // Use smart collapsible state
  const { expanded, toggle } = useCollapsibleState({
    defaultExpanded: false,
    expandInExport: true,
  });

  // Get unique agent names
  const agentNames = [...new Set(messages.map(m => m.agentName).filter(Boolean))];

  // Get color from first agent or default
  const firstAgentId = messages.find(m => m.agentId)?.agentId;
  const borderColor = (firstAgentId && getAgentColor?.(firstAgentId)) || '#9B7EBD';

  // Build header with summary
  const header = (
    <>
      <span>{expanded ? '收起内部讨论' : '查看内部讨论'}</span>
      <span style={{ marginLeft: 8, color: 'rgba(0, 0, 0, 0.45)', fontSize: 13 }}>
        ({agentNames.join(', ')}, {messages.length} 条)
      </span>
    </>
  );

  return (
    <CollapsiblePanel
      header={header}
      expanded={expanded}
      onToggle={toggle}
      accentColor={borderColor}
      className="agent-collaboration"
    >
      <div className="agent-collaboration-messages">
        {messages.map((msg) => (
          <div key={msg.id} className="agent-collaboration-message">
            {renderMessage(msg)}
          </div>
        ))}
      </div>
    </CollapsiblePanel>
  );
};

export default React.memo(AgentCollaborationPanel);