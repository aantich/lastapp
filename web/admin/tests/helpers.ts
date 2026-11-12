import { expect, type Page } from "@playwright/test";
export async function login(
  page: Page,
  subject = "operator",
  origin = "http://127.0.0.1:8787",
) {
  await page.goto(origin + "/console/");
  await page.getByRole("link", { name: "Sign in to Lastapp" }).click();
  await page.locator("select[name=subject]").selectOption(subject);
  await page.getByLabel("Password", { exact: true }).fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(origin + "/console/");
  await expect(
    page.getByRole("heading", {
      name: subject === "operator" ? "Overview" : "Home",
      exact: true,
    }),
  ).toBeVisible();
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
export async function selectTenant(page: Page, tenant: string) {
  const picker = page.getByRole("combobox", { name: "Tenant", exact: true });
  if (await picker.count()) {
    await page
      .getByRole("textbox", { name: "Search Tenant", exact: true })
      .fill(tenant);
    await picker.selectOption(tenant);
    await expect
      .poll(async () => {
        const field = await picker.boundingBox();
        const sidebar = await page.locator(".sidebar").boundingBox();
        return field && sidebar
          ? field.x + field.width - (sidebar.x + sidebar.width)
          : 1;
      })
      .toBeLessThanOrEqual(0);
  }
  await expect
    .poll(async () => {
      const r = await page.request.get(`/v1/tenants/${tenant}/capabilities`);
      return r.status();
    })
    .toBe(200);
}
export async function fixtureTenant(page: Page, label = "UI workspace") {
  const users = (await (await page.request.get("/admin/v1/users")).json())
    .items;
  const owner = users.find((u: any) => u.oidc_subject === "tenant-admin");
  const outsider = users.find((u: any) => u.oidc_subject === "outsider");
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
  return { tenant, owner, outsider, base: `/v1/tenants/${tenant}` };
}
