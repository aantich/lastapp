import { test, expect, Page } from "@playwright/test";
async function login(page: Page, subject = "operator") {
  await page.goto("/console/");
  await page.getByRole("link", { name: "Sign in to Lastapp" }).click();
  await expect(page).toHaveURL(/8788\/authorize/);
  await page.locator("select[name=subject]").selectOption(subject);
  await page.getByLabel("Password").fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(/8787\/console\//);
  await expect(
    page.getByRole("heading", { name: /Overview|Spaces/, exact: true }),
  ).toBeVisible();
}
async function post(
  page: Page,
  path: string,
  data: any,
  method = "POST",
  revision?: number,
) {
  return page.evaluate(
    async ({ path, data, method, revision }) => {
      const csrf = document.cookie
        .split("; ")
        .find((s) => s.startsWith("lastapp_csrf="))!
        .split("=")[1];
      const r = await fetch(path, {
        method,
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrf,
          ...(revision ? { "If-Match": String(revision) } : {}),
        },
        ...(data ? { body: JSON.stringify(data) } : {}),
      });
      return { status: r.status, body: await r.json() };
    },
    { path, data, method, revision },
  );
}
test("admin provisions a complete Tenant hierarchy through the console", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page);
  await page.getByRole("button", { name: "Tenants", exact: true }).click();
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const suffix = Date.now().toString().slice(-7);
  await page.getByLabel("Name", { exact: true }).fill("Northstar " + suffix);
  await page.getByLabel("Slug", { exact: true }).fill("northstar-" + suffix);
  await page
    .getByLabel("Plan", { exact: true })
    .selectOption("10000000-0000-4000-8000-000000000001");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const tenantRow = page
    .getByRole("row")
    .filter({ hasText: "Northstar " + suffix });
  await expect(tenantRow).toBeVisible();
  await tenantRow.getByRole("button", { name: "Open" }).click();
  await page.getByRole("button", { name: "Create Space", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Sales");
  await page.getByLabel("Slug", { exact: true }).fill("sales");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("row")
      .filter({ hasText: "Sales" })
      .getByText("ready", { exact: true }),
  ).toBeVisible({ timeout: 30000 });
  await page.getByRole("button", { name: "Apps", exact: true }).click();
  await page.getByRole("button", { name: "Create App", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Custom CRM");
  await page.getByLabel("Slug", { exact: true }).fill("crm");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "Custom CRM" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Micro-apps", exact: true }).click();
  await page
    .getByRole("button", { name: "Register micro-app", exact: true })
    .click();
  await page.getByLabel("Name", { exact: true }).fill("Deal review");
  await page.getByLabel("Slug", { exact: true }).fill("deal-review");
  await page
    .getByLabel("Artifact reference", { exact: true })
    .fill("library:crm/deal-review@1");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "Deal review" }),
  ).toBeVisible();
  await page.screenshot({
    path: "../../artifacts/admin-microapps.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Databases", exact: true }).click();
  await expect(page.getByText("ready", { exact: true })).toBeVisible();
  await page.screenshot({
    path: "../../artifacts/admin-databases.png",
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
test("OIDC user without grants cannot administer the platform; CSRF and stale revisions are rejected", async ({
  page,
  browser,
}) => {
  await login(page);
  const users = (await page.request.get("/admin/v1/users")).json();
  const me = await (await page.request.get("/v1/me")).json();
  const denied = await page.evaluate(async () => {
    const r = await fetch("/admin/v1/tenants", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
    return r.status;
  });
  expect(denied).toBe(403);
  const supergrants = await (
    await page.request.get("/admin/v1/role-assignments")
  ).json();
  const own = supergrants.items.find((g: any) => g.user_id === me.id);
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
  const plan = (await (await page.request.get("/admin/v1/plans")).json())
    .items[0];
  const stale = await post(
    page,
    "/admin/v1/plans/" + plan.id,
    {
      ...Object.fromEntries(
        Object.entries(plan).filter(
          ([k]) => !["revision", "created_at"].includes(k),
        ),
      ),
      allow_shared_database: !!plan.allow_shared_database,
      allow_dedicated_database: !!plan.allow_dedicated_database,
      allow_managed_database: !!plan.allow_managed_database,
      allow_registered_database: !!plan.allow_registered_database,
      allow_dedicated_compute: !!plan.allow_dedicated_compute,
    },
    "PATCH",
    999999,
  );
  expect(stale.status).toBe(412);
  await users;
  const context = await browser.newContext();
  const outsider = await context.newPage();
  await login(outsider, "outsider");
  expect((await outsider.request.get("/admin/v1/tenants")).status()).toBe(403);
  expect((await outsider.request.get("/admin/v1/users")).status()).toBe(403);
  await context.close();
});
test("management screens support existing database registration and Tenant access administration", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  const fs = await import("node:fs/promises");
  const { execFileSync } = await import("node:child_process");
  const { randomBytes } = await import("node:crypto");
  const suffix = Date.now().toString();
  const dbName = "ui_" + suffix;
  const password = randomBytes(24).toString("hex");
  const accountHost = process.env.CI ? "%" : "127.0.0.1";
  const databaseArgs = process.env.CI
    ? ["--protocol=TCP", "--host=127.0.0.1", "--port=33317"]
    : ["--protocol=SOCKET", "--socket=../../.local/mariadb.sock"];
  execFileSync("mariadb", ["--no-defaults", ...databaseArgs, "-uroot"], {
    input: `CREATE DATABASE ${dbName}; CREATE TABLE ${dbName}.existing_data (id INT PRIMARY KEY); INSERT INTO ${dbName}.existing_data VALUES (7); CREATE USER '${dbName}'@'${accountHost}' IDENTIFIED BY '${password}'; GRANT SELECT,INSERT,UPDATE,DELETE ON \`ui\\_${suffix}\`.* TO '${dbName}'@'${accountHost}';`,
    stdio: ["pipe", "pipe", "pipe"],
  });
  await login(page);
  const users = (await (await page.request.get("/admin/v1/users")).json())
    .items;
  const owner = users.find((u: any) => u.oidc_subject === "tenant-admin");
  const outsider = users.find((u: any) => u.oidc_subject === "outsider");
  const tenant = crypto.randomUUID();
  expect(
    (
      await post(page, "/admin/v1/tenants", {
        id: tenant,
        slug: "ui-" + suffix,
        display_name: "Access demo " + suffix,
        plan_id: "10000000-0000-4000-8000-000000000001",
        initial_admin_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await page.locator("#tenant-context").selectOption(tenant);
  await page.getByRole("button", { name: "Databases", exact: true }).click();
  await page.getByRole("button", { name: "Add database", exact: true }).click();
  await page.getByLabel("Method", { exact: true }).selectOption("registered");
  await page.getByLabel("Existing database name", { exact: true }).fill(dbName);
  await page
    .getByLabel("Existing database username", { exact: true })
    .fill(dbName);
  await page
    .getByLabel("Existing database password", { exact: true })
    .fill(password);
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("row")
      .filter({ hasText: dbName })
      .getByText("ready", { exact: true }),
  ).toBeVisible({ timeout: 30000 });
  const marker = execFileSync(
    "mariadb",
    [
      "--no-defaults",
      ...databaseArgs,
      "-uroot",
      "-N",
      "-e",
      `SELECT id FROM ${dbName}.existing_data`,
    ],
    { encoding: "utf8" },
  );
  expect(marker.trim()).toBe("7");
  await page.getByRole("button", { name: "Members", exact: true }).click();
  await page.getByRole("button", { name: "Add member", exact: true }).click();
  await page.getByLabel("User", { exact: true }).selectOption(outsider.id);
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page
    .getByRole("button", { name: "Access control", exact: true })
    .click();
  await page.getByRole("button", { name: "Roles", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Create role", exact: true }),
  ).toHaveCount(0);
  const resources = (
    await (
      await page.request.get(`/v1/tenants/${tenant}/database-resources`)
    ).json()
  ).items;
  const space = crypto.randomUUID(),
    app = crypto.randomUUID();
  expect(
    (
      await post(page, `/v1/tenants/${tenant}/spaces`, {
        id: space,
        slug: "sales",
        display_name: "Sales",
        initial_manager_user_id: owner.id,
        database: {
          resource_id: resources.find((r: any) => r.database_name === dbName)
            .id,
        },
      })
    ).status,
  ).toBe(201);
  expect(
    (
      await post(page, `/v1/tenants/${tenant}/spaces/${space}/apps`, {
        id: app,
        slug: "crm",
        display_name: "CRM",
        initial_manager_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  await page.getByRole("button", { name: "Teams", exact: true }).click();
  await page.getByRole("button", { name: "Create team", exact: true }).click();
  let form = page.getByRole("form", { name: "Create team", exact: true });
  await form.getByLabel("Name", { exact: true }).fill("Sales team");
  await form.getByLabel("Slug", { exact: true }).fill("sales-team");
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await page
    .getByRole("button", { name: "Add team member", exact: true })
    .click();
  form = page.getByRole("form", { name: "Add team member", exact: true });
  await form
    .getByRole("combobox", { name: "Tenant member", exact: true })
    .selectOption(outsider.id);
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await expect(
    page
      .locator(".access-members")
      .getByRole("row")
      .filter({ hasText: "Outside user" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Assign role", exact: true }).click();
  form = page.getByRole("form", { name: "Assign role", exact: true });
  await form
    .getByRole("combobox", { name: "Role", exact: true })
    .selectOption("00000000-0000-4000-8000-000000000004");
  await form
    .getByRole("combobox", { name: "User", exact: true })
    .selectOption(outsider.id);
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "Team Reviewer" }),
  ).toContainText("Outside user");
  const member = page
    .locator(".access-members")
    .getByRole("row")
    .filter({ hasText: "Outside user" });
  await member.getByRole("button", { name: "Disable", exact: true }).click();
  await expect(
    member.getByRole("button", { name: "Enable", exact: true }),
  ).toBeVisible();
  await member.getByRole("button", { name: "Enable", exact: true }).click();
  await expect(
    member.getByRole("button", { name: "Disable", exact: true }),
  ).toBeVisible();
  await page
    .locator(".access-foundation > .tabs")
    .getByRole("button", { name: "Spaces", exact: true })
    .click();
  await page
    .getByRole("combobox", { name: "Space scope", exact: true })
    .selectOption(space);
  await page.getByRole("button", { name: "Roles", exact: true }).click();
  await page.getByRole("button", { name: "Create role", exact: true }).click();
  form = page.getByRole("form", { name: "Create role", exact: true });
  await form.getByLabel("Name", { exact: true }).fill("Space observer");
  await form.getByLabel("Code", { exact: true }).fill("space-observer");
  await expect(form.getByRole("checkbox")).toHaveCount(0);
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "Space observer" }),
  ).toBeVisible();
  const definitions = (
    await (
      await page.request.get(`/v1/tenants/${tenant}/spaces/${space}/roles`)
    ).json()
  ).items;
  const custom = definitions.find((r: any) => r.code === "space-observer");
  await page
    .getByRole("button", { name: "Role assignments", exact: true })
    .click();
  await page.getByRole("button", { name: "Assign role", exact: true }).click();
  form = page.getByRole("form", { name: "Assign role", exact: true });
  await form
    .getByRole("combobox", { name: "Role", exact: true })
    .selectOption(custom.id);
  await form
    .getByRole("combobox", { name: "Recipient type", exact: true })
    .selectOption("team");
  await form
    .getByRole("combobox", { name: "Recipient team", exact: true })
    .selectOption({ label: "Sales team" });
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "Sales team" }),
  ).toContainText("Space observer");
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
  await page.screenshot({
    path: "../../artifacts/foundation-space-roles.png",
    fullPage: true,
  });
  await page
    .locator(".access-foundation > .tabs")
    .getByRole("button", { name: "Apps", exact: true })
    .click();
  await page
    .getByRole("combobox", { name: "App scope", exact: true })
    .selectOption(app);
  await page.getByRole("button", { name: "Roles", exact: true }).click();
  await page.getByRole("button", { name: "Create role", exact: true }).click();
  form = page.getByRole("form", { name: "Create role", exact: true });
  await form.getByLabel("Name", { exact: true }).fill("Forecast analyst");
  await form.getByLabel("Code", { exact: true }).fill("forecast-analyst");
  await form.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(form).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "Forecast analyst" }),
  ).toBeVisible();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
  await page.screenshot({
    path: "../../artifacts/foundation-app-roles.png",
    fullPage: true,
  });
  await page.locator("#tenant-context").selectOption("");
  await page
    .getByRole("button", { name: "Access control", exact: true })
    .click();
  await expect(
    page.getByRole("row").filter({ hasText: "Platform superadmin" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Plans", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "Development" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Database targets", exact: true })
    .click();
  await expect(
    page.getByRole("row").filter({ hasText: "Local MariaDB" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Activity", exact: true }).click();
  await expect(page.getByRole("table")).toBeVisible();
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  await expect(page.getByRole("status")).toHaveCount(0);
  await page.screenshot({
    path: "../../artifacts/admin-overview.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: "../../artifacts/admin-mobile.png",
    fullPage: true,
  });
  await fs.access("../../artifacts/admin-overview.png");
  expect(errors).toEqual([]);
});
