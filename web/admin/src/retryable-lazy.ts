import { createElement, lazy, type ComponentType } from "react";

const retries = new Set<() => void>();

// React.lazy remembers a failed download for good. A retryable lazy component
// keeps the failure (so the ErrorBoundary shows it, instead of the Suspense
// retry starting a new download in a loop) until retryLazyLoads() is called;
// the ErrorBoundary's "Try again" does that, and the next render downloads
// the chunk again.
export function retryableLazy<P extends object>(
  load: () => Promise<{ default: ComponentType<P> }>,
) {
  let failed = false;
  const attempt = () =>
    load().catch((e: unknown) => {
      failed = true;
      throw e;
    });
  let Lazy = lazy(attempt);
  retries.add(() => {
    if (!failed) return;
    failed = false;
    Lazy = lazy(attempt);
  });
  return function Retryable(props: P) {
    return createElement(Lazy, props);
  };
}

export function retryLazyLoads() {
  retries.forEach((retry) => retry());
}
