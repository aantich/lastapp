import type { CSSProperties, ReactNode } from "react";

// A page outside the console shell (an OAuth consent, a one-off decision):
// the canvas background and one centred column of at most the page width
// (--ui-page-width, or --ui-page-width-narrow), with a phone gutter. The page's content is its own landmark.
export function StandalonePage({
  width = "default",
  children,
}: {
  width?: "default" | "narrow";
  children: ReactNode;
}) {
  return (
    <main
      className="ui-standalone"
      style={
        {
          "--ui-standalone-width":
            width === "narrow"
              ? "var(--ui-page-width-narrow)"
              : "var(--ui-page-width)",
        } as CSSProperties
      }
    >
      <div className="ui-standalone-column">{children}</div>
    </main>
  );
}
