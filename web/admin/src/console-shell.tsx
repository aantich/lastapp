import { useCallback, useState, type ReactNode } from "react";
import { LogOut, Palette, UserRound } from "lucide-react";
import { useAppearance } from "./appearance";
import { AppearancePanel } from "./appearance-panel";
import {
  AccountMenu,
  AppShell,
  Breadcrumbs,
  CommandPalette,
  PageHeader,
  QuickSearch,
  SidebarFooter,
  SidebarNav,
  useCommandPaletteShortcut,
  type Command,
  type Crumb,
  type MenuItem,
  type NavItem,
} from "./ui";
import type { Me } from "./api-types";

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
  secondaryContext,
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
  secondaryContext?: ReactNode;
  breadcrumbs?: Crumb[];
  actions?: ReactNode;
  commands?: Command[];
  children: ReactNode;
  appearanceControls: ReturnType<typeof useAppearance>;
  account: () => void;
  signOut: () => void;
}) {
  const [appearanceOpen, setAppearanceOpen] = useState(false),
    [palette, setPalette] = useState(false);
  useCommandPaletteShortcut(useCallback(() => setPalette(true), []));
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
    <AppShell
      kind={kind}
      sidebar={({ rail, close }) => (
        <>
          <SidebarNav
            label="Console screens"
            items={nav}
            active={active}
            collapsed={rail}
            onSelect={(id) => {
              navigate(id);
              close();
            }}
          />
          <SidebarFooter name={me.display_name} detail={me.email} />
        </>
      )}
      context={context}
      secondaryContext={secondaryContext}
      actions={
        <>
          <QuickSearch onOpen={() => setPalette(true)} />
          <AccountMenu
            className="profile-menu"
            name={me.display_name}
            items={accountItems}
          />
        </>
      }
      footer={<span>© {new Date().getFullYear()} Lastapp</span>}
      overlays={
        <>
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
        </>
      }
    >
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
    </AppShell>
  );
}
