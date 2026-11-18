import { expect, test } from "@playwright/test";
import { fixtureTenant, login, logout, tenantSwitcher } from "./helpers";

// The sidebar's Tenant switcher pages through /v1/my-tenants (50 a page).
// "Load more" appends the next page inside the open popup: the popup stays
// open, the search field keeps focus (so the keyboard still works) and the
// first new Tenant is brought into view. Before the fix, focus fell to the
// page when the clicked button was disabled and removed, which left a list
// that looked unchanged and no longer answered the keyboard.
// The switcher is always the sidebar's Tenant section, whatever the number
// of Tenants, so switching is discoverable: reachable by role and name, it
// opens on the current Tenant.
test("Tenant switcher is reachable from the sidebar", async ({ page }) => {
  await login(page, "tenant-admin");
  const trigger = tenantSwitcher(page);
  await expect(trigger).toBeVisible();
  await expect(trigger).toHaveAttribute("aria-haspopup", "listbox");
  await trigger.click();
  const list = page.getByRole("listbox", { name: "Tenants", exact: true });
  await expect(list).toBeVisible();
  await expect(list.getByRole("option", { selected: true })).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
});

test("Tenant switcher loads more Tenants and stays open", async ({ page }) => {
  test.setTimeout(180000);
  await login(page);
  for (let i = 0; i < 52; i++) await fixtureTenant(page, "Paging");
  await logout(page);
  await login(page, "tenant-admin");

  const trigger = tenantSwitcher(page);
  await trigger.click();
  const search = page.getByRole("combobox", {
    name: "Find a Tenant",
    exact: true,
  });
  await expect(search).toBeFocused();
  const list = page.getByRole("listbox", { name: "Tenants", exact: true });
  const options = list.getByRole("option");
  await expect(options).toHaveCount(50);

  await page.getByRole("button", { name: "Load more", exact: true }).click();
  await expect.poll(() => options.count()).toBeGreaterThan(50);
  await expect(trigger).toHaveAttribute("aria-expanded", "true");
  await expect(list).toBeVisible();
  await expect(search).toBeFocused();
  await expect(options.nth(50)).toBeInViewport();

  // The keyboard still drives the open list.
  await page.keyboard.press("Escape");
  await expect(list).toBeHidden();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
});
