import { test, expect } from "@playwright/test";
import { login, menu, nav } from "./helpers";
async function appearance(page: import("@playwright/test").Page) {
  await menu(page);
  await page.getByRole("button", { name: "Appearance", exact: true }).click();
  return page.getByRole("dialog", { name: "Appearance", exact: true });
}
test("appearance changes shared screens, persists and resets with focus restoration", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.setViewportSize({ width: 1440, height: 1050 });
  await login(page);
  await page
    .getByRole("button", { name: "Collapse sidebar", exact: true })
    .click();
  await expect(page.locator(".shell")).toHaveClass(/sidebar-collapsed/);
  await page
    .getByRole("button", { name: "Expand sidebar", exact: true })
    .click();
  const panel = page.locator(".panel").first();
  await expect(panel).toBeVisible();
  const original = await panel.evaluate(
    (el) => getComputedStyle(el).backgroundColor,
  );
  await page.screenshot({
    path: "../../artifacts/simple-platform.png",
    fullPage: true,
  });
  const drawer = await appearance(page);
  await drawer.getByRole("radio", { name: /Studio/ }).check();
  await drawer.getByRole("radio", { name: "Dark", exact: true }).check();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "studio");
  await expect(panel).not.toHaveCSS("background-color", original);
  await drawer.getByRole("button", { name: "Done", exact: true }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "studio");
  await page
    .getByRole("button", { name: "Create Tenant", exact: true })
    .click();
  const form = page.getByRole("dialog", { name: "Create Tenant", exact: true });
  await expect(form).toBeVisible();
  await page.screenshot({
    path: "../../artifacts/simple-dark-form.png",
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  await expect(form).toHaveCount(0);
  await appearance(page);
  await drawer.getByRole("button", { name: "Reset to default" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "light");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "neo");
  await page.keyboard.press("Escape");
  await expect(page.getByLabel("Account menu", { exact: true })).toBeFocused();
  expect(errors).toEqual([]);
});
test("system mode, cross-tab sync and invalid preferences recover", async ({
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
  await appearance(page);
  await page.getByRole("radio", { name: "System", exact: true }).check();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await page.emulateMedia({ colorScheme: "light" });
  await expect(page.locator("html")).toHaveAttribute("data-mode", "light");
  await page.keyboard.press("Escape");
  const second = await context.newPage();
  await second.goto("/console/");
  await appearance(second);
  await second.getByRole("radio", { name: /Evergreen/ }).check();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "evergreen");
  await second.getByRole("radio", { name: "Dark", exact: true }).check();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await second.close();
});
test("mobile navigation, forms and theme drawer fit and trap focus", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  const fits = async () =>
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  await fits();
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  const sidebar = page.getByRole("dialog", { name: "Navigation", exact: true });
  await sidebar.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(sidebar).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: "Settings", exact: true, level: 1 }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Plans", exact: true }).click();
  await page.getByRole("button", { name: "Create plan", exact: true }).click();
  await fits();
  await page.keyboard.press("Escape");
  const drawer = await appearance(page);
  await drawer.getByRole("radio", { name: /Evergreen/ }).check();
  await drawer.getByRole("radio", { name: "Dark", exact: true }).check();
  await fits();
  await page.screenshot({
    path: "../../artifacts/simple-mobile.png",
    fullPage: true,
  });
  await drawer.getByRole("button", { name: "Done", exact: true }).focus();
  await page.keyboard.press("Tab");
  await expect(
    drawer.getByRole("button", { name: "Close appearance", exact: true }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(page.getByLabel("Account menu", { exact: true })).toBeFocused();
  await fits();
});
test("navigation and appearance work with browser storage disabled", async ({
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
  await appearance(page);
  await page.getByRole("radio", { name: /Evergreen/ }).check();
  await page.getByRole("radio", { name: "Dark", exact: true }).check();
  await page.getByRole("button", { name: "Done", exact: true }).click();
  await nav(page, "Users");
  await expect(
    page.getByRole("heading", { name: "Users", exact: true, level: 1 }),
  ).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "evergreen");
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
});
