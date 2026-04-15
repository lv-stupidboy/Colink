// web/src/components/thread/useAutoScrollControl.ts
import { useState, useEffect, useRef, useCallback, RefObject } from 'react';

interface AutoScrollControlResult {
  isNearBottom: boolean;
  bottomAnchorRef: RefObject<HTMLDivElement>;
  scrollToBottom: () => void;
}

/**
 * 自动滚动控制 Hook
 * 使用 IntersectionObserver 监听底部锚点，判断用户是否接近底部
 *
 * @param containerRef - 消息列表容器 ref
 * @param threshold - 底部阈值（px），在此范围内视为接近底部
 */
export const useAutoScrollControl = (
  containerRef: RefObject<HTMLElement>,
  threshold: number = 100
): AutoScrollControlResult => {
  const [isNearBottom, setIsNearBottom] = useState(true);
  const bottomAnchorRef = useRef<HTMLDivElement>(null);
  const isObservingRef = useRef(false);

  // IntersectionObserver 监听底部锚点
  useEffect(() => {
    if (!bottomAnchorRef.current || !containerRef.current) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        // 当底部锚点进入视口时，认为接近底部
        setIsNearBottom(entry.isIntersecting);
      },
      {
        root: containerRef.current,
        threshold: 0.1,
        rootMargin: `0px 0px ${threshold}px 0px`, // 扩大底部检测范围
      }
    );

    observer.observe(bottomAnchorRef.current);
    isObservingRef.current = true;

    return () => {
      observer.disconnect();
      isObservingRef.current = false;
    };
  }, [containerRef, threshold]);

  // 手动滚动到底部
  const scrollToBottom = useCallback(() => {
    if (bottomAnchorRef.current) {
      bottomAnchorRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
      setIsNearBottom(true);
    }
  }, []);

  return {
    isNearBottom,
    bottomAnchorRef,
    scrollToBottom,
  };
};

export default useAutoScrollControl;