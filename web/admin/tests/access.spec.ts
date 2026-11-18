import { test, expect, type Page } from "@playwright/test";
import {
  accountMenu,
  fixtureTenant,
  login,
  logout,
  nav,
  people,
  post,
  selectTenant,
} from "./helpers";

async function create(page: Page, path: string, data: unknown) {
  const r = await post(page, path, data);
  expect(r.status, path).toBeLessThan(300);
  return r.body;
}

test("Access opens the Space's access, with every role holder and inherited App assignments", async ({
  page,
}) => {
  test.setTimeout(120000);
  await login(page);
  const { tenant, owner, base } = await fixtureTenant(page, "Access");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  // The account's broadest fixed control role in this Tenant, from the
  // capabilities the server reports, sits under the name.
  const held = await (await page.request.get(base + "/capabilities")).json();
  expect(held.control_roles[0].code).toBe("tenant.admin");
  await expect(accountMenu(page).locator("small")).toHaveText(
    held.control_roles[0].display_name,
  );

  const space = crypto.randomUUID(),
    app = crypto.randomUUID(),
    late = crypto.randomUUID(),
    busy = crypto.randomUUID();
  await create(page, base + "/spaces", {
    id: space,
    slug: "access-crm",
    display_name: "Access CRM",
    initial_manager_user_id: owner.id,
    database: {
      mode: "managed",
      layout: "shared",
      target_id: "20000000-0000-4000-8000-000000000001",
    },
  });
  await create(page, base + "/spaces/" + space + "/apps", {
    id: app,
    slug: "access-app",
    display_name: "Access app",
    initial_manager_user_id: owner.id,
  });
  const spacePath = base + "/spaces/" + space;
  await create(page, spacePath + "/roles", {
    id: late,
    code: "late-reviewer",
    display_name: "Late reviewer",
  });
  await create(page, spacePath + "/roles", {
    id: busy,
    code: "busy-reviewer",
    display_name: "Busy reviewer",
  });
  // The first assignment made is the oldest, so it falls beyond the first
  // page of 50 once 51 newer ones exist.
  const teams = [
    "Late holder",
    ...Array.from({ length: 51 }, (_, i) => `Busy team ${i}`),
  ];
  for (const [i, name] of teams.entries()) {
    // Populate at the documented administrative request rate.
    if (i > 0 && i % 10 === 0) await page.waitForTimeout(1000);
    const team = crypto.randomUUID();
    await create(page, base + "/teams", {
      id: team,
      slug: "t-" + team.slice(0, 8),
      display_name: name,
    });
    await create(page, spacePath + "/role-assignments", {
      id: crypto.randomUUID(),
      role_id: i === 0 ? late : busy,
      group_id: team,
    });
  }
  const entity = await create(page, "/v1/spaces/" + space + "/entities", {
    definition: {
      format_version: 1,
      key: "deal",
      label: "Deal",
      fields: [{ key: "name", label: "Name", type: "text", max_length: 100 }],
      default_projection: ["name"],
      rules: [],
      permissions: [
        {
          key: "late",
          label: "Late reads",
          operation: "read",
          roles: [late],
          fields: ["name"],
        },
        {
          key: "busy",
          label: "Busy reads",
          operation: "read",
          roles: [busy],
          fields: ["name"],
        },
      ],
    },
  });

  // The sidebar's Access opens the (first) Space's Access tab.
  await page.reload();
  await expect(
    page
      .getByRole("complementary", { name: "Navigation" })
      .getByRole("button", { name: "Access CRM", exact: true }),
  ).toBeVisible();
  await nav(page, "Access");
  await expect(page).toHaveURL(
    new RegExp(`/tenants/${tenant}/spaces/${space}/access$`),
  );
  await expect(
    page.getByRole("heading", { name: "Access · Access CRM", level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("tab", { name: "Access", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await expect(
    page
      .getByRole("navigation", { name: "Console screens" })
      .getByRole("button", { name: "Access", exact: true }),
  ).toHaveAttribute("aria-current", "page");

  // Every holder counts, not just the first page of assignments.
  await expect(
    page.getByRole("row").filter({ hasText: "Late reviewer" }),
  ).toContainText("Late holder");
  await expect(
    page.getByRole("row").filter({ hasText: "Busy reviewer" }),
  ).toContainText("+49");
  await expect(
    page.getByRole("list", { name: "Space role assignments" }),
  ).not.toContainText("Late holder");

  // A cell opens the entity on its Access tab.
  await page
    .getByRole("button", {
      name: "Late reviewer read: allowed. Open the permission",
      exact: true,
    })
    .click();
  await expect(page).toHaveURL(
    new RegExp(`/spaces/${space}/entities\\?entity=${entity.id}&tab=access$`),
  );
  await expect(
    page
      .getByRole("tablist", { name: "Entity sections" })
      .getByRole("tab", { name: "Access", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("Late reads", { exact: true })).toBeVisible();

  // Members and teams stay reachable from the Space's Access page and the
  // account menu.
  await page.goto(`/console/tenants/${tenant}/spaces/${space}/access`);
  await page
    .getByRole("button", { name: "Members and teams", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Members and teams", level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("tab", { name: "Members", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await nav(page, "Home");
  await people(page);

  // App Access lists the App's own assignments (removable) first, then its
  // Space's, newest first, inherited and read-only.
  await page.goto(`/console/tenants/${tenant}/apps/${app}/access`);
  const rows = page
    .getByRole("list", { name: "App role assignments" })
    .getByRole("listitem");
  await expect(rows).toHaveCount(50);
  const own = rows.first();
  await expect(own).toContainText("App manager");
  await expect(own).toContainText("App · Access app");
  await expect(own.getByRole("button", { name: /^Remove / })).toBeVisible();
  await expect(own).not.toContainText("Inherited");
  const inherited = rows.nth(1);
  await expect(inherited).toContainText("Busy reviewer");
  await expect(inherited).toContainText("Busy team 50");
  await expect(inherited).toContainText("Space · Access CRM");
  await expect(inherited).toContainText("Inherited");
  await expect(inherited.getByRole("button")).toHaveCount(0);
});
