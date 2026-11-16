import { test, expect } from "@playwright/test";
import { createServer, type Server } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { login, logout, fixtureTenant, selectTenant, post } from "./helpers";
import { apiOrigin, appOrigin, appPort } from "./stack";
let staticServer: Server;
let approvedApp = "";
test.afterEach(async ({ page }) => {
  if (approvedApp && !page.isClosed()) {
    const current = await (
      await page.request.get("/v1/apps/" + approvedApp + "/data-access")
    ).json();
    if (current.revision)
      await post(
        page,
        "/v1/apps/" + approvedApp + "/data-access",
        { ...current.access, origin: "" },
        "PATCH",
        current.revision,
      );
  }
  approvedApp = "";
});
test.beforeAll(async () => {
  staticServer = createServer(async (req, res) => {
    const paths: Record<string, string> = {
      "/": "index.html",
      "/index.html": "index.html",
      "/app.js": "app.js",
      "/client.js": "client.js",
      "/style.css": "style.css",
    };
    const file = paths[(req.url || "").split("?")[0]];
    if (!file) {
      res.writeHead(404).end();
      return;
    }
    try {
      const data = await readFile(
        resolve("../../examples/data-app/public", file),
      );
      res.setHeader(
        "Content-Type",
        file.endsWith(".js")
          ? "text/javascript"
          : file.endsWith(".css")
            ? "text/css"
            : "text/html",
      );
      res.end(data);
    } catch {
      res.writeHead(404).end();
    }
  });
  await new Promise<void>((r) => staticServer.listen(appPort, "127.0.0.1", r));
});
test.afterAll(async () => {
  await new Promise<void>((r, j) =>
    staticServer?.close((e) => (e ? j(e) : r())),
  );
});
test("isolated browser App connects with PKCE and uses the real entity API", async ({
  page,
  context,
}) => {
  await login(page);
  const { tenant, base, owner } = await fixtureTenant(page, "Isolated App");
  await logout(page);
  await login(page, "tenant-admin");
  await selectTenant(page, tenant);
  const space = crypto.randomUUID(),
    app = crypto.randomUUID();
  expect(
    (
      await post(page, base + "/spaces", {
        id: space,
        slug: "crm",
        display_name: "SDK CRM",
        initial_manager_user_id: owner.id,
        database: { mode: "managed", layout: "shared" },
      })
    ).status,
  ).toBe(201);
  expect(
    (
      await post(page, base + "/spaces/" + space + "/apps", {
        id: app,
        slug: "deals",
        display_name: "Deal review",
        initial_manager_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  const entity = await post(page, "/v1/spaces/" + space + "/entities", {
    definition: {
      format_version: 1,
      key: "deal",
      label: "Deal",
      fields: [
        {
          key: "name",
          label: "Name",
          type: "text",
          max_length: 100,
          required: true,
        },
      ],
      default_projection: ["name"],
      rules: [],
      permissions: [
        {
          key: "read",
          label: "Read",
          operation: "read",
          relationship: "member",
          fields: ["name"],
        },
        {
          key: "create",
          label: "Create",
          operation: "create",
          relationship: "member",
          fields: ["name"],
        },
      ],
    },
  });
  expect(entity.status).toBe(200);
  await expect
    .poll(
      async () => {
        const s = await (
          await page.request.get(base + "/spaces/" + space)
        ).json();
        return s.database_state;
      },
      { timeout: 30000 },
    )
    .toBe("ready");
  expect(
    (
      await post(
        page,
        "/v1/entities/" + entity.body.id + "/publications",
        {},
        "POST",
        entity.body.revision,
      )
    ).status,
  ).toBe(202);
  await expect
    .poll(
      async () => {
        const e = await (
          await page.request.get("/v1/entities/" + entity.body.id)
        ).json();
        return e.publications[0]?.state;
      },
      { timeout: 30000 },
    )
    .toBe("published");
  const appInfo = await (await page.request.get(base + "/apps/" + app)).json();
  expect(
    (
      await post(
        page,
        "/v1/apps/" + app + "/data-access",
        {
          origin: appOrigin,
          inherit: true,
          ceiling: [],
          restricted: false,
          restrictions: [],
        },
        "PATCH",
        appInfo.revision,
      )
    ).status,
  ).toBe(200);
  approvedApp = app;
  const browserApp = await context.newPage();
  await browserApp.goto(appOrigin);
  await browserApp.getByLabel("API origin", { exact: true }).fill(apiOrigin);
  await browserApp.getByLabel("App ID", { exact: true }).fill(app);
  const popupPromise = context.waitForEvent("page");
  await browserApp
    .getByRole("button", { name: "Connect App", exact: true })
    .click();
  const popup = await popupPromise;
  await expect(
    popup.getByRole("heading", { name: "Connect Deal review", exact: true }),
  ).toBeVisible();
  await popup.screenshot({
    path: "test-results/app-connection-dialog.png",
    animations: "disabled",
  });
  await popup.getByRole("button", { name: "Connect App", exact: true }).click();
  await expect(browserApp.locator("#data")).toBeVisible();
  await expect(browserApp.getByLabel("Entity", { exact: true })).toHaveValue(
    "deal",
  );
  await browserApp.getByText("Create a record", { exact: true }).click();
  await browserApp
    .locator("#values")
    .fill(JSON.stringify({ name: "Real browser Deal" }));
  await browserApp.getByRole("button", { name: "Create", exact: true }).click();
  await expect(browserApp.locator("#records")).toContainText("revision");
  await browserApp
    .getByRole("button", { name: "Load records", exact: true })
    .click();
  await expect(browserApp.locator("#records")).toContainText(
    "Real browser Deal",
  );
  expect(await browserApp.evaluate(() => document.cookie)).not.toContain(
    "lastapp_session",
  );
  expect(await browserApp.evaluate(() => localStorage.length)).toBe(0);
  await browserApp.screenshot({
    path: "test-results/isolated-data-app-desktop.png",
    fullPage: true,
    animations: "disabled",
  });
  await browserApp.setViewportSize({ width: 390, height: 844 });
  await browserApp.evaluate(
    () =>
      new Promise((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(resolve)),
      ),
  );
  expect(
    await browserApp.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await browserApp.screenshot({
    path: "test-results/isolated-data-app-mobile.png",
    animations: "disabled",
  });
  await browserApp.close();
});
