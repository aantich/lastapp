import { useEffect, useState, type ReactNode } from "react";
import {
  ChevronDown,
  Layers3,
  Menu as MenuIcon,
  Search,
  X,
} from "lucide-react";
import { Avatar } from "./avatar";
import { IconButton } from "./button";
import { Kbd } from "./feedback";
import { Menu, type MenuItem } from "./menu";
import { useSidebarCollapse } from "./hooks";
import { useDialogFocus } from "./use-dialog-focus";
import { cx } from "./utils";

const narrow = "(max-width:800px)";

// The console frame: the ink sidebar (collapsible to a 68 px rail that
// survives reloads, a modal drawer under 800 px), the top bar with the menu
// toggle, the working context and the trailing actions, and the content
// region with a skip link and the page footer. The sidebar's own content is
// a render function: `rail` says whether only icons show, `close` shuts the
// drawer after a choice. `secondaryContext` (the Space switcher) hides on
// narrow screens. The frame's class names (.shell, aside.sidebar, .main,
// .topbar, .content, .sidebar-collapsed) are also the e2e and theme hooks.
export function AppShell({
  kind,
  sidebar,
  context,
  secondaryContext,
  actions,
  footer,
  overlays,
  children,
}: {
  // The workspace name after the wordmark ("Tenant", "Platform").
  kind: ReactNode;
  sidebar: (state: { rail: boolean; close: () => void }) => ReactNode;
  context?: ReactNode;
  secondaryContext?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
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
  return (
    <div
      className={cx(
        "shell tenant-console",
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
        <div className="sidebar-brand">
          <span className="brand-mark">
            <Layers3 size={23} aria-hidden />
          </span>
          <strong>
            lastapp<span>{kind}</span>
          </strong>
          {mobile && (
            <IconButton
              label="Close navigation"
              icon={<X aria-hidden />}
              onClick={close}
            />
          )}
        </div>
        {sidebar({ rail, close })}
      </aside>
      <main className="main">
        <header className="topbar">
          <div className="topbar-left">
            <IconButton
              className="menu-toggle"
              label={
                mobile
                  ? "Open navigation"
                  : collapsed
                    ? "Expand sidebar"
                    : "Collapse sidebar"
              }
              aria-expanded={mobile ? open : !collapsed}
              icon={<MenuIcon size={21} aria-hidden />}
              onClick={() =>
                mobile ? setOpen(true) : setCollapsed(!collapsed)
              }
            />
            {(context || secondaryContext) && (
              <div className="topbar-context">
                {context}
                {secondaryContext && (
                  <span className="topbar-space">{secondaryContext}</span>
                )}
              </div>
            )}
          </div>
          {actions && <div className="topbar-actions">{actions}</div>}
        </header>
        <section className="content" id="main-content" tabIndex={-1}>
          {children}
          {footer && <footer className="page-footer">{footer}</footer>}
        </section>
      </main>
      {overlays}
    </div>
  );
}

const shortcut = /Mac|iPhone|iPad/.test(navigator.userAgent) ? "⌘K" : "Ctrl K";

// The top bar's search trigger: opens the command palette, shows the
// platform's shortcut (⌘K or Ctrl K) and keeps its label in the accessible
// name when narrow screens hide it.
export function QuickSearch({
  label = "Search or jump to…",
  onOpen,
}: {
  label?: string;
  onOpen: () => void;
}) {
  return (
    <button
      type="button"
      className="quick-search"
      aria-keyshortcuts="Meta+K Control+K"
      onClick={onOpen}
    >
      <Search size={16} aria-hidden />
      <span className="quick-search-label">{label}</span>
      <span className="quick-search-key" aria-hidden>
        <Kbd>{shortcut}</Kbd>
      </span>
    </button>
  );
}

// The signed-in person's menu in the top bar: avatar and name (the name is
// kept for screen readers on narrow screens) over account actions.
export function AccountMenu({
  name,
  items,
  className,
}: {
  name: string;
  items: MenuItem[];
  className?: string;
}) {
  return (
    <Menu
      className={className}
      trigger={
        <>
          <Avatar name={name} size={30} />
          <span className="profile-name">{name}</span>
          <span className="ui-sr-only">, account menu</span>
          <ChevronDown size={14} aria-hidden />
        </>
      }
      items={items}
    />
  );
}

// The working context when there is nothing to switch to (a person in one
// Tenant): the kind as an eyebrow over the truncating name.
export function ContextLabel({
  kind,
  value,
  className,
}: {
  kind: ReactNode;
  value: ReactNode;
  className?: string;
}) {
  return (
    <span className={cx("ui-context-label", className)}>
      <small>{kind}</small>
      <strong>{value}</strong>
    </span>
  );
}
