import { test, expect } from "@playwright/test";
import {
  login,
  fixtureTenant,
  selectTenant,
  nav,
  choose,
  pick,
} from "./helpers";
import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";

test("existing database registration, shared access controls and Tenant-switch isolation", async ({
  page,
  browser,
}) => {
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
  const { tenant, base } = await fixtureTenant(page, "Registered database");
  const other = await fixtureTenant(page, "Switch destination");
  const context = await browser.newContext(),
    admin = await context.newPage();
  await login(admin, "tenant-admin");
  await selectTenant(admin, tenant);
  await nav(admin, "Settings");
  await admin.getByRole("button", { name: "Databases", exact: true }).click();
  await admin
    .getByRole("button", { name: "Add database", exact: true })
    .click();
  await pick(admin.getByLabel("Method", { exact: true }), "registered");
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
    .getByRole("dialog")
    .getByRole("button", { name: "Add database", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await expect(
    admin
      .getByRole("row")
      .filter({ hasText: dbName })
      .getByText("ready", { exact: true }),
  ).toBeVisible({ timeout: 30000 });
  expect(
    execFileSync(
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
    ).trim(),
  ).toBe("7");
  await nav(admin, "Spaces");
  await admin
    .getByRole("button", { name: "Create Space", exact: true })
    .click();
  await admin.getByLabel("Name", { exact: true }).fill("Sales");
  await admin.getByText("Advanced", { exact: true }).click();
  await pick(admin.getByLabel("Database", { exact: true }), "existing");
  await choose(admin, "Existing database", dbName);
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Create Space", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin
    .getByRole("row")
    .filter({ hasText: "Sales" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  const access = async (name: string) => {
    await admin.getByRole("button", { name: "Access", exact: true }).click();
    await admin
      .getByRole("button", { name: "Manage roles", exact: true })
      .click();
    await admin
      .getByRole("button", { name: "Create role", exact: true })
      .click();
    await admin.getByLabel("Name", { exact: true }).fill(name);
    await admin
      .getByRole("dialog")
      .getByRole("button", { name: "Create role", exact: true })
      .click();
    await expect(admin.getByRole("dialog")).toHaveCount(0);
    await expect(
      admin.getByRole("row").filter({ hasText: name }),
    ).toBeVisible();
  };
  await access("Sales observer");
  await admin.screenshot({
    path: "../../artifacts/simple-space-access.png",
    fullPage: true,
  });
  await admin.getByRole("button", { name: "Apps", exact: true }).last().click();
  await admin.getByRole("button", { name: "Create App", exact: true }).click();
  await admin.getByLabel("Name", { exact: true }).fill("Sales CRM");
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Create App", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await admin
    .getByRole("row")
    .filter({ hasText: "Sales CRM" })
    .getByRole("button", { name: "Open", exact: true })
    .click();
  await access("App analyst");
  // Arbitrary role grants use a name picker even when the user only manages this App.
  // Tenant authors may choose users and teams, without entering internal IDs.
  await admin
    .getByRole("button", { name: "People and teams", exact: true })
    .click();
  await admin.getByRole("button", { name: "Add access", exact: true }).click();
  await choose(admin, "Role", "App analyst");
  await expect(admin.getByLabel("Recipient", { exact: true })).toBeVisible();
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Cancel", exact: true })
    .click();
  let release!: () => void, started!: () => void;
  const held = new Promise<void>((r) => (release = r)),
    pending = new Promise<void>((r) => (started = r));
  await admin.route("**" + base + "/database-targets", async (route) => {
    started();
    await held;
    await route.continue();
  });
  await nav(admin, "Spaces");
  await admin
    .getByRole("button", { name: "Create Space", exact: true })
    .click();
  await pending;
  await selectTenant(admin, other.tenant);
  const done = admin.waitForResponse(
    (r) => new URL(r.url()).pathname === base + "/database-targets",
  );
  release();
  await (await done).finished();
  await admin.evaluate(
    () =>
      new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))),
  );
  await expect(
    admin.getByRole("heading", { name: "Apps", exact: true, level: 1 }),
  ).toBeVisible();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await context.close();
});
