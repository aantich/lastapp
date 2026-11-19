import { useEffect, useState, type ReactNode } from "react";
import { Menu as MenuIcon, X } from "lucide-react";
import { IconButton } from "./button";
import { Tooltip } from "./tooltip";
import { useSidebarCollapse } from "./hooks";
import { useDialogFocus } from "./use-dialog-focus";
import { SearchField } from "./search-field";
import { cx } from "./utils";
export { SearchField } from "./search-field";
export { AccountMenu } from "./account-menu";

const narrow = "(max-width:800px)";

// The sidebar toggle's mark: a panel with its divider and a chevron pointing
// the way the sidebar will move.
function PanelMark({ open }: { open: boolean }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      aria-hidden
    >
      <rect x="3" y="4" width="18" height="16" rx="3" />
      <path d={open ? "M9 4v16M16 10l-2 2 2 2" : "M9 4v16M14 10l2 2-2 2"} />
    </svg>
  );
}

// The console frame: the ink sidebar (brand, collapse toggle and search at
// the top; collapsible to a 68 px rail that survives reloads; a modal drawer
// under 800 px), the top bar with the breadcrumbs, the search field and the
// page's primary actions, and the centred content column with a skip link.
// The sidebar's own content is a render function: `rail` says whether only
// icons show, `close` shuts the drawer after a choice. `context` (the
// Tenant switcher) sits under the brand row, above the search field. The
// frame's class
// names (.shell, aside.sidebar, .main, .topbar, .content, .sidebar-collapsed)
// are also the e2e and theme hooks.
export function AppShell({
  sidebar,
  context,
  breadcrumbs,
  onSearch,
  actions,
  overlays,
  children,
}: {
  sidebar: (state: { rail: boolean; close: () => void }) => ReactNode;
  // The working-context control under the brand row.
  context?: (rail: boolean) => ReactNode;
  // The top bar's location (a Breadcrumbs element).
  breadcrumbs?: ReactNode;
  // Opens the command palette from the sidebar and top bar search fields.
  onSearch: () => void;
  actions?: ReactNode;
  // Dialogs the frame owns (command palette, appearance panel), rendered
  // inside the shell after the main region.
  overlays?: ReactNode;
  children: ReactNode;
}) {
  const [mobile, setMobile] = useState(() => matchMedia(narrow).matches),
    [open, setOpen] = useState(false),
    [collapsed, setCollapsed] = useSidebarCollapse();
  const close = () => setOpen(false);
  const ref = useDialogFocus(close, mobile && open);
  useEffect(() => {
    const m = matchMedia(narrow);
    const update = () => {
      setMobile(m.matches);
      setOpen(false);
    };
    m.addEventListener("change", update);
    return () => m.removeEventListener("change", update);
  }, []);
  const rail = collapsed && !mobile;
  const toggleLabel = rail ? "Expand sidebar" : "Collapse sidebar";
  const toggle = mobile ? (
    <IconButton
      className="ui-sidebar-toggle"
      label="Close navigation"
      icon={<X size={16} aria-hidden />}
      onClick={close}
    />
  ) : (
    <Tooltip content={toggleLabel} side="right" labelOnly>
      <button
        type="button"
        className="ui-sidebar-toggle"
        aria-label={toggleLabel}
        aria-expanded={!collapsed}
        onClick={() => setCollapsed(!collapsed)}
      >
        <PanelMark open={!rail} />
      </button>
    </Tooltip>
  );
  return (
    <div
      className={cx(
        "shell",
        open && "navigation-open",
        rail && "sidebar-collapsed",
      )}
    >
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      {mobile && open && (
        <button
          className="navigation-backdrop"
          aria-label="Close navigation"
          onClick={close}
        />
      )}
      <aside
        className="sidebar"
        ref={ref}
        inert={mobile && !open}
        role={mobile && open ? "dialog" : undefined}
        aria-modal={mobile && open ? true : undefined}
        aria-label="Navigation"
      >
        <div className="ui-sidebar-head">
          <span className="ui-brand-tile" aria-hidden>
            L
          </span>
          <strong className="ui-brand-name">Lastapp</strong>
          {toggle}
        </div>
        {context?.(rail)}
        <SearchField
          variant="sidebar"
          label="Search or run…"
          rail={rail}
          onOpen={() => {
            close();
            onSearch();
          }}
        />
        {sidebar({ rail, close })}
      </aside>
      <main className="main">
        <header className="topbar">
          {mobile && (
            <IconButton
              className="menu-toggle"
              label="Open navigation"
              aria-expanded={open}
              icon={<MenuIcon size={20} aria-hidden />}
              onClick={() => setOpen(true)}
            />
          )}
          {breadcrumbs}
          <SearchField label="Search, jump to, or run…" onOpen={onSearch} />
          {actions && <div className="topbar-actions">{actions}</div>}
        </header>
        <section className="content" id="main-content" tabIndex={-1}>
          {children}
        </section>
      </main>
      {overlays}
    </div>
  );
}

// The working context when there is nothing to switch to (a person in one
// Tenant): the sidebar's section label.
export function ContextLabel({
  value,
  className,
}: {
  value: ReactNode;
  className?: string;
}) {
  return <span className={cx("ui-nav-section", className)}>{value}</span>;
}
