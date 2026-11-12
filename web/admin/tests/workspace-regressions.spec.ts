import { test, expect, type Route } from "@playwright/test";
import { login, post, selectTenant, fixtureTenant } from "./helpers";

test("only platform admins create global credentials; Tenant admins add existing accounts", async ({
  page,
  browser,
}) => {
  await login(page);
  const id = crypto.randomUUID(),
    email = `trusted-member-${id}@example.test`;
  await page.getByRole("button", { name: "Users", exact: true }).click();
  await page
    .getByRole("button", { name: "Create account", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name", { exact: true }).fill("Trusted member " + id);
  await dialog.getByLabel("Email", { exact: true }).fill(email);
  await dialog
    .getByRole("button", { name: "Generate password", exact: true })
    .click();
  const initial = await dialog
    .getByLabel("Initial password", { exact: true })
    .inputValue();
  expect(initial.length).toBeGreaterThanOrEqual(15);
  await dialog
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
  const user = (
    await (await page.request.get("/admin/v1/users?q=" + email)).json()
  ).items[0];
  expect(user.email).toBe(email);
  expect(user.platform_admin).toBeFalsy();
  const { tenant, base } = await fixtureTenant(page, "Trusted accounts");
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  const malicious = {
    display_name: "Impersonation",
    email: `unclaimed-${id}@example.test`,
    password: "Attacker supplied initial password!",
  };
  expect(
    (await post(page, base + "/memberships", { account: malicious })).status,
  ).toBe(403);
  expect((await post(page, "/admin/v1/users", malicious)).status).toBe(403);
  await page.getByRole("button", { name: "People", exact: true }).click();
  await page.getByRole("button", { name: "Add member", exact: true }).click();
  await expect(dialog).toContainText(
    "A platform administrator creates new global accounts",
  );
  await expect(
    dialog.getByRole("option", {
      name: "Create an email/password account",
      exact: true,
    }),
  ).toHaveCount(0);
  await dialog.getByLabel("Email", { exact: true }).fill(email);
  await dialog
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("row").filter({ hasText: email })).toBeVisible();
  const members = (await (await page.request.get(base + "/memberships")).json())
    .items;
  expect(members.find((m: any) => m.email === email).user_id).toBe(user.id);

  // The platform-created credentials work. Delaying a real request with the old
  // session across password rotation must not sign out the new session.
  const context = await browser.newContext();
  const member = await context.newPage();
  await member.goto("/console/");
  await member.getByLabel("Email", { exact: true }).fill(email);
  await member.getByLabel("Password", { exact: true }).fill(initial);
  await member.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    member.getByRole("heading", { name: "Set your own password" }),
  ).toBeVisible();
  const personal = "My trusted global account password!";
  await member.getByLabel("Current password", { exact: true }).fill(initial);
  await member.getByLabel("New password", { exact: true }).fill(personal);
  await member
    .getByLabel("Confirm new password", { exact: true })
    .fill(personal);
  await member
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  await expect(
    member.getByRole("heading", { name: "Home", exact: true }),
  ).toBeVisible();
  await expect(member.locator(".company-name")).toContainText(
    "Trusted accounts",
  );
  let capture: (r: Route) => void = () => {};
  const pending = new Promise<Route>((resolve) => {
    capture = resolve;
  });
  let intercepted = false;
  await member.route("**" + base + "/overview", (route) => {
    if (!intercepted) {
      intercepted = true;
      capture(route);
    } else return route.continue();
  });
  await member.getByRole("button", { name: "Refresh", exact: true }).click();
  const held = await pending;
  await member.locator(".profile-button").click();
  await member.getByLabel("Current password", { exact: true }).fill(personal);
  await member
    .getByLabel("New password", { exact: true })
    .fill("Another personal account password!");
  await member
    .getByLabel("Confirm new password", { exact: true })
    .fill("Another personal account password!");
  await member
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  await expect(member.getByRole("status")).toContainText(
    "Password changed successfully",
  );
  const oldResponse = await held.fetch();
  expect(oldResponse.status()).toBe(401);
  const finished = member.waitForResponse(
    (r) => r.url().endsWith(base + "/overview") && r.status() === 401,
  );
  await held.fulfill({ response: oldResponse });
  await (await finished).finished();
  await member.evaluate(
    () =>
      new Promise((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(resolve)),
      ),
  );
  await expect(
    member.getByRole("heading", { name: "My account", exact: true }),
  ).toBeVisible();
  await expect(
    member.getByRole("button", { name: "Sign in", exact: true }),
  ).toHaveCount(0);
  await context.close();
});

test("App-only managers can page, search and open Apps beyond the six recent links", async ({
  page,
}) => {
  await login(page);
  const { tenant, base, owner, outsider } = await fixtureTenant(
    page,
    "App directory",
  );
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  expect(
    (await post(page, base + "/memberships", { user_id: outsider.id })).status,
  ).toBe(201);
  const space = crypto.randomUUID();
  expect(
    (
      await post(page, base + "/spaces", {
        id: space,
        slug: "apps",
        display_name: "Delegation",
        initial_manager_user_id: owner.id,
        database: {
          mode: "managed",
          layout: "shared",
          target_id: "20000000-0000-4000-8000-000000000001",
        },
      })
    ).status,
  ).toBe(201);
  for (let i = 0; i < 101; i++) {
    if (i > 0 && i % 20 === 0) await page.waitForTimeout(1000);
    expect(
      (
        await post(page, base + `/spaces/${space}/apps`, {
          id: crypto.randomUUID(),
          slug: `delegated-${i}`,
          display_name: `Delegated ${String(i).padStart(3, "0")}`,
          initial_manager_user_id: outsider.id,
        })
      ).status,
    ).toBe(201);
  }
  expect(
    (
      await post(page, base + `/spaces/${space}/apps`, {
        id: crypto.randomUUID(),
        slug: "private",
        display_name: "Private sibling",
        initial_manager_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page, "outsider");
  await selectTenant(page, tenant);
  await expect(page.locator(".app-card")).toHaveCount(6);
  expect(
    (await (await page.request.get(base + "/spaces")).json()).items,
  ).toHaveLength(0);
  expect((await page.request.get(base + "/spaces/" + space)).status()).toBe(
    403,
  );
  await page
    .getByRole("button", { name: "View all Apps", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "All Apps", exact: true, level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("row").filter({ hasText: "Delegated 000" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Next records", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "Delegated 000" }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill("Delegated 000");
  await expect(
    page.getByRole("button", { name: "Previous records", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Next records", exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("row")
    .filter({ hasText: "Delegated 000" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Delegated 000", exact: true, level: 1 }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Micro-apps", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Register micro-app", exact: true }),
  ).toBeVisible();
  expect(
    (await (await page.request.get(base + "/apps?q=Private%20sibling")).json())
      .items,
  ).toHaveLength(0);
});

test("expired sessions return to login and signing out an expired session succeeds", async ({
  page,
  context,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page, "tenant-admin");
  await context.clearCookies();
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Home", exact: true }),
  ).toHaveCount(0);
  // Complete another login directly from the recovered UI, without a reload.
  await page.getByRole("link", { name: "Sign in to Lastapp" }).click();
  await page.locator("select[name=subject]").selectOption("tenant-admin");
  await page.getByLabel("Password", { exact: true }).fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Home", exact: true }),
  ).toBeVisible();
  await context.clearCookies();
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Home", exact: true }),
  ).toHaveCount(0);
  expect(errors).toEqual([]);
});
