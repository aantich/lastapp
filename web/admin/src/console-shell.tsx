import { useEffect, useRef, useState, type ReactNode } from "react";
import { Layers3, Menu, X, ChevronDown, type LucideIcon } from "lucide-react";
import { useAppearance } from "./appearance";
import { AppearancePanel } from "./appearance-panel";
import { useDialogFocus } from "./dialog";
import type { Item } from "./api-types";
export function ConsoleShell({
  me,
  kind,
  title,
  nav,
  active,
  navigate,
  context,
  breadcrumbs,
  actions,
  children,
  appearanceControls,
  account,
  signOut,
}: {
  me: Item;
  kind: string;
  title: string;
  nav: { id: string; label: string; icon: LucideIcon }[];
  active: string;
  navigate: (page: string) => void;
  context?: ReactNode;
  breadcrumbs?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  appearanceControls: ReturnType<typeof useAppearance>;
  account: () => void;
  signOut: () => void;
}) {
  const [mobile, setMobile] = useState(
      () => matchMedia("(max-width:800px)").matches,
    ),
    [open, setOpen] = useState(false),
    [collapsed, setCollapsed] = useState(false),
    [appearanceOpen, setAppearanceOpen] = useState(false);
  const ref = useDialogFocus(() => setOpen(false), mobile && open);
  const profile = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    const outside = (e: PointerEvent) => {
      if (profile.current && !profile.current.contains(e.target as Node))
        profile.current.open = false;
    };
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape" && profile.current?.open) {
        profile.current.open = false;
        profile.current.querySelector("summary")?.focus();
      }
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", escape);
    };
  }, []);
  useEffect(() => {
    const m = matchMedia("(max-width:800px)");
    const update = () => {
      setMobile(m.matches);
      setOpen(false);
    };
    m.addEventListener("change", update);
    return () => m.removeEventListener("change", update);
  }, []);
  return (
    <div
      className={
        "shell tenant-console" +
        (open ? " navigation-open" : "") +
        (collapsed ? " sidebar-collapsed" : "")
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
            <Layers3 size={23} />
          </span>
          <strong>
            lastapp<span>{kind}</span>
          </strong>
          {mobile && (
            <button
              className="icon"
              aria-label="Close navigation"
              onClick={() => setOpen(false)}
            >
              <X />
            </button>
          )}
        </div>
        {context && <div className="tenant-switch">{context}</div>}
        <nav aria-label="Console screens">
          <div className="nav-group">
            {nav.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                className={active === id ? "selected" : ""}
                aria-current={active === id ? "page" : undefined}
                onClick={() => {
                  navigate(id);
                  setOpen(false);
                }}
              >
                <Icon size={19} />
                <span>{label}</span>
              </button>
            ))}
          </div>
        </nav>
      </aside>
      <main className="main">
        <header className="topbar">
          <div className="topbar-left">
            <button
              className="icon menu-toggle"
              aria-label={
                mobile
                  ? "Open navigation"
                  : collapsed
                    ? "Expand sidebar"
                    : "Collapse sidebar"
              }
              aria-expanded={mobile ? open : !collapsed}
              onClick={() =>
                mobile ? setOpen(true) : setCollapsed(!collapsed)
              }
            >
              <Menu size={21} />
            </button>
            <span className="topbar-workspace">{breadcrumbs || kind}</span>
          </div>
          <details className="profile-menu" ref={profile}>
            <summary aria-label="Account menu">
              <span className="avatar person">
                {me.display_name?.slice(0, 1)}
              </span>
              <span>{me.display_name}</span>
              <ChevronDown size={14} />
            </summary>
            <div className="profile-menu-items">
              <button
                onClick={(e) => {
                  e.currentTarget.closest("details")!.open = false;
                  account();
                }}
              >
                My account
              </button>
              <button
                onClick={(e) => {
                  const details = e.currentTarget.closest("details")!;
                  details.open = false;
                  details.querySelector("summary")?.focus();
                  setAppearanceOpen(true);
                }}
              >
                Appearance
              </button>
              <button onClick={signOut}>Sign out</button>
            </div>
          </details>
        </header>
        <section className="content" id="main-content" tabIndex={-1}>
          <div className="page-title-row">
            <h1>{title}</h1>
            <div className="row-actions">{actions}</div>
          </div>
          {children}
          <footer className="page-footer">
            <span>© {new Date().getFullYear()} Lastapp</span>
          </footer>
        </section>
      </main>
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
