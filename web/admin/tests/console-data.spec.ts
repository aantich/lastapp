import { test, expect, type Page, type Route } from "@playwright/test";
import {
  login,
  logout,
  menu,
  fixtureTenant,
  selectTenant,
  post,
  pick,
  openSpaces,
} from "./helpers";

// Console data layer: cached reads poll without overlap and pause behind open
// forms, editable lists keep their items' identity, and the split pages lay
// out at desktop and phone widths. A page that fails to render, or whose lazy
// chunk fails to download, stays inside the shell and can be retried.

async function managedSpace(page: Page, base: string, owner: string) {
  const space = crypto.randomUUID();
  const created = await post(page, base + "/spaces", {
    id: space,
    slug: "data-" + space.slice(0, 8),
    display_name: "Data layer",
    initial_manager_user_id: owner,
    database: {
      mode: "managed",
      layout: "shared",
      target_id: "20000000-0000-4000-8000-000000000001",
    },
  });
  expect(created.status).toBe(201);
  return space;
}

test("tenant polling never overlaps and pauses while a form is open", async ({
  page,
}) => {
  await login(page);
  const { tenant } = await fixtureTenant(page, "Polling");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  const overview = `/v1/tenants/${tenant}/overview`;
  const started: number[] = [];
  let open = 0,
    overlapped = false;
  page.on("request", (r) => {
    if (!r.url().endsWith(overview)) return;
    started.push(Date.now());
    if (++open > 1) overlapped = true;
  });
  const finished = (r: { url: () => string }) => {
    if (r.url().endsWith(overview)) open--;
  };
  page.on("requestfinished", finished);
  page.on("requestfailed", finished);
  await page.waitForTimeout(11000);
  expect(started.length).toBeGreaterThanOrEqual(2);
  expect(started.length).toBeLessThanOrEqual(3);
  expect(overlapped).toBe(false);

  await openSpaces(page);
  await page.getByRole("button", { name: "Create Space", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  const before = started.length;
  await page.waitForTimeout(6500);
  expect(started.length).toBe(before);
  await page.getByRole("button", { name: "Close dialog" }).click();
  await expect
    .poll(() => started.length, { timeout: 7000 })
    .toBeGreaterThan(before);
});

test("removing an expression argument keeps the other arguments' inputs", async ({
  page,
}) => {
  await login(page);
  const { tenant, owner, base } = await fixtureTenant(page, "Stable keys");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  const space = await managedSpace(page, base, owner.id);
  await page.goto(`/console/tenants/${tenant}/spaces/${space}/entities`);
  await page.getByRole("button", { name: "New entity", exact: true }).click();
  await page.getByLabel("Entity name", { exact: true }).fill("Ticket");
  await page
    .getByRole("button", { name: "Create entity", exact: true })
    .click();
  await page.getByRole("button", { name: "Add field", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name", { exact: true }).fill("Title");
  await dialog.getByRole("button", { name: "Save field", exact: true }).click();
  await page.getByRole("tab", { name: "Rules", exact: true }).click();
  await page.getByRole("button", { name: "Add rule", exact: true }).click();
  await pick(
    dialog.getByLabel("Must be true operator", { exact: true }),
    "coalesce",
  );
  await dialog
    .getByRole("button", { name: "Add argument", exact: true })
    .click();
  await dialog.getByLabel("Argument 2 value", { exact: true }).fill("two");
  await dialog.getByLabel("Argument 3 value", { exact: true }).fill("three");
  const third = await dialog
    .getByLabel("Argument 3 value", { exact: true })
    .elementHandle();
  await dialog
    .getByRole("button", { name: "Remove argument", exact: true })
    .first()
    .click();
  await expect(
    dialog.getByLabel("Argument 1 value", { exact: true }),
  ).toHaveValue("two");
  await expect(
    dialog.getByLabel("Argument 2 value", { exact: true }),
  ).toHaveValue("three");
  // The input that held "three" is the same element: its key followed it.
  expect(
    await third!.evaluate((e: HTMLInputElement) => e.isConnected && e.value),
  ).toBe("three");
});

test("split Tenant and Platform pages fit desktop and phone widths", async ({
  page,
}) => {
  await login(page);
  const { tenant, owner, base } = await fixtureTenant(page, "Split layout");
  const pages = [
    "/console/platform/tenants",
    "/console/platform/users",
    "/console/platform/settings/plans",
    `/console/platform/tenants/${tenant}/overview`,
  ];
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const path of pages) {
      await page.goto(path);
      await expect(page.locator("h1")).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
        path + " at " + width,
      ).toBe(true);
    }
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  const space = await managedSpace(page, base, owner.id);
  const tenantPages = [
    `/console/tenants/${tenant}/home`,
    `/console/tenants/${tenant}/apps`,
    `/console/tenants/${tenant}/spaces`,
    `/console/tenants/${tenant}/spaces/${space}/settings`,
    `/console/tenants/${tenant}/spaces/${space}/entities`,
    `/console/tenants/${tenant}/spaces/${space}/access`,
    `/console/tenants/${tenant}/settings/general`,
    `/console/tenants/${tenant}/activity`,
  ];
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const path of tenantPages) {
      await page.goto(path);
      await expect(page.locator("h1")).toBeVisible();
      await expect(page.getByRole("alert")).toHaveCount(0);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
        path + " at " + width,
      ).toBe(true);
    }
  }
});

const fallback = (page: Page) =>
  page.getByRole("alert").filter({ hasText: "This page could not be shown" });

test("a page that throws while rendering shows the fallback inside a working shell", async ({
  page,
}) => {
  await login(page);
  const { tenant } = await fixtureTenant(page, "Boundary");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  // A connection without its permission list makes the Agent access panel
  // throw during render (permissions_json.map).
  const malformed = (route: Route) =>
    route.fulfill({
      json: {
        items: [
          {
            id: crypto.randomUUID(),
            client_name: "Malformed client",
            scope_kind: "tenant",
            state: "active",
            permissions_json: null,
            expires_at: new Date().toISOString(),
          },
        ],
        pagination: { has_more: false },
      },
    });
  await page.route("**/v1/agent-access?*", malformed);
  await menu(page);
  await page.getByRole("menuitem", { name: "My account", exact: true }).click();
  await expect(fallback(page)).toBeVisible();
  await expect(fallback(page)).toContainText("Cannot read properties of null");
  // The shell around the failed page still works.
  await openSpaces(page);
  await expect(
    page.getByRole("heading", { name: "Spaces", exact: true, level: 1 }),
  ).toBeVisible();
  await expect(fallback(page)).toHaveCount(0);
  // Returning while the response is still malformed fails the same way...
  await page.goBack();
  await expect(fallback(page)).toBeVisible();
  // ...and "Try again" renders from a fresh request once it is valid.
  await page.unroute("**/v1/agent-access?*", malformed);
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(fallback(page)).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: "Agent access", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Malformed client")).toHaveCount(0);
});

test("a failed designer chunk download shows the fallback and retries", async ({
  page,
}) => {
  await login(page);
  const { tenant, owner, base } = await fixtureTenant(page, "Lazy chunk");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  const space = await managedSpace(page, base, owner.id);
  let offline = true,
    requests = 0;
  await page.route(/\/assets\/designer-[^/]+\.js$/, (route) => {
    requests++;
    return offline ? route.abort("internetdisconnected") : route.continue();
  });
  await page.goto(`/console/tenants/${tenant}/spaces/${space}/entities`);
  await expect(fallback(page)).toBeVisible();
  expect(requests).toBe(1);
  // The shell is still usable while the designer is unavailable: the
  // breadcrumb leads back to the Space and its tabs.
  await page
    .getByRole("navigation", { name: "Breadcrumb" })
    .getByRole("button")
    .last()
    .click();
  await page.getByRole("tab", { name: "Access", exact: true }).click();
  await expect(fallback(page)).toHaveCount(0);
  await page.getByRole("tab", { name: "Entities", exact: true }).click();
  await expect(fallback(page)).toBeVisible();
  expect(requests).toBe(1);
  offline = false;
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(fallback(page)).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "New entity", exact: true }),
  ).toBeVisible();
  expect(requests).toBe(2);
});
