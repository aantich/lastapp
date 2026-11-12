import { test, expect, Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/");
  await page.getByRole("link", { name: "Sign in to Lastapp" }).click();
  await page.locator("select[name=subject]").selectOption("operator");
  await page.getByLabel("Password").fill("lastapp-local");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Overview", exact: true }),
  ).toBeVisible();
}

test("appearance changes every screen, persists across reloads and resets", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 1050 });
  await login(page);
  // The signed-out /v1/me probe intentionally returns 401 before login.
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  await page
    .getByRole("button", { name: "Collapse sidebar", exact: true })
    .click();
  await expect(page.locator("#tenant-context")).toBeHidden();
  await page
    .getByRole("button", { name: "Expand sidebar", exact: true })
    .click();
  await expect(page.locator("#tenant-context")).toBeVisible();
  const original = await page
    .locator(".stat")
    .first()
    .evaluate((el) => ({
      background: getComputedStyle(el).backgroundColor,
      border: getComputedStyle(el).borderTopStyle,
    }));
  expect(original.border).toBe("dashed");
  await page.screenshot({
    path: "../../artifacts/admin-neo-overview.png",
    fullPage: true,
    animations: "disabled",
  });
  await page.getByRole("button", { name: "Customize appearance" }).click();
  const drawer = page.getByRole("dialog", { name: "Appearance", exact: true });
  await drawer.getByRole("radio", { name: /Studio/ }).check();
  await drawer.getByRole("radio", { name: "Dark", exact: true }).check();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "studio");
  const surfaceColor = await drawer.evaluate(
    (el) => getComputedStyle(el).backgroundColor,
  );
  await expect(page.locator(".stat").first()).toHaveCSS(
    "background-color",
    surfaceColor,
  );
  const changed = await page
    .locator(".stat")
    .first()
    .evaluate((el) => ({
      background: getComputedStyle(el).backgroundColor,
      border: getComputedStyle(el).borderTopStyle,
    }));
  expect(changed.background).not.toBe(original.background);
  expect(changed.border).toBe("solid");
  await drawer.getByRole("button", { name: "Done" }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "studio");
  await page.getByRole("button", { name: "Tenants", exact: true }).click();
  await expect(page.getByRole("table")).toBeVisible();
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const form = page.getByRole("dialog", { name: "Create a Tenant" });
  await expect(form).toBeVisible();
  expect(
    await form.evaluate((el) => getComputedStyle(el).backgroundColor),
  ).toBe(changed.background);
  await page.screenshot({
    path: "../../artifacts/admin-studio-dark-form.png",
    fullPage: true,
    animations: "disabled",
  });
  await page.keyboard.press("Escape");
  await expect(form).toHaveCount(0);
  await page.getByRole("button", { name: "Customize appearance" }).click();
  await drawer.getByRole("button", { name: "Reset to default" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "neo");
  await expect(page.locator("html")).toHaveAttribute("data-mode", "light");
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("button", { name: "Customize appearance" }),
  ).toBeFocused();
  expect(errors).toEqual([]);
});

test("system mode follows OS changes, cross-tab preferences sync, and invalid preferences recover", async ({
  page,
  context,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem(
      "lastapp-appearance",
      '{"theme":"unknown","mode":"invalid"}',
    ),
  );
  await page.emulateMedia({ colorScheme: "dark" });
  await login(page);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "neo");
  await expect(page.locator("html")).toHaveAttribute("data-mode", "light");
  await page.getByRole("button", { name: "Customize appearance" }).click();
  await page.getByRole("radio", { name: "System", exact: true }).check();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await expect(
    page.getByRole("button", { name: "Switch to light mode" }),
  ).toBeAttached();
  await page.emulateMedia({ colorScheme: "light" });
  await expect(page.locator("html")).toHaveAttribute("data-mode", "light");
  await page.keyboard.press("Escape");
  const second = await context.newPage();
  await second.goto("/console/");
  await second.getByRole("button", { name: "Customize appearance" }).click();
  await second.getByRole("radio", { name: /Evergreen/ }).check();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "evergreen");
  await second.getByRole("radio", { name: "Dark", exact: true }).check();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await second.close();
});

test("mobile workspace navigation and theme drawer remain accessible without page overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  const viewportWidth = () =>
    page.evaluate(() => ({
      page: document.documentElement.scrollWidth,
      viewport: innerWidth,
    }));
  expect((await viewportWidth()).page).toBe((await viewportWidth()).viewport);
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  const nav = page.getByRole("dialog", { name: "Main navigation" });
  await expect(
    nav.getByRole("button", { name: "Tenants", exact: true }),
  ).toBeVisible();
  await nav.getByRole("button", { name: "Tenants", exact: true }).click();
  await expect(nav).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: "Tenants", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  const context = page.locator("#tenant-context");
  const tenant = await context.locator("option").nth(1).getAttribute("value");
  expect(tenant).toBeTruthy();
  await context.selectOption(tenant!);
  await expect(
    page.getByRole("heading", { name: "Tenant details", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Customize appearance" }).click();
  const drawer = page.getByRole("dialog", { name: "Appearance", exact: true });
  await drawer.getByRole("radio", { name: /Evergreen/ }).check();
  await drawer.getByRole("radio", { name: "Dark", exact: true }).check();
  expect((await viewportWidth()).page).toBe((await viewportWidth()).viewport);
  await page.screenshot({
    path: "../../artifacts/admin-mobile-appearance.png",
    fullPage: true,
    animations: "disabled",
  });
  await drawer.getByRole("button", { name: "Done" }).focus();
  await page.keyboard.press("Tab");
  await expect(
    drawer.getByRole("button", { name: "Close appearance" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: /Quick navigation/ }).click();
  await page.getByRole("textbox", { name: "Find a screen" }).fill("plans");
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("heading", { name: "Plans", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect((await viewportWidth()).page).toBe((await viewportWidth()).viewport);
});

test("console navigation and appearance still work when browser storage is disabled", async ({
  page,
}) => {
  await page.addInitScript(() => {
    Storage.prototype.getItem = () => {
      throw new DOMException("Storage disabled", "SecurityError");
    };
    Storage.prototype.setItem = () => {
      throw new DOMException("Storage disabled", "SecurityError");
    };
  });
  await login(page);
  await page.getByRole("button", { name: "Customize appearance" }).click();
  await page.getByRole("radio", { name: /Evergreen/ }).check();
  await page.getByRole("radio", { name: "Dark", exact: true }).check();
  await page.getByRole("button", { name: "Done" }).click();
  await page.getByRole("button", { name: "Tenants", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Tenants", exact: true }),
  ).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "evergreen");
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
});
