import { test, expect, type Locator } from "@playwright/test";
import {
  fixtureTenant,
  login,
  logout,
  menu,
  nav,
  post,
  pick,
  homeHeading,
} from "./helpers";

test("all password forms and generation consume the server policy, including Unicode and focus updates", async ({
  page,
}) => {
  let policy = {
    min_length: 9,
    max_length: 12,
    length_hint: "9–12 characters.",
  };
  await page.route("**/v1/config", async (route) => {
    const response = await route.fetch();
    await route.fulfill({
      response,
      json: { ...(await response.json()), password_policy: policy },
    });
  });
  await login(page);
  const { tenant } = await fixtureTenant(page, "Password policy");

  async function checkInput(input: Locator) {
    await input.fill("a".repeat(policy.min_length - 1));
    await expect(input).toHaveJSProperty(
      "validationMessage",
      `Use a password of ${policy.length_hint}`,
    );
    await input.fill("😀".repeat(policy.min_length));
    await expect(input).toHaveJSProperty("validationMessage", "");
    await input.fill("😀".repeat(policy.max_length + 1));
    await expect(input).toHaveJSProperty(
      "validationMessage",
      `Use a password of ${policy.length_hint}`,
    );
    // No stale native limit or UTF-16 truncation may override the server policy.
    await expect(input).not.toHaveAttribute("minlength");
    await expect(input).not.toHaveAttribute("maxlength");
  }

  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toContainText(policy.length_hint);
  await checkInput(page.getByLabel("Initial password", { exact: true }));
  await page
    .getByRole("button", { name: "Generate password", exact: true })
    .click();
  await expect(
    page.getByLabel("Initial password", { exact: true }),
  ).toHaveValue(/^[0-9a-f]{12}$/);
  await expect(
    page.getByLabel("Initial password", { exact: true }),
  ).toHaveJSProperty("validationMessage", "");
  await page.getByLabel("Close dialog", { exact: true }).click();

  await nav(page, "Users");
  await page
    .getByRole("button", { name: "Create account", exact: true })
    .click();
  await checkInput(page.getByLabel("Initial password", { exact: true }));
  // A policy refresh must update an OPEN form, preserving its current draft.
  const input = page.getByLabel("Initial password", { exact: true });
  await input.fill("abcdefgh");
  policy = { min_length: 8, max_length: 64, length_hint: "8–64 characters." };
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect(page.getByRole("dialog")).toContainText(policy.length_hint);
  await expect(input).toHaveValue("abcdefgh");
  await expect(input).toHaveJSProperty("validationMessage", "");
  await page.getByLabel("Close dialog", { exact: true }).click();

  await page.goto(`/console/platform/tenants/${tenant}/administrators`);
  await page
    .getByRole("button", { name: "Add administrator", exact: true })
    .click();
  await pick(page.getByLabel("Account", { exact: true }), "new");
  await checkInput(page.getByLabel("Initial password", { exact: true }));
  await page.getByLabel("Close dialog", { exact: true }).click();

  // Render the shared change-password controls without changing any credentials.
  let required = true;
  await page.route("**/v1/me", async (route) => {
    const response = await route.fetch();
    await route.fulfill({
      response,
      json: {
        ...(await response.json()),
        auth_method: "password",
        password_change_required: required,
      },
    });
  });
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Set your own password" }),
  ).toBeVisible();
  await checkInput(page.getByLabel("New password", { exact: true }));
  await expect(
    page.getByLabel("Current password", { exact: true }),
  ).not.toHaveAttribute("minlength");
  required = false;
  await page.reload();
  await menu(page);
  await page.getByRole("menuitem", { name: "My account", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Change password" }),
  ).toBeVisible();
  await checkInput(page.getByLabel("New password", { exact: true }));
});

test("missing server policy has an explicit retry and no hardcoded fallback", async ({
  page,
}) => {
  let available = false;
  await page.route("**/v1/config", async (route) => {
    const response = await route.fetch();
    const config = await response.json();
    if (!available) delete config.password_policy;
    await route.fulfill({ response, json: config });
  });
  await page.goto("/console/");
  await expect(page.getByRole("alert")).toContainText("valid password policy");
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toHaveCount(0);
  available = true;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("sign-in refreshes an older policy before initial-password replacement", async ({
  page,
}) => {
  await login(page);
  const id = crypto.randomUUID();
  const email = `policy-${id}@example.test`;
  const initial = "Init8!ab",
    personal = "Own8!xyz";
  expect(
    (
      await post(page, "/admin/v1/tenants", {
        id,
        slug: "policy-" + id,
        display_name: "Policy refresh " + id,
        plan_id: "10000000-0000-4000-8000-000000000001",
        initial_admin: {
          display_name: "Policy test owner",
          email,
          password: initial,
        },
      })
    ).status,
  ).toBe(201);
  await logout(page);
  const config = await (await page.request.get("/v1/config")).json();
  let policy = {
    min_length: 15,
    max_length: 128,
    length_hint: "15–128 characters.",
  };
  await page.route("**/v1/config", async (route) => {
    await route.fulfill({ json: { ...config, password_policy: policy } });
  });
  await page.reload();
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill(initial);
  // The server changes policy while the same login page is already open.
  policy = config.password_policy;
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Set your own password" }),
  ).toBeVisible();
  await expect(page.locator(".password-welcome")).toContainText(
    policy.length_hint,
  );
  await page.getByLabel("Current password", { exact: true }).fill(initial);
  await page.getByLabel("New password", { exact: true }).fill(personal);
  await page.getByLabel("Confirm new password", { exact: true }).fill(personal);
  await expect(
    page.getByLabel("New password", { exact: true }),
  ).toHaveJSProperty("validationMessage", "");
  await page.screenshot({
    path: "../../artifacts/password-policy-onboarding.png",
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Save password", exact: true })
    .click();
  await expect(homeHeading(page)).toBeVisible();
});
