import { test, expect } from "@playwright/test";
import {
  login,
  fixtureTenant,
  selectTenant,
  nav,
  pick,
  rootSQL,
  accountHost,
  confirmDialog,
} from "./helpers";
import { randomBytes } from "node:crypto";

test("database actions guard double clicks and confirm through ConfirmDialog", async ({
  page,
  browser,
}) => {
  const suffix = Date.now().toString(),
    dbName = "ui_" + suffix,
    password = randomBytes(24).toString("hex");
  rootSQL(
    `CREATE DATABASE ${dbName}; CREATE USER '${dbName}'@'${accountHost}' IDENTIFIED BY '${password}'; GRANT SELECT,INSERT,UPDATE,DELETE ON \`ui\\_${suffix}\`.* TO '${dbName}'@'${accountHost}';`,
  );
  await login(page);
  const { tenant } = await fixtureTenant(page, "Confirm dialog");
  const context = await browser.newContext({
      viewport: { width: 1280, height: 800 },
    }),
    admin = await context.newPage();
  await login(admin, "tenant-admin");
  await selectTenant(admin, tenant);
  await nav(admin, "Settings");
  await admin.getByRole("button", { name: "Databases", exact: true }).click();
  await admin
    .getByRole("button", { name: "Add database", exact: true })
    .click();
  await pick(admin.getByLabel("Method", { exact: true }), "registered");
  for (const label of ["name", "username"])
    await admin
      .getByLabel("Existing database " + label, { exact: true })
      .fill(dbName);
  await admin
    .getByLabel("Existing database password", { exact: true })
    .fill(password);
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Add database", exact: true })
    .click();
  const row = admin.getByRole("row").filter({ hasText: dbName });
  await expect(row.getByText("ready", { exact: true })).toBeVisible({
    timeout: 30000,
  });

  // A rapid double click on a mutating action sends one request.
  const mutations: string[] = [];
  admin.on("request", (r) => {
    if (r.method() !== "GET" && r.url().includes("/database-resources/"))
      mutations.push(r.method() + " " + new URL(r.url()).pathname);
  });
  await admin.route("**/database-resources/*/retry", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 400));
    await route.continue();
  });
  await row
    .getByRole("button", { name: "Recheck connection", exact: true })
    .dblclick();
  await expect
    .poll(() => mutations.filter((m) => m.endsWith("/retry")).length)
    .toBe(1);
  await admin.waitForTimeout(800);
  expect(mutations.filter((m) => m.endsWith("/retry"))).toHaveLength(1);

  const retire = row.getByRole("button", { name: "Retire", exact: true });
  const dialog = admin.getByRole("alertdialog");
  for (const viewport of [
    { width: 1280, height: 800 },
    { width: 390, height: 844 },
  ]) {
    await admin.setViewportSize(viewport);
    await expect(retire).toBeEnabled();
    await retire.click();
    await expect(dialog).toBeVisible();
    await expect(dialog).toHaveAccessibleName("Retire database resource?");
    const box = (await dialog.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(viewport.width);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);
    // Destructive confirmations focus Cancel and keep Tab inside the dialog.
    await expect(
      dialog.getByRole("button", { name: "Cancel", exact: true }),
    ).toBeFocused();
    for (let i = 0; i < 4; i++) {
      await admin.keyboard.press("Tab");
      expect(
        await dialog.evaluate((d) => d.contains(document.activeElement)),
      ).toBe(true);
    }
    await expect(
      dialog.getByRole("button", { name: "Retire", exact: true }),
    ).toHaveClass(/danger/);
    // Escape cancels without a request and returns focus to the trigger.
    await admin.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(retire).toBeFocused();
    expect(mutations.filter((m) => m.startsWith("PATCH"))).toHaveLength(0);
  }

  // The recheck finishes in a worker that bumps the revision, so refresh
  // until the confirmed retirement is accepted.
  await expect(async () => {
    await admin.getByRole("button", { name: "Refresh", exact: true }).click();
    await retire.click();
    await confirmDialog(admin, "Retire");
    await expect(row.getByText("retired", { exact: true })).toBeVisible({
      timeout: 3000,
    });
  }).toPass({ timeout: 30000 });
  expect(
    mutations.filter((m) => m.startsWith("PATCH")).length,
  ).toBeGreaterThanOrEqual(1);
  await context.close();
});
