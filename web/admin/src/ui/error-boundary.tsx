import { Component, type ReactNode } from "react";
import { TriangleAlert } from "lucide-react";
import { errorMessage } from "../use-action";
import { queryClient } from "../query";
import { retryLazyLoads } from "../retryable-lazy";

// Keeps a page's render failure (or a lazily loaded screen that failed to
// download) inside the console shell, so navigation and sign-out still work.
// Workspaces key it by route: opening another page starts a fresh boundary.
export class ErrorBoundary extends Component<
  { children: ReactNode },
  { error: unknown }
> {
  state: { error: unknown } = { error: null };
  static getDerivedStateFromError(error: unknown) {
    return { error };
  }
  render() {
    if (this.state.error === null) return this.props.children;
    return (
      <section className="panel empty" role="alert">
        <TriangleAlert size={28} />
        <h3>This page could not be shown</h3>
        <p>{errorMessage(this.state.error)}</p>
        <p>Try again, or reload the console if the problem continues.</p>
        <button
          className="secondary"
          onClick={() => {
            // The failed page's reads have no mounted observer now. Drop
            // their cached data so the retry renders from a fresh request,
            // not from the response it failed on.
            void queryClient.resetQueries({
              queryKey: ["api"],
              type: "inactive",
            });
            retryLazyLoads();
            this.setState({ error: null });
          }}
        >
          Try again
        </button>
      </section>
    );
  }
}
