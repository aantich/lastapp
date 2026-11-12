import { test, expect } from "@playwright/test";
import { login, post, selectTenant, fixtureTenant } from "./helpers";

test("superadmin creates an email/password admin who builds a complete Tenant workspace", async ({
  page,
  browser,
}) => {
  const errors: string[] = [];
  await login(page);
  await page.getByRole("button", { name: "Tenants", exact: true }).click();
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const id = crypto.randomUUID(),
    email = `ui-owner-${id}@example.test`;
  const temporary = "Initial password for the UI owner!",
    personal = "My personal Lastapp passphrase!";
  await page
    .getByLabel("Name", { exact: true })
    .fill("UI Company " + id.slice(0, 8));
  await page.getByLabel("Slug", { exact: true }).fill("company-" + id);
  await page
    .getByLabel("Plan", { exact: true })
    .selectOption("10000000-0000-4000-8000-000000000001");
  await page
    .getByLabel("Administrator name", { exact: true })
    .fill("Taylor Owner");
  await page.getByLabel("Administrator email", { exact: true }).fill(email);
  await page.getByLabel("Initial password", { exact: true }).fill(temporary);
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const tenant = (
    await (await page.request.get("/admin/v1/tenants?q=company-" + id)).json()
  ).items[0].id;
  const members = (
    await (await page.request.get(`/v1/tenants/${tenant}/memberships`)).json()
  ).items;
  await expect(
    page.locator("nav").getByRole("button", { name: "Spaces", exact: true }),
  ).toHaveCount(0);
  expect(
    (
      await post(page, `/v1/tenants/${tenant}/spaces`, {
        id: crypto.randomUUID(),
        slug: "forbidden",
        display_name: "Forbidden",
        initial_manager_user_id: members[0].user_id,
        database: { mode: "managed", layout: "shared" },
      })
    ).status,
  ).toBe(403);

  const context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
  });
  const admin = await context.newPage();
  admin.on("pageerror", (e) => errors.push(e.message));
  await admin.goto("/console/");
  await admin.getByLabel("Email", { exact: true }).fill(email);
  await admin.getByLabel("Password", { exact: true }).fill(temporary);
  await admin.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    admin.getByRole("heading", { name: "Set your own password" }),
  ).toBeVisible();
  expect(
    (await admin.request.get(`/v1/tenants/${tenant}/overview`)).status(),
  ).toBe(403);
  await admin
    .getByLabel("Current password", { exact: true })
    .fill("Wrong current password");
  await admin.getByLabel("New password", { exact: true }).fill(personal);
  await admin
    .getByLabel("Confirm new password", { exact: true })
    .fill(personal);
  await admin
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  await expect(admin.getByRole("alert")).toContainText(
    "Email or password is incorrect.",
  );
  await expect(
    admin.getByRole("heading", { name: "Set your own password" }),
  ).toBeVisible();
  await admin.getByLabel("Current password", { exact: true }).fill(temporary);
  await admin
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  await expect(
    admin.getByRole("heading", { name: "Home", exact: true }),
  ).toBeVisible();
  await expect(
    admin.getByRole("combobox", { name: "Tenant", exact: true }),
  ).toHaveCount(0);
  await expect(
    admin.locator("nav").getByRole("button", { name: "Tenants", exact: true }),
  ).toHaveCount(0);
  await admin
    .getByRole("button", { name: "Create your first Space", exact: true })
    .click();
  await admin.getByLabel("Name", { exact: true }).fill("Sales");
  await admin
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin.getByRole("button", { name: "Spaces", exact: true }).click();
  await admin
    .getByRole("row")
    .filter({ hasText: "Sales" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await admin.getByRole("button", { name: "Database", exact: true }).click();
  await expect(
    admin.locator(".workspace-facts").getByText("ready", { exact: true }),
  ).toBeVisible({ timeout: 30000 });
  await admin.getByRole("button", { name: "Apps", exact: true }).click();
  await admin.getByRole("button", { name: "Create App", exact: true }).click();
  await admin.getByLabel("Name", { exact: true }).fill("CRM");
  await admin
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin
    .getByRole("row")
    .filter({ hasText: "CRM" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await admin.getByRole("button", { name: "Micro-apps", exact: true }).click();
  await admin
    .getByRole("button", { name: "Register micro-app", exact: true })
    .click();
  await admin.getByLabel("Name", { exact: true }).fill("Deal review");
  await admin
    .getByLabel("Artifact reference", { exact: true })
    .fill("library/deal-review@1");
  await admin
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await expect(
    admin.getByRole("row").filter({ hasText: "Deal review" }),
  ).toContainText("library/deal-review@1");
  await admin.screenshot({
    path: "../../artifacts/tenant-app-microapps.png",
    fullPage: true,
  });
  await admin.getByRole("button", { name: "Home", exact: true }).click();
  await expect(admin.locator(".app-card")).toContainText("CRM");
  await admin.screenshot({
    path: "../../artifacts/tenant-home.png",
    fullPage: true,
  });
  await admin
    .getByRole("button", { name: "Customize appearance", exact: true })
    .click();
  const appearance = admin.getByRole("dialog", {
    name: "Appearance",
    exact: true,
  });
  await appearance.getByRole("radio", { name: /Studio/ }).check();
  await appearance.getByRole("radio", { name: "Dark", exact: true }).check();
  await expect(admin.locator("html")).toHaveAttribute("data-theme", "studio");
  await expect(admin.locator("html")).toHaveAttribute("data-mode", "dark");
  await appearance.getByRole("button", { name: "Done", exact: true }).click();
  await admin.screenshot({
    path: "../../artifacts/tenant-home-dark.png",
    fullPage: true,
  });
  await admin
    .getByRole("button", { name: "Customize appearance", exact: true })
    .click();
  await appearance
    .getByRole("button", { name: "Reset to default", exact: true })
    .click();
  await appearance.getByRole("button", { name: "Done", exact: true }).click();
  await admin.setViewportSize({ width: 390, height: 844 });
  await admin
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  const nav = admin.getByRole("dialog", { name: "Tenant navigation" });
  await nav.getByRole("button", { name: "People", exact: true }).click();
  await expect(nav).toHaveCount(0);
  await expect
    .poll(async () =>
      admin
        .locator("#tenant-navigation")
        .evaluate((el) => el.getBoundingClientRect().right),
    )
    .toBeLessThanOrEqual(0);
  await expect(
    admin.getByRole("row").filter({ hasText: "Taylor Owner" }),
  ).toBeVisible();
  expect(
    await admin.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await admin.screenshot({
    path: "../../artifacts/tenant-mobile.png",
    fullPage: true,
  });
  expect(errors).toEqual([]);
  await context.close();
});

test("OIDC roles, CSRF and last-superadmin protection remain enforced", async ({
  page,
  browser,
}) => {
  await login(page);
  const response = await page.request.post("/admin/v1/tenants", { data: {} });
  expect(response.status()).toBe(403);
  const grants = (
    await (await page.request.get("/admin/v1/role-assignments")).json()
  ).items;
  const me = await (await page.request.get("/v1/me")).json();
  const own = grants.find((g: any) => g.user_id === me.id);
  expect(
    (
      await post(
        page,
        "/admin/v1/role-assignments/" + own.id,
        undefined,
        "DELETE",
        own.revision,
      )
    ).status,
  ).toBe(409);
  const context = await browser.newContext();
  const outside = await context.newPage();
  await login(outside, "outsider");
  expect((await outside.request.get("/admin/v1/tenants")).status()).toBe(403);
  expect((await outside.request.get("/admin/v1/users")).status()).toBe(403);
  await context.close();
});

test("team editing preserves its original revision through background refreshes", async ({
  page,
  browser,
}) => {
  await login(page);
  const { tenant, base } = await fixtureTenant(page, "Concurrent editing");
  const context = await browser.newContext();
  const admin = await context.newPage();
  await login(admin, "tenant-admin");
  await selectTenant(admin, tenant);
  const team = crypto.randomUUID(),
    path = `${base}/teams/${team}`;
  expect(
    (
      await post(admin, base + "/teams", {
        id: team,
        slug: "review-team",
        display_name: "Original team",
      })
    ).status,
  ).toBe(201);
  await admin.getByRole("button", { name: "People", exact: true }).click();
  await admin.getByRole("button", { name: "Teams", exact: true }).click();
  await admin
    .getByRole("combobox", { name: "Team scope", exact: true })
    .selectOption(team);
  await admin.getByRole("button", { name: "Edit team", exact: true }).click();
  const form = admin.getByRole("form", { name: "Edit team", exact: true });
  await form.getByLabel("Name", { exact: true }).fill("My draft rename");
  expect(
    (
      await post(
        admin,
        path,
        { display_name: "Concurrent rename", state: "disabled" },
        "PATCH",
        1,
      )
    ).status,
  ).toBe(200);
  const refreshed = admin.waitForResponse(
    (r) => new URL(r.url()).pathname === path + "/members",
  );
  await admin
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill("refresh");
  await refreshed;
  await expect(admin.getByText("0 records", { exact: true })).toBeVisible();
  const saved = admin.waitForResponse(
    (r) =>
      new URL(r.url()).pathname === path && r.request().method() === "PATCH",
  );
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  const stale = await saved;
  expect(stale.request().headers()["if-match"]).toBe("1");
  expect(stale.status()).toBe(412);
  await expect(form).toBeVisible();
  await expect(admin.getByRole("alert")).toContainText("This resource changed");
  expect(await (await admin.request.get(path)).json()).toMatchObject({
    display_name: "Concurrent rename",
    state: "disabled",
    revision: 2,
  });
  await form.getByRole("button", { name: "Cancel", exact: true }).click();
  await admin.getByRole("button", { name: "Edit team", exact: true }).click();
  await expect(form.getByLabel("State", { exact: true })).toHaveValue(
    "disabled",
  );
  await form.getByLabel("Name", { exact: true }).fill("Reviewed rename");
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  expect(await (await admin.request.get(path)).json()).toMatchObject({
    display_name: "Reviewed rename",
    state: "disabled",
    revision: 3,
  });
  await context.close();
});

test("Tenant admin registers an existing database and manages members, teams and Space roles", async ({
  page,
  browser,
}) => {
  const { execFileSync } = await import("node:child_process");
  const { randomBytes } = await import("node:crypto");
  const suffix = Date.now().toString(),
    dbName = "ui_" + suffix,
    password = randomBytes(24).toString("hex"),
    accountHost = process.env.CI ? "%" : "127.0.0.1";
  const args = process.env.CI
    ? ["--protocol=TCP", "--host=127.0.0.1", "--port=33317"]
    : ["--protocol=SOCKET", "--socket=../../.local/mariadb.sock"];
  execFileSync("mariadb", ["--no-defaults", ...args, "-uroot"], {
    input: `CREATE DATABASE ${dbName}; CREATE TABLE ${dbName}.existing_data(id INT PRIMARY KEY); INSERT INTO ${dbName}.existing_data VALUES(7); CREATE USER '${dbName}'@'${accountHost}' IDENTIFIED BY '${password}'; GRANT SELECT,INSERT,UPDATE,DELETE ON \`ui\\_${suffix}\`.* TO '${dbName}'@'${accountHost}';`,
    stdio: ["pipe", "pipe", "pipe"],
  });
  await login(page);
  const { tenant, base, outsider } = await fixtureTenant(
    page,
    "Access management",
  );
  const context = await browser.newContext();
  const admin = await context.newPage();
  await login(admin, "tenant-admin");
  await selectTenant(admin, tenant);
  await admin.getByRole("button", { name: "Settings", exact: true }).click();
  await admin.getByRole("button", { name: "Databases", exact: true }).click();
  await admin
    .getByRole("button", { name: "Add database", exact: true })
    .click();
  await admin.getByLabel("Method", { exact: true }).selectOption("registered");
  await admin
    .getByLabel("Existing database name", { exact: true })
    .fill(dbName);
  await admin
    .getByLabel("Existing database username", { exact: true })
    .fill(dbName);
  await admin
    .getByLabel("Existing database password", { exact: true })
    .fill(password);
  await admin
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await expect(
    admin
      .getByRole("row")
      .filter({ hasText: dbName })
      .getByText("ready", { exact: true }),
  ).toBeVisible({ timeout: 30000 });
  const marker = execFileSync(
    "mariadb",
    [
      "--no-defaults",
      ...args,
      "-uroot",
      "-N",
      "-e",
      `SELECT id FROM ${dbName}.existing_data`,
    ],
    { encoding: "utf8" },
  );
  expect(marker.trim()).toBe("7");
  await admin.getByRole("button", { name: "People", exact: true }).click();
  await admin.getByRole("button", { name: "Add member", exact: true }).click();
  await admin.getByLabel("Account", { exact: true }).selectOption("id");
  await admin.getByLabel("User ID", { exact: true }).fill(outsider.id);
  await admin
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin.getByRole("button", { name: "Teams", exact: true }).click();
  await admin.getByRole("button", { name: "Create team", exact: true }).click();
  let form = admin.getByRole("form", { name: "Create team", exact: true });
  await form.getByLabel("Name", { exact: true }).fill("Sales team");
  await form.getByLabel("Slug", { exact: true }).fill("sales-team");
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await admin
    .getByRole("button", { name: "Add team member", exact: true })
    .click();
  form = admin.getByRole("form", { name: "Add team member", exact: true });
  await form
    .getByRole("combobox", { name: "Tenant member", exact: true })
    .selectOption(outsider.id);
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await admin.getByRole("button", { name: "Assign role", exact: true }).click();
  form = admin.getByRole("form", { name: "Assign role", exact: true });
  await form
    .getByLabel("Role", { exact: true })
    .selectOption("00000000-0000-4000-8000-000000000004");
  await form.getByLabel("User", { exact: true }).selectOption(outsider.id);
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  const member = admin
    .locator(".access-members")
    .getByRole("row")
    .filter({ hasText: "Outside user" });
  await member.getByRole("button", { name: "Disable", exact: true }).click();
  await expect(
    member.getByRole("button", { name: "Enable", exact: true }),
  ).toBeVisible();
  await member.getByRole("button", { name: "Enable", exact: true }).click();
  await admin.getByRole("button", { name: "Spaces", exact: true }).click();
  await admin
    .getByRole("button", { name: "Create Space", exact: true })
    .click();
  await admin.getByLabel("Name", { exact: true }).fill("Sales");
  await admin.getByLabel("Database", { exact: true }).selectOption("existing");
  const resources = (
    await (await admin.request.get(base + "/database-resources")).json()
  ).items;
  const resource = resources.find((d: any) => d.database_name === dbName);
  await admin
    .getByLabel("Existing database", { exact: true })
    .selectOption(resource.id);
  await admin
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin
    .getByRole("row")
    .filter({ hasText: "Sales" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await admin.getByRole("button", { name: "Access", exact: true }).click();
  await admin.getByRole("button", { name: "Roles", exact: true }).click();
  await admin.getByRole("button", { name: "Create role", exact: true }).click();
  form = admin.getByRole("form", { name: "Create role", exact: true });
  await form.getByLabel("Name", { exact: true }).fill("Sales observer");
  await form.getByLabel("Code", { exact: true }).fill("sales-observer");
  await expect(form.getByRole("checkbox")).toHaveCount(0);
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await expect(
    admin.getByRole("row").filter({ hasText: "Sales observer" }),
  ).toBeVisible();
  await admin.screenshot({
    path: "../../artifacts/tenant-space-access.png",
    fullPage: true,
  });
  await admin.getByRole("button", { name: "Apps", exact: true }).click();
  await admin.getByRole("button", { name: "Create App", exact: true }).click();
  await admin.getByLabel("Name", { exact: true }).fill("Sales CRM");
  await admin
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin
    .getByRole("row")
    .filter({ hasText: "Sales CRM" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await admin.getByRole("button", { name: "Access", exact: true }).click();
  await admin.getByRole("button", { name: "Roles", exact: true }).click();
  await admin.getByRole("button", { name: "Create role", exact: true }).click();
  form = admin.getByRole("form", { name: "Create role", exact: true });
  await form.getByLabel("Name", { exact: true }).fill("App analyst");
  await form.getByLabel("Code", { exact: true }).fill("app-analyst");
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await expect(
    admin.getByRole("row").filter({ hasText: "App analyst" }),
  ).toBeVisible();
  await admin.screenshot({
    path: "../../artifacts/tenant-app-access.png",
    fullPage: true,
  });
  // A delayed form preparation must not follow the user into another Tenant.
  const other = await fixtureTenant(page, "Other workspace");
  let release!: () => void, intercepted!: () => void;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const started = new Promise<void>((resolve) => {
    intercepted = resolve;
  });
  await admin.route(
    "**/v1/tenants/" + tenant + "/database-targets",
    async (route) => {
      intercepted();
      await held;
      await route.continue();
    },
  );
  await admin.getByRole("button", { name: "Spaces", exact: true }).click();
  await admin
    .getByRole("button", { name: "Create Space", exact: true })
    .click();
  await started;
  await selectTenant(admin, other.tenant);
  const completed = admin.waitForResponse(
    (r) => new URL(r.url()).pathname === base + "/database-targets",
  );
  release();
  const oldTenantResponse = await completed;
  expect(oldTenantResponse.status()).toBe(200);
  await oldTenantResponse.finished();
  await admin.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
  await expect(
    admin.getByRole("heading", { name: "Home", exact: true }),
  ).toBeVisible();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await context.close();
});
