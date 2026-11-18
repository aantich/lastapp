import { test, expect } from "@playwright/test";
import { login, post, menu } from "./helpers";

test("shared dialogs align advanced fields and credential pairs on desktop and fit mobile", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await login(page);
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await dialog.getByText("Advanced", { exact: true }).click();
  const name = await dialog.getByLabel("Name", { exact: true }).boundingBox();
  const slug = await dialog.getByLabel("Slug", { exact: true }).boundingBox();
  expect(Math.abs(name!.x - slug!.x)).toBeLessThanOrEqual(1);
  const password = await dialog
    .getByLabel("Initial password", { exact: true })
    .boundingBox();
  const email = await dialog.getByLabel("Email", { exact: true }).boundingBox();
  expect(password!.width).toBeGreaterThan(email!.width);
  await page.keyboard.press("Escape");
  await page.goto("/console/platform/settings/servers");
  await page
    .getByRole("button", { name: "Edit server", exact: true })
    .first()
    .click();
  const username = await dialog
    .getByLabel("Provisioning username", { exact: true })
    .boundingBox();
  const secret = await dialog
    .getByLabel("Provisioning password", { exact: true })
    .boundingBox();
  expect(Math.abs(username!.y - secret!.y)).toBeLessThanOrEqual(1);
  await page.keyboard.press("Escape");
  await page.goto("/console/platform/settings/plans");
  await page.getByRole("button", { name: "Create plan", exact: true }).click();
  await dialog.getByText("Advanced", { exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".main")).toHaveCSS("margin-left", "0px");
  const bounds = await dialog.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
  expect(bounds!.height).toBeLessThanOrEqual(844);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await dialog
    .getByRole("button", { name: "Create plan", exact: true })
    .scrollIntoViewIfNeeded();
  await expect(
    dialog.getByRole("button", { name: "Create plan", exact: true }),
  ).toBeVisible();
});

test("first-login password setup is centered and account forms work before Tenant assignment", async ({
  page,
  browser,
}) => {
  await login(page);
  const email = `layout-${crypto.randomUUID()}@example.test`;
  expect(
    (
      await post(page, "/admin/v1/users", {
        display_name: "Layout account",
        email,
        password: "Initial8!password",
      })
    ).status,
  ).toBe(201);
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
  });
  const account = await context.newPage();
  const errors: string[] = [];
  account.on("pageerror", (error) => errors.push(error.message));
  await account.goto("/console/");
  await account.getByLabel("Email", { exact: true }).fill(email);
  await account
    .getByLabel("Password", { exact: true })
    .fill("Initial8!password");
  await account.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    account.getByRole("heading", { name: "Set your own password" }),
  ).toBeVisible();
  const card = await account
    .locator(".password-welcome .welcome-card")
    .boundingBox();
  expect(Math.abs(card!.x + card!.width / 2 - 720)).toBeLessThanOrEqual(1);
  await account
    .getByLabel("Current password", { exact: true })
    .fill("Initial8!password");
  await account
    .getByLabel("New password", { exact: true })
    .fill("Personal8!password");
  await account
    .getByLabel("Confirm new password", { exact: true })
    .fill("Personal8!password");
  await account
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  // No Tenant yet: the console's Home is the "No Tenant yet" page.
  await expect(
    account.getByRole("heading", { name: "Home", exact: true, level: 1 }),
  ).toBeVisible();
  await menu(account);
  await account
    .getByRole("menuitem", { name: "My account", exact: true })
    .click();
  await expect(
    account.getByRole("heading", { name: "Change password", level: 2 }),
  ).toBeVisible();
  const inputs = account.locator(".login-form input");
  const widths = await inputs.evaluateAll((elements) =>
    elements.map((el) => el.getBoundingClientRect().width),
  );
  expect(new Set(widths).size).toBe(1);
  await account.setViewportSize({ width: 390, height: 844 });
  await expect(account.locator(".main")).toHaveCSS("margin-left", "0px");
  expect(
    await account.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
  await context.close();
});
