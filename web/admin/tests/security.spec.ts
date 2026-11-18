import { test, expect, Page } from "@playwright/test";
import {
  selectTenant,
  logout,
  choose,
  fixtureTenant,
  pick,
  openSpaces,
  homeHeading,
} from "./helpers";
import { apiOrigin, uiOrigin } from "./stack";

async function signIn(page: Page, origin = apiOrigin, subject = "operator") {
  await page.goto(origin + "/console/");
  await page
    .getByRole("link", { name: "Continue with SSO", exact: true })
    .click();
  await page.locator(`input[name=subject][value="${subject}"]`).check();
  await page.getByLabel("Password", { exact: true }).fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(new RegExp("^" + origin + "/console/"));
  await expect(
    subject === "operator"
      ? page.getByRole("heading", { name: "Tenants", exact: true, level: 1 })
      : homeHeading(page),
  ).toBeVisible();
}

async function create(page: Page, path: string, body: object) {
  const result = await page.evaluate(
    async ({ path, body }) => {
      const csrf = document.cookie
        .split("; ")
        .find((v) => v.startsWith("lastapp_csrf="))!
        .split("=")[1];
      const response = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify(body),
      });
      return { status: response.status, body: await response.json() };
    },
    { path, body },
  );
  expect(result.status, JSON.stringify(result.body)).toBe(201);
  return result.body;
}

test("server paging, search and option pickers reach records beyond the first page", async ({
  page,
}) => {
  await signIn(page);
  const { tenant, owner } = await fixtureTenant(page, "AAA Paging");
  await logout(page);
  await signIn(page, apiOrigin, "tenant-admin");
  await selectTenant(page, tenant);
  const space = crypto.randomUUID();
  await create(page, `/v1/tenants/${tenant}/spaces`, {
    id: space,
    slug: "paging",
    display_name: "Paging",
    initial_manager_user_id: owner.id,
    database: {
      mode: "managed",
      layout: "shared",
      target_id: "20000000-0000-4000-8000-000000000001",
    },
  });

  for (let i = 0; i < 101; i++) {
    // Populate at the documented administrative request rate.
    if (i > 0 && i % 20 === 0) await page.waitForTimeout(1000);
    const lastRole = crypto.randomUUID();
    await create(page, `/v1/tenants/${tenant}/spaces/${space}/roles`, {
      id: lastRole,
      code: `paged-${i}`,
      display_name: `PageRole ${String(i).padStart(3, "0")}`,
    });
  }
  await openSpaces(page);
  await page
    .getByRole("row")
    .filter({ hasText: "Paging" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await page.getByRole("tab", { name: "Access", exact: true }).click();
  await page.getByRole("button", { name: "Manage roles", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "PageRole 100" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Next records", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "PageRole 100" }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill("PageRole 100");
  await expect(
    page.getByRole("row").filter({ hasText: "PageRole 100" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Previous records", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Next records", exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("button", { name: "People and teams", exact: true })
    .click();
  await page.getByRole("button", { name: "Add access", exact: true }).click();
  const role = page.getByRole("combobox", { name: "Role", exact: true });
  await role.click();
  const choices = page
    .getByRole("listbox", { name: "Role", exact: true })
    .getByRole("option");
  await expect(choices).toHaveCount(50);
  await page.getByRole("button", { name: "Load more", exact: true }).click();
  await expect(choices).toHaveCount(100);
  await page.getByRole("button", { name: "Load more", exact: true }).click();
  await expect(choices.filter({ hasText: "PageRole 100" })).toBeVisible();
  await role.press("Escape");
  await expect(
    page.getByRole("dialog", { name: "Add access", exact: true }),
  ).toBeVisible();
  await choose(page, "Role", "PageRole 100");
  await expect(
    page.getByRole("combobox", { name: "Role", exact: true }),
  ).toHaveValue("PageRole 100");
});

test("Vite login returns to the development UI and authenticated writes succeed", async ({
  page,
}) => {
  await signIn(page, uiOrigin);
  await page.getByRole("button", { name: "Tenants", exact: true }).click();
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const suffix = Date.now().toString();
  await pick(page.getByLabel("Account", { exact: true }), "existing");
  await choose(page, "Existing account", "Tenant administrator");
  await page.getByLabel("Name", { exact: true }).fill("AAA Proxy " + suffix);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill("AAA Proxy " + suffix);
  await expect(
    page.getByRole("row").filter({ hasText: "AAA Proxy " + suffix }),
  ).toBeVisible();
});
