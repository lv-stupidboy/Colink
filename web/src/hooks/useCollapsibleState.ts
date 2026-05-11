import { useState, useRef, useEffect, useLayoutEffect } from 'react';

interface UseCollapsibleStateOptions {
  defaultExpanded?: boolean;
  forceExpanded?: boolean;  // Pass streaming status here
  expandInExport?: boolean;
  onToggle?: (expanded: boolean) => void;
}

interface UseCollapsibleStateReturn {
  expanded: boolean;
  toggle: () => void;
  userInteracted: React.MutableRefObject<boolean>;
}

/**
 * Smart collapsible state hook with:
 * - User interaction tracking
 * - Auto-collapse when streaming finishes (if user didn't interact)
 * - Auto-expand in export mode
 * - Layout change event dispatch
 */
export function useCollapsibleState(options: UseCollapsibleStateOptions = {}): UseCollapsibleStateReturn {
  const {
    defaultExpanded = false,
    forceExpanded = false,
    expandInExport = true,
    onToggle,
  } = options;

  // Detect export mode
  const isExport = typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('export') === 'true';

  // Calculate initial expanded state (不再自动展开 streaming 状态)
  const shouldExpand = (isExport && expandInExport) || defaultExpanded;

  const [expanded, setExpanded] = useState(shouldExpand);

  // Track if user manually interacted with the panel
  const userInteracted = useRef(false);

  // Track previous forceExpanded to detect streaming -> done transition
  const prevForceExpanded = useRef(forceExpanded);

  // Track if component has mounted (to avoid firing event on initial mount)
  const hasMounted = useRef(false);

  // Auto-collapse when streaming finishes (if user didn't interact)
  useEffect(() => {
    // Detect transition from streaming (true) to done (false)
    if (prevForceExpanded.current && !forceExpanded && !userInteracted.current) {
      setExpanded(false);
    }
    prevForceExpanded.current = forceExpanded;
  }, [forceExpanded]);

  // Dispatch layout change event when expanded changes
  useLayoutEffect(() => {
    if (!hasMounted.current) {
      hasMounted.current = true;
      return;
    }
    window.dispatchEvent(new Event('isdp:chat-layout-changed'));
  }, [expanded]);

  const toggle = () => {
    userInteracted.current = true;
    const newValue = !expanded;
    setExpanded(newValue);
    onToggle?.(newValue);
  };

  return {
    expanded,
    toggle,
    userInteracted,
  };
}

export default useCollapsibleState;