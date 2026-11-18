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
  SidebarNav,
  useCommandPaletteShortcut,
  type Command,
  type Crumb,
  type MenuItem,
  type NavItem,
} from "./ui";
import type { Me } from "./api-types";

// The console frame shared by the Platform and Tenant workspaces: the ink
// sidebar (the workspace's section label or switcher over its screens, any
// further places such as Spaces, and the account menu at the bottom), the top
// bar (breadcrumbs, search and the workspace's primary actions), the command
// palette (⌘K/Ctrl+K) and the page header. The palette's Navigate and
// Account commands come from the same `nav` and account items the sidebar
// and menu show; workspaces add their own jumps in `commands`.
export function ConsoleShell({
  me,
  role,
  title,
  description,
  header = true,
  nav,
  active,
  navigate,
  section,
  places,
  breadcrumbs,
  actions,
  topActions,
  commands = [],
  accountItems: extraAccountItems = [],
  children,
  appearanceControls,
  account,
  signOut,
}: {
  me: Me;
  // What the account menu shows under the person's name (the workspace
  // passes a real field: the email, or the platform role).
  role?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  // False when the page draws its own heading (Home).
  header?: boolean;
  nav: NavItem[];
  active: string;
  navigate: (page: string) => void;
  section?: ReactNode;
  places?: (rail: boolean, close: () => void) => ReactNode;
  breadcrumbs: Crumb[];
  actions?: ReactNode;
  topActions?: ReactNode;
  commands?: Command[];
  // Workspace entries of the account menu before My account (Settings).
  accountItems?: MenuItem[];
  children: ReactNode;
  appearanceControls: ReturnType<typeof useAppearance>;
  account: () => void;
  signOut: () => void;
}) {
  const [appearanceOpen, setAppearanceOpen] = useState(false),
    [palette, setPalette] = useState(false);
  const openPalette = useCallback(() => setPalette(true), []);
  useCommandPaletteShortcut(openPalette);
  const accountItems: MenuItem[] = [
    ...extraAccountItems,
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
      onSearch={openPalette}
      sidebar={({ rail, close }) => (
        <>
          <SidebarNav
            label="Console screens"
            items={nav}
            active={active}
            collapsed={rail}
            section={section}
            onSelect={(id) => {
              navigate(id);
              close();
            }}
          />
          {places?.(rail, close)}
          <AccountMenu
            className="profile-menu"
            name={me.display_name}
            detail={role}
            rail={rail}
            // On a narrow screen the account sits in the drawer: an item
            // that changes the page closes it, as the navigation does.
            // Appearance opens above the drawer and returns focus to it.
            items={accountItems.map((item) =>
              item.id === "appearance"
                ? item
                : {
                    ...item,
                    onSelect: () => {
                      close();
                      item.onSelect();
                    },
                  },
            )}
          />
        </>
      )}
      breadcrumbs={<Breadcrumbs items={breadcrumbs} />}
      actions={topActions}
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
      {header && (
        <PageHeader title={title} description={description} actions={actions} />
      )}
      {children}
    </AppShell>
  );
}
