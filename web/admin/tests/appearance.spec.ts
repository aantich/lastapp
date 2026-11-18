import { test, expect } from "@playwright/test";
import { accountMenu, login, menu, nav } from "./helpers";
async function appearance(page: import("@playwright/test").Page) {
  await menu(page);
  await page.getByRole("menuitem", { name: "Appearance", exact: true }).click();
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
  // Signal is the only theme: light cards are white on the warm canvas and
  // the sidebar stays ink in both modes.
  expect(original).toBe("rgb(255, 255, 255)");
  await expect(page.locator(".sidebar")).toHaveCSS(
    "background-color",
    "rgb(14, 15, 18)",
  );
  const drawer = await appearance(page);
  await expect(drawer.getByRole("radio")).toHaveCount(3);
  await expect(drawer.getByText("Theme preset")).toHaveCount(0);
  await drawer.getByRole("radio", { name: "Dark", exact: true }).check();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await expect(panel).not.toHaveCSS("background-color", original);
  await expect(page.locator(".sidebar")).toHaveCSS(
    "background-color",
    "rgb(14, 15, 18)",
  );
  await drawer.getByRole("button", { name: "Done", exact: true }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await expect(page.locator("html")).not.toHaveAttribute("data-theme", /.*/);
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
  await page.keyboard.press("Escape");
  await expect(accountMenu(page)).toBeFocused();
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
  await page.getByRole("tab", { name: "Plans", exact: true }).click();
  await page.getByRole("button", { name: "Create plan", exact: true }).click();
  await fits();
  await page.keyboard.press("Escape");
  const drawer = await appearance(page);
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
  await expect(accountMenu(page)).toBeFocused();
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
  await page.getByRole("radio", { name: "Dark", exact: true }).check();
  await page.getByRole("button", { name: "Done", exact: true }).click();
  await nav(page, "Users");
  await expect(
    page.getByRole("heading", { name: "Users", exact: true, level: 1 }),
  ).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
});
test("a saved preference from the former theme presets keeps its mode", async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem(
      "lastapp-appearance",
      '{"theme":"evergreen","mode":"dark"}',
    ),
  );
  await login(page);
  await expect(page.locator("html")).toHaveAttribute("data-mode", "dark");
  await expect(page.locator("html")).not.toHaveAttribute("data-theme", /.*/);
  await appearance(page);
  await page.getByRole("radio", { name: "Light", exact: true }).check();
  expect(
    await page.evaluate(() => localStorage.getItem("lastapp-appearance")),
  ).toBe('{"mode":"light"}');
});
