import { expect, type Locator, type Page } from "@playwright/test";
export async function login(
  page: Page,
  subject = "operator",
  origin = "http://127.0.0.1:8787",
) {
  await page.goto(origin + "/console/");
  await page.getByRole("link", { name: "Continue with SSO" }).click();
  await page.locator("select[name=subject]").selectOption(subject);
  await page.getByLabel("Password", { exact: true }).fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", {
      name: subject === "operator" ? "Tenants" : "Apps",
      exact: true,
      level: 1,
    }),
  ).toBeVisible();
}
export async function menu(page: Page) {
  if (
    !(await page
      .locator(".profile-menu")
      .evaluate((e: HTMLDetailsElement) => e.open))
  )
    await page.getByLabel("Account menu", { exact: true }).click();
}
export async function logout(page: Page) {
  await menu(page);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
}
export async function nav(page: Page, name: string) {
  await page.locator("nav").getByRole("button", { name, exact: true }).click();
}
export async function choose(
  page: Page,
  label: string,
  query: string,
  match?: string,
) {
  const field = page.getByRole("combobox", { name: label, exact: true });
  await field.fill(query);
  const options = page
    .getByRole("listbox", { name: label, exact: true })
    .getByRole("option");
  await (match ? options.filter({ hasText: match }) : options).first().click();
}
// Chooses an option of the console's Select control by value or label.
export async function pick(field: Locator, option: string) {
  // The open listbox shares the field's name, so resolve its id first.
  const id = await field.getAttribute("aria-controls");
  await field.click();
  const list = field.page().locator(`[id="${id}"]`);
  await expect(list).toBeVisible();
  const byValue = list.locator(`[role="option"][data-value="${option}"]`);
  await (
    (await byValue.count())
      ? byValue
      : list.getByRole("option", { name: option, exact: true })
  ).click();
  await expect(field).toHaveAttribute("aria-expanded", "false");
}
export async function selectTenant(page: Page, tenant: string) {
  const field = page.getByRole("combobox", { name: "Tenant", exact: true });
  if (await field.count()) await choose(page, "Tenant", tenant);
  await expect(page).toHaveURL(new RegExp("/tenants/" + tenant + "/"));
}
export async function post(
  page: Page,
  path: string,
  data: any,
  method = "POST",
  revision?: number,
) {
  return page.evaluate(
    async ({ path, data, method, revision }) => {
      const csrf =
        document.cookie
          .split("; ")
          .find(
            (s) =>
              s.startsWith("lastapp_csrf=") ||
              s.startsWith("__Host-lastapp_csrf="),
          )
          ?.split("=")[1] || "";
      const r = await fetch(path, {
        method,
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrf,
          ...(revision ? { "If-Match": String(revision) } : {}),
        },
        ...(data !== undefined ? { body: JSON.stringify(data) } : {}),
      });
      return { status: r.status, body: await r.json() };
    },
    { path, data, method, revision },
  );
}
export async function fixtureTenant(page: Page, label = "UI workspace") {
  const [owners, outsiders] = await Promise.all([
    page.request.get("/admin/v1/users?q=tenant-admin").then((r) => r.json()),
    page.request.get("/admin/v1/users?q=outsider").then((r) => r.json()),
  ]);
  const owner = owners.items.find(
      (u: any) => u.oidc_subject === "tenant-admin",
    ),
    outsider = outsiders.items.find((u: any) => u.oidc_subject === "outsider");
  const tenant = crypto.randomUUID();
  expect(
    (
      await post(page, "/admin/v1/tenants", {
        id: tenant,
        slug: "ui-" + tenant,
        display_name: label + " " + tenant,
        plan_id: "10000000-0000-4000-8000-000000000001",
        initial_admin_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  return { tenant, owner, outsider, base: "/v1/tenants/" + tenant };
}
