import type { ReactNode } from "react";
import { Spinner } from "./feedback";

// The whole-window wait before the console can draw anything (session and
// server settings loading): a centred, announced spinner with its label.
export function LoadingScreen({ label }: { label: ReactNode }) {
  return (
    <div className="ui-loading-screen">
      <Spinner label={label} size="4xl" />
    </div>
  );
}
