import { useEffect, useRef } from 'react';

export type HeaderRefreshHandler = () => void | Promise<void>;

let activeHeaderRefreshHandler: HeaderRefreshHandler | null = null;

/**
 * Runs the current page's refresh handler. Returns false when no handler is
 * registered (e.g. the page was unmounted or never opted in), so callers like
 * the header "Refresh all" button can distinguish "nothing to refresh" from a
 * silent success.
 */
export const triggerHeaderRefresh = async (): Promise<boolean> => {
  if (!activeHeaderRefreshHandler) return false;
  await activeHeaderRefreshHandler();
  return true;
};

export const useHeaderRefresh = (handler?: HeaderRefreshHandler | null, enabled = true) => {
  const lastHandlerRef = useRef<HeaderRefreshHandler | null>(null);

  useEffect(() => {
    const previousHandler = lastHandlerRef.current;
    lastHandlerRef.current = handler ?? null;

    if (!enabled || !handler) {
      if (previousHandler && activeHeaderRefreshHandler === previousHandler) {
        activeHeaderRefreshHandler = null;
      }
      return;
    }

    activeHeaderRefreshHandler = handler;

    return () => {
      if (activeHeaderRefreshHandler === handler) {
        activeHeaderRefreshHandler = null;
      }
    };
  }, [enabled, handler]);
};
