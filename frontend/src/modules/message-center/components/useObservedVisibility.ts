import { useCallback, useEffect, useState } from 'react';

export function usePageVisibility(): boolean {
  const [isVisible, setIsVisible] = useState(() => document.visibilityState !== 'hidden');

  useEffect(() => {
    const handleVisibilityChange = () => setIsVisible(document.visibilityState !== 'hidden');
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
  }, []);

  return isVisible;
}

export function useObservedVisibility<T extends Element>(): {
  ref: (node: T | null) => void;
  isVisible: boolean;
} {
  const [element, setElement] = useState<T | null>(null);
  const [isVisible, setIsVisible] = useState(false);
  const ref = useCallback((node: T | null) => {
    setElement(node);
    if (!node) setIsVisible(false);
    else if (typeof IntersectionObserver === 'undefined') setIsVisible(true);
  }, []);

  useEffect(() => {
    if (!element || typeof IntersectionObserver === 'undefined') return;

    const observer = new IntersectionObserver(
      ([entry]) => setIsVisible(entry.isIntersecting && entry.intersectionRatio >= 0.5),
      { threshold: [0, 0.5] },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);

  return { ref, isVisible };
}
