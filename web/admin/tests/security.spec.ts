import { test, expect, Page } from "@playwright/test";
import { selectTenant } from "./helpers";

async function signIn(
  page: Page,
  origin = "http://127.0.0.1:8787",
  subject = "operator",
) {
  await page.goto(origin + "/console/");
  await page.getByRole("link", { name: "Sign in to Lastapp" }).click();
  await page.locator("select[name=subject]").selectOption(subject);
  await page.getByLabel("Password").fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(origin + "/console/");
  await expect(
    page.getByRole("heading", {
      name: subject === "operator" ? "Overview" : "Home",
      exact: true,
    }),
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
  const tenant = crypto.randomUUID();
  const users = await (await page.request.get("/admin/v1/users")).json();
  await create(page, "/admin/v1/tenants", {
    id: tenant,
    slug: "paging-" + tenant,
    display_name: "AAA Paging " + tenant.slice(0, 8),
    plan_id: "10000000-0000-4000-8000-000000000001",
    initial_admin_user_id: users.items.find(
      (u: any) => u.oidc_subject === "tenant-admin",
    ).id,
  });
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await signIn(page, "http://127.0.0.1:8787", "tenant-admin");
  await selectTenant(page, tenant);
  const space = crypto.randomUUID();
  await create(page, `/v1/tenants/${tenant}/spaces`, {
    id: space,
    slug: "paging",
    display_name: "Paging",
    initial_manager_user_id: users.items.find(
      (u: any) => u.oidc_subject === "tenant-admin",
    ).id,
    database: {
      mode: "managed",
      layout: "shared",
      target_id: "20000000-0000-4000-8000-000000000001",
    },
  });
  let lastRole = "";
  for (let i = 0; i < 101; i++) {
    // Populate at the documented administrative request rate.
    if (i > 0 && i % 20 === 0) await page.waitForTimeout(1000);
    lastRole = crypto.randomUUID();
    await create(page, `/v1/tenants/${tenant}/spaces/${space}/roles`, {
      id: lastRole,
      code: `paged-${i}`,
      display_name: `PageRole ${String(i).padStart(3, "0")}`,
    });
  }
  await page.getByRole("button", { name: "Spaces", exact: true }).click();
  await page
    .getByRole("row")
    .filter({ hasText: "Paging" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await page.getByRole("button", { name: "Access", exact: true }).click();
  await page.getByRole("button", { name: "Roles", exact: true }).click();
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
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Next records", exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("button", { name: "Role assignments", exact: true })
    .click();
  await page.getByRole("button", { name: "Assign role", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Search Role", exact: true })
    .fill("PageRole 100");
  await page.getByLabel("Role", { exact: true }).selectOption(lastRole);
  await expect(page.getByLabel("Role", { exact: true })).toHaveValue(lastRole);
});

test("Vite login returns to the development UI and authenticated writes succeed", async ({
  page,
}) => {
  await signIn(page, "http://127.0.0.1:5173");
  await page.getByRole("button", { name: "Tenants", exact: true }).click();
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const suffix = Date.now().toString();
  await page
    .getByLabel("Administrator account", { exact: true })
    .selectOption("existing");
  await page.getByLabel("Name", { exact: true }).fill("AAA Proxy " + suffix);
  await page.getByLabel("Slug", { exact: true }).fill("proxy-" + suffix);
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "AAA Proxy " + suffix }),
  ).toBeVisible();
});
