import { useCallback, useEffect, useState, type ReactNode } from "react";
import {
  ChevronDown,
  Layers3,
  LogOut,
  Menu as MenuIcon,
  Palette,
  Search,
  UserRound,
  X,
} from "lucide-react";
import { useAppearance } from "./appearance";
import { AppearancePanel } from "./appearance-panel";
import {
  Avatar,
  Breadcrumbs,
  CommandPalette,
  IconButton,
  Kbd,
  Menu,
  PageHeader,
  SidebarFooter,
  SidebarNav,
  useCommandPaletteShortcut,
  useDialogFocus,
  useSidebarCollapse,
  type Command,
  type Crumb,
  type MenuItem,
  type NavItem,
} from "./ui";
import type { Me } from "./api-types";

const shortcut = /Mac|iPhone|iPad/.test(navigator.userAgent) ? "⌘K" : "Ctrl K";

// The console frame shared by the Platform and Tenant workspaces: the ink
// sidebar (collapsible to a rail, a drawer on narrow screens), the top bar
// with the workspace's context switchers, the command palette (⌘K/Ctrl+K)
// and the account menu, and the page header. The palette's Navigate and
// Account commands come from the same `nav` and account items the sidebar
// and menu show; workspaces add their own jumps in `commands`.
export function ConsoleShell({
  me,
  kind,
  title,
  description,
  nav,
  active,
  navigate,
  context,
  breadcrumbs,
  actions,
  commands = [],
  children,
  appearanceControls,
  account,
  signOut,
}: {
  me: Me;
  kind: string;
  title: ReactNode;
  description?: ReactNode;
  nav: NavItem[];
  active: string;
  navigate: (page: string) => void;
  context?: ReactNode;
  breadcrumbs?: Crumb[];
  actions?: ReactNode;
  commands?: Command[];
  children: ReactNode;
  appearanceControls: ReturnType<typeof useAppearance>;
  account: () => void;
  signOut: () => void;
}) {
  const [mobile, setMobile] = useState(
      () => matchMedia("(max-width:800px)").matches,
    ),
    [open, setOpen] = useState(false),
    [collapsed, setCollapsed] = useSidebarCollapse(),
    [appearanceOpen, setAppearanceOpen] = useState(false),
    [palette, setPalette] = useState(false);
  const ref = useDialogFocus(() => setOpen(false), mobile && open);
  useCommandPaletteShortcut(useCallback(() => setPalette(true), []));
  useEffect(() => {
    const m = matchMedia("(max-width:800px)");
    const update = () => {
      setMobile(m.matches);
      setOpen(false);
    };
    m.addEventListener("change", update);
    return () => m.removeEventListener("change", update);
  }, []);
  const accountItems: MenuItem[] = [
    {
      id: "account",
      label: "My account",
      icon: <UserRound size={16} aria-hidden />,
      onSelect: account,
    },
    {
      id: "appearance",
      label: "Appearance",
      icon: <Palette size={16} aria-hidden />,
      onSelect: () => setAppearanceOpen(true),
    },
    {
      id: "signout",
      label: "Sign out",
      icon: <LogOut size={16} aria-hidden />,
      onSelect: signOut,
    },
  ];
  const paletteCommands: Command[] = [
    ...nav.map(({ id, label, icon: Icon }) => ({
      id: "nav:" + id,
      label,
      group: "Navigate",
      icon: <Icon size={16} />,
      run: () => navigate(id),
    })),
    ...commands,
    ...accountItems.map(({ id, label, icon, onSelect }) => ({
      id: "account:" + id,
      label,
      group: "Account",
      icon,
      run: onSelect,
    })),
  ];
  return (
    <div
      className={
        "shell tenant-console" +
        (open ? " navigation-open" : "") +
        (collapsed && !mobile ? " sidebar-collapsed" : "")
      }
    >
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      {mobile && open && (
        <button
          className="navigation-backdrop"
          aria-label="Close navigation"
          onClick={() => setOpen(false)}
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
              onClick={() => setOpen(false)}
            />
          )}
        </div>
        <SidebarNav
          label="Console screens"
          items={nav}
          active={active}
          collapsed={collapsed && !mobile}
          onSelect={(id) => {
            navigate(id);
            setOpen(false);
          }}
        />
        <SidebarFooter name={me.display_name} detail={me.email} />
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
            {context && <div className="topbar-context">{context}</div>}
          </div>
          <div className="topbar-actions">
            <button
              type="button"
              className="quick-search"
              aria-label="Search or jump to…"
              aria-keyshortcuts="Meta+K Control+K"
              onClick={() => setPalette(true)}
            >
              <Search size={16} aria-hidden />
              <span>Search or jump to…</span>
              <span aria-hidden>
                <Kbd>{shortcut}</Kbd>
              </span>
            </button>
            <Menu
              className="profile-menu"
              label={`${me.display_name}, account menu`}
              trigger={
                <>
                  <Avatar name={me.display_name} size={30} />
                  <span className="profile-name">{me.display_name}</span>
                  <ChevronDown size={14} aria-hidden />
                </>
              }
              items={accountItems}
            />
          </div>
        </header>
        <section className="content" id="main-content" tabIndex={-1}>
          <PageHeader
            title={title}
            description={description}
            breadcrumbs={
              breadcrumbs && breadcrumbs.length > 1 ? (
                <Breadcrumbs items={breadcrumbs} />
              ) : undefined
            }
            actions={actions}
          />
          {children}
          <footer className="page-footer">
            <span>© {new Date().getFullYear()} Lastapp</span>
          </footer>
        </section>
      </main>
      {palette && (
        <CommandPalette
          commands={paletteCommands}
          onClose={() => setPalette(false)}
          placeholder="Search screens, Apps, Spaces and entities…"
        />
      )}
      {appearanceOpen && (
        <AppearancePanel
          value={appearanceControls.appearance}
          onChange={appearanceControls.setAppearance}
          reset={appearanceControls.reset}
          close={() => setAppearanceOpen(false)}
        />
      )}
    </div>
  );
}
