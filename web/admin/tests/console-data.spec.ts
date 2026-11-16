import { test, expect, type Page } from "@playwright/test";
import {
  login,
  logout,
  fixtureTenant,
  selectTenant,
  post,
  pick,
} from "./helpers";

// Console data layer: cached reads poll without overlap and pause behind open
// forms, editable lists keep their items' identity, and the split pages lay
// out at desktop and phone widths.

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

  await page.locator("nav").getByRole("button", { name: "Spaces" }).click();
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
  await page.getByLabel("Entity name", { exact: true }).fill("Ticket");
  await page
    .getByRole("button", { name: "Create entity", exact: true })
    .click();
  await page.getByRole("button", { name: "Add field", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name", { exact: true }).fill("Title");
  await dialog.getByRole("button", { name: "Save field", exact: true }).click();
  await page.getByRole("button", { name: "Rules", exact: true }).click();
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
