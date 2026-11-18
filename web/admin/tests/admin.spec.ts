import { test, expect } from "@playwright/test";
import {
  login,
  logout,
  nav,
  choose,
  post,
  fixtureTenant,
  selectTenant,
  pick,
  homeHeading,
} from "./helpers";

test("Tenant creation and one-step administrator management lead to a complete Tenant workspace", async ({
  page,
  browser,
}) => {
  await login(page);
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const suffix = crypto.randomUUID(),
    name = "Simple " + suffix,
    email = "simple-" + suffix + "@example.test",
    password = "Init8!ab",
    personal = "Own8!xyz";
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByLabel("Account name", { exact: true }).fill("Ada Owner");
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Initial password", { exact: true }).fill(password);
  await expect(
    page.getByLabel("Initial password", { exact: true }),
  ).toHaveJSProperty("validationMessage", "");
  await expect(page.getByRole("dialog")).toContainText("8–128 characters.");
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .last()
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const tenant = (
    await (
      await page.request.get("/admin/v1/tenants?q=" + encodeURIComponent(name))
    ).json()
  ).items[0].id;
  await page
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill(name);
  await page
    .getByRole("row")
    .filter({ hasText: name })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await expect(page).toHaveURL(
    new RegExp("/platform/tenants/" + tenant + "/overview"),
  );
  await page.getByRole("tab", { name: "Administrators", exact: true }).click();
  await expect(page.getByRole("row").filter({ hasText: email })).toBeVisible();
  await page
    .getByRole("button", { name: "Add administrator", exact: true })
    .click();
  await choose(page, "Existing account", "Outside user", "Outside user");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Add administrator", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "Outside user" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Add administrator", exact: true })
    .click();
  await pick(page.getByLabel("Account", { exact: true }), "new");
  await page
    .getByLabel("Account name", { exact: true })
    .fill("Second administrator");
  const secondEmail = "second-" + suffix + "@example.test";
  await page.getByLabel("Email", { exact: true }).fill(secondEmail);
  await page.getByLabel("Initial password", { exact: true }).fill(password);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Add administrator", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: secondEmail }),
  ).toBeVisible();
  await page.screenshot({
    path: "../../artifacts/simple-administrators.png",
    fullPage: true,
  });
  const context = await browser.newContext({
      viewport: { width: 1440, height: 1000 },
    }),
    admin = await context.newPage();
  const errors: string[] = [];
  admin.on("pageerror", (e) => errors.push(e.message));
  await admin.goto("/console/");
  await admin.getByLabel("Email", { exact: true }).fill(email);
  await admin.getByLabel("Password", { exact: true }).fill(password);
  await admin.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    admin.getByRole("heading", { name: "Set your own password" }),
  ).toBeVisible();
  await admin.getByLabel("Current password", { exact: true }).fill("wrong");
  await admin.getByLabel("New password", { exact: true }).fill(personal);
  await expect(
    admin.getByLabel("New password", { exact: true }),
  ).toHaveJSProperty("validationMessage", "");
  await admin
    .getByLabel("Confirm new password", { exact: true })
    .fill(personal);
  await admin
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  await expect(admin.getByRole("alert")).toContainText(
    "Email or password is incorrect",
  );
  await admin.getByLabel("Current password", { exact: true }).fill(password);
  await admin
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  await expect(homeHeading(admin)).toBeVisible();
  await admin
    .getByRole("button", { name: "Create Space", exact: true })
    .click();
  await expect(admin.getByLabel("Slug", { exact: true })).not.toBeVisible();
  await admin.getByLabel("Name", { exact: true }).fill("Sales");
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Create Space", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await nav(admin, "Spaces");
  await admin
    .getByRole("row")
    .filter({ hasText: "Sales" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await expect(admin).toHaveURL(/\/spaces\/.+\/apps$/);
  const spaceURL = admin.url();
  await admin.reload();
  await expect(
    admin.getByRole("heading", { name: "Sales", level: 1, exact: true }),
  ).toBeVisible();
  await admin.getByRole("button", { name: "Create App", exact: true }).click();
  await admin.getByLabel("Name", { exact: true }).fill("CRM");
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Create App", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin
    .getByRole("row")
    .filter({ hasText: "CRM" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await expect(admin).toHaveURL(/\/apps\/.+\/microapps$/);
  await admin
    .getByRole("button", { name: "Add micro-app", exact: true })
    .click();
  await admin.getByLabel("Name", { exact: true }).fill("Deal review");
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Add micro-app", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await expect(
    admin.getByRole("row").filter({ hasText: "Deal review" }),
  ).toBeVisible();
  await admin.goBack();
  await expect(admin).toHaveURL(spaceURL);
  await admin.goForward();
  await expect(admin).toHaveURL(/\/microapps$/);
  await expect(
    admin.getByRole("row").filter({ hasText: "Deal review" }),
  ).toBeVisible();
  await admin.screenshot({
    path: "../../artifacts/simple-app.png",
    fullPage: true,
  });
  await nav(admin, "Spaces");
  await admin
    .getByRole("row")
    .filter({ hasText: "Sales" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await admin
    .locator(".workspace-tabs")
    .getByRole("tab", { name: "Settings", exact: true })
    .click();
  await expect(admin.locator(".workspace-facts")).toContainText("ready", {
    timeout: 30000,
  });
  await admin.setViewportSize({ width: 390, height: 844 });
  await expect(admin.locator(".main")).toHaveCSS("margin-left", "0px");
  await expect(admin.locator("aside.sidebar")).toBeHidden();
  expect(
    await admin.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await admin.screenshot({
    path: "../../artifacts/simple-tenant-mobile.png",
    fullPage: true,
    animations: "disabled",
  });
  expect(errors).toEqual([]);
  await context.close();
});

test("People combines Tenant administration and team membership/roles, retaining stale edit protection", async ({
  page,
}) => {
  await login(page);
  const { tenant, base, outsider } = await fixtureTenant(page, "People");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  expect(
    (await post(page, base + "/memberships", { user_id: outsider.id })).status,
  ).toBe(201);
  await nav(page, "Access");
  await page
    .getByRole("row")
    .filter({ hasText: "Outside user" })
    .getByRole("checkbox", { name: /Tenant administrator/ })
    .click();
  await expect(
    page
      .getByRole("row")
      .filter({ hasText: "Outside user" })
      .getByRole("checkbox"),
  ).toBeChecked();
  await page.getByRole("tab", { name: "Teams", exact: true }).click();
  await page.getByRole("button", { name: "Create team", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Marketing");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Create team", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page
    .getByRole("row")
    .filter({ hasText: "Marketing" })
    .getByRole("button", { name: "Open team", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Add team member", exact: true })
    .click();
  await choose(page, "Person", "Outside user", "Outside user");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Add team member", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page
    .getByRole("checkbox", { name: "Reviewer for Outside user", exact: true })
    .click();
  await expect(
    page.getByRole("checkbox", {
      name: "Reviewer for Outside user",
      exact: true,
    }),
  ).toBeChecked();
  await page.getByRole("button", { name: "Edit team", exact: true }).click();
  const group = (
    await (await page.request.get(base + "/teams?q=Marketing")).json()
  ).items[0];
  expect(
    (
      await post(
        page,
        base + "/teams/" + group.id,
        { display_name: "Concurrent Marketing", state: "disabled" },
        "PATCH",
        group.revision,
      )
    ).status,
  ).toBe(200);
  await page
    .getByRole("button", { name: "Refresh", exact: true })
    .dispatchEvent("click");
  await expect(
    page.getByRole("heading", { name: /Concurrent Marketing/ }),
  ).toBeVisible();
  await page.getByLabel("Name", { exact: true }).fill("Stale edit");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Save", exact: true })
    .click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText(
    /changed|revision|refresh/i,
  );
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Cancel", exact: true })
    .click();
  await page.getByRole("button", { name: "Edit team", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Marketing restored");
  await pick(page.getByLabel("Status", { exact: true }), "active");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Save", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.screenshot({
    path: "../../artifacts/simple-team.png",
    fullPage: true,
  });
});

test("Platform settings and direct user administration retain authorization guards", async ({
  page,
}) => {
  await login(page);
  await nav(page, "Users");
  const me = await (await page.request.get("/v1/me")).json();
  await page
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill(me.email || me.display_name);
  await page
    .getByRole("checkbox", {
      name: "Platform administrator for " + me.display_name,
      exact: true,
    })
    .click();
  await expect(page.getByRole("alert")).toContainText(/administrator/i);
  expect(
    (
      await page.request.post("/admin/v1/users", {
        data: {
          display_name: "No CSRF",
          email: "csrf@example.test",
          password: "An invalid request password!",
        },
      })
    ).status(),
  ).toBe(403);
  await nav(page, "Settings");
  await expect(
    page.getByRole("button", { name: "Add database server", exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Plans", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "Development" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Create plan", exact: true }),
  ).toBeVisible();
});
