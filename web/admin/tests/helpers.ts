import { expect, type Locator, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { accountHost, apiOrigin, rootSQLCommand } from "./stack";

export { accountHost };

// Runs SQL as MariaDB root on the disposable e2e database (see stack.ts).
export function rootSQL(sql: string, extra: string[] = []): string {
  const [cmd, ...args] = rootSQLCommand;
  return execFileSync(cmd, [...args, ...extra], {
    input: sql,
    encoding: "utf8",
  });
}
export async function login(
  page: Page,
  subject = "operator",
  origin = apiOrigin,
) {
  await page.goto(origin + "/console/");
  await page.getByRole("link", { name: "Continue with SSO" }).click();
  await page.locator(`input[name=subject][value="${subject}"]`).check();
  await page.getByLabel("Password", { exact: true }).fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    subject === "operator"
      ? page.getByRole("heading", { name: "Tenants", exact: true, level: 1 })
      : homeHeading(page),
  ).toBeVisible();
}
// A Tenant's Home is titled with a greeting to the signed-in person.
export function homeHeading(page: Page) {
  return page.getByRole("heading", {
    name: /^Good (morning|afternoon|evening), /,
    level: 1,
  });
}
// Opens the Tenant's Spaces list: the title of the sidebar's Spaces section.
export async function openSpaces(page: Page) {
  await page
    .getByRole("complementary", { name: "Navigation" })
    .getByRole("button", { name: "Spaces", exact: true })
    .click();
}
// The account menu button: its name is the signed-in user's visible name
// followed by "account menu" (label in name).
export function accountMenu(page: Page) {
  return page
    .locator(".profile-menu")
    .getByRole("button", { name: /, account menu$/ });
}
// Opens the account menu; on a narrow screen it sits in the navigation
// drawer, which is opened first.
export async function menu(page: Page) {
  const drawer = page.getByRole("button", {
    name: "Open navigation",
    exact: true,
  });
  const trigger = accountMenu(page);
  // The shell switches to the drawer with this media query; asking the page
  // (not a possibly stale button) is stable right after a viewport resize.
  const narrow = await page.evaluate(
    () => matchMedia("(max-width:800px)").matches,
  );
  if (narrow && (await drawer.getAttribute("aria-expanded")) !== "true")
    await drawer.click();
  if ((await trigger.getAttribute("aria-expanded")) !== "true")
    await trigger.click();
  await expect(page.getByRole("menu")).toBeVisible();
}
export async function logout(page: Page) {
  await menu(page);
  await page.getByRole("menuitem", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
}
// Tenant settings is an account-menu entry, not a sidebar screen.
export async function tenantSettings(page: Page) {
  await menu(page);
  await page
    .getByRole("menuitem", { name: "Tenant settings", exact: true })
    .click();
}
export async function nav(page: Page, name: string) {
  await page
    .getByRole("navigation", { name: "Console screens" })
    .getByRole("button", { name, exact: true })
    .click();
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
// Switches the Tenant with the sidebar's Tenant section (a button named
// "Tenant <current name>" that opens a search field over the paged list).
// An account without a Tenant has no switcher and nothing to switch.
// The sidebar's Tenant switcher: "Tenant <name>" (the account menu, which
// may also start with "Tenant", ends with "account menu").
export function tenantSwitcher(page: Page) {
  return page
    .getByRole("complementary", { name: "Navigation" })
    .getByRole("button", { name: /^Tenant (?!.*account menu$)/ });
}
export async function selectTenant(page: Page, tenant: string) {
  const trigger = tenantSwitcher(page);
  if (await trigger.count()) {
    await trigger.click();
    await page
      .getByRole("combobox", { name: "Find a Tenant", exact: true })
      .fill(tenant);
    await page
      .getByRole("listbox", { name: "Tenants", exact: true })
      .locator(`[role="option"][data-value="${tenant}"]`)
      .click();
  }
  await expect(page).toHaveURL(new RegExp("/tenants/" + tenant + "/"));
}
export async function post(
  page: Page,
  path: string,
  data: unknown,
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
      (u: { oidc_subject: string }) => u.oidc_subject === "tenant-admin",
    ),
    outsider = outsiders.items.find(
      (u: { oidc_subject: string }) => u.oidc_subject === "outsider",
    );
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
// Answers the console's ConfirmDialog (it replaced window.confirm()).
export async function confirmDialog(page: Page, button: string) {
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: button, exact: true }).click();
  await expect(dialog).toHaveCount(0);
}
