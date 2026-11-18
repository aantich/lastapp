import type { CSSProperties, ReactNode } from "react";

// A page outside the console shell (an OAuth consent, a one-off decision):
// the canvas background and one centred column of at most `width` px, with
// a phone gutter. The page's content is its own landmark.
export function StandalonePage({
  width = 760,
  children,
}: {
  width?: number;
  children: ReactNode;
}) {
  return (
    <main
      className="ui-standalone"
      style={{ "--ui-standalone-width": width + "px" } as CSSProperties}
    >
      <div className="ui-standalone-column">{children}</div>
    </main>
  );
}
