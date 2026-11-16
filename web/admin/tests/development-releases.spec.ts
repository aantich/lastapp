import { test, expect } from "@playwright/test";
import {
  mkdtemp,
  mkdir,
  writeFile,
  symlink,
  rm,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, dirname, join } from "node:path";
import { execFileSync } from "node:child_process";
import { login, logout, fixtureTenant, post } from "./helpers";

test("full production capacity still permits creating an App in a workspace", async ({
  page,
}) => {
  await login(page);
  const { tenant, base, owner } = await fixtureTenant(
    page,
    "Development capacity",
  );
  const plan = crypto.randomUUID();
  expect(
    (
      await post(page, "/admin/v1/plans", {
        id: plan,
        code: "dev-capacity-" + plan,
        display_name: "One production App " + plan,
        allow_shared_database: true,
        allow_managed_database: true,
        default_layout: "shared",
        max_apps: 1,
      })
    ).status,
  ).toBe(201);
  expect(
    (
      await post(
        page,
        "/admin/v1/tenants/" + tenant,
        { plan_id: plan },
        "PATCH",
        1,
      )
    ).status,
  ).toBe(200);
  await logout(page);
  await login(page, "tenant-admin");
  const space = crypto.randomUUID();
  expect(
    (
      await post(page, base + "/spaces", {
        id: space,
        slug: "production",
        display_name: "Production",
        initial_manager_user_id: owner.id,
        database: { mode: "managed", layout: "shared" },
      })
    ).status,
  ).toBe(201);
  expect(
    (
      await post(page, base + `/spaces/${space}/apps`, {
        id: crypto.randomUUID(),
        slug: "production",
        display_name: "Production App",
        initial_manager_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  const created = await post(page, `/v1/spaces/${space}/workspaces`, {
    id: crypto.randomUUID(),
    display_name: "Capacity workspace",
  });
  expect(created.status).toBe(200);
  await expect
    .poll(
      async () => {
        const response = await page.request.get(
          `/v1/workspaces/${created.body.id}`,
        );
        return (await response.json()).state;
      },
      { timeout: 45000 },
    )
    .toBe("ready");
  await page.goto(
    `/console/tenants/${tenant}/spaces/${created.body.backing_space_id}/apps`,
  );
  await expect(page.locator(".development-banner")).toBeVisible();
  await page.getByRole("button", { name: "Create App", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText(
    "Production capacity is checked when you deploy.",
  );
  await dialog.getByLabel("Name", { exact: true }).fill("New development App");
  await page.screenshot({
    path: "test-results/workspace-capacity-desktop.png",
    fullPage: true,
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 390);
  await page.screenshot({
    path: "test-results/workspace-capacity-mobile.png",
    fullPage: true,
    animations: "disabled",
  });
  await dialog.getByRole("button", { name: "Create App", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(
    page.getByRole("row").filter({ hasText: "New development App" }),
  ).toBeVisible();
  const production = await (await page.request.get(base + "/apps")).json();
  expect(production.items).toHaveLength(1);
});

test("local React build uses workspace APIs, then the same files deploy to production", async ({
  page,
  context,
}) => {
  test.setTimeout(120000);
  await login(page);
  const { tenant, base, owner } = await fixtureTenant(page, "Release browser");
  await logout(page);
  await login(page, "tenant-admin");
  const space = crypto.randomUUID();
  expect(
    (
      await post(page, base + "/spaces", {
        id: space,
        slug: "crm",
        display_name: "Production CRM",
        initial_manager_user_id: owner.id,
        database: { mode: "managed", layout: "shared" },
      })
    ).status,
  ).toBe(201);
  await page.goto(`/console/tenants/${tenant}/spaces/${space}/development`);
  await page
    .getByRole("button", { name: "Create workspace", exact: true })
    .click();
  await page
    .getByLabel("Workspace name", { exact: true })
    .fill("Browser author workspace");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Create workspace", exact: true })
    .click();
  let workspace: any;
  await expect
    .poll(
      async () => {
        const list = await (
          await page.request.get(`/v1/spaces/${space}/workspaces`)
        ).json();
        workspace = list.items[0];
        return workspace?.state;
      },
      { timeout: 45000 },
    )
    .toBe("ready");
  await page
    .getByRole("button", { name: "Refresh workspaces", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Open workspace", exact: true })
    .click();
  await expect(page.locator(".development-banner")).toContainText(
    "Development workspace",
  );
  const app = crypto.randomUUID();
  expect(
    (
      await post(page, base + `/spaces/${workspace.backing_space_id}/apps`, {
        id: app,
        slug: "pipeline",
        display_name: "Pipeline",
        initial_manager_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  expect(
    (
      await post(
        page,
        `/v1/apps/${app}/data-access`,
        {
          origin: "",
          inherit: true,
          ceiling: [],
          restrictions: [],
          restricted: false,
        },
        "PATCH",
        1,
      )
    ).status,
  ).toBe(200);
  const definition = {
    format_version: 1,
    key: "deal",
    label: "Deal",
    fields: [
      {
        key: "name",
        label: "Name",
        type: "text",
        required: true,
        max_length: 100,
        filterable: true,
        sortable: true,
      },
    ],
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
    default_projection: ["name"],
  };
  const entity = await post(
    page,
    `/v1/spaces/${workspace.backing_space_id}/entities`,
    { definition },
  );
  expect(entity.status).toBe(200);
  expect(
    (
      await post(
        page,
        `/v1/entities/${entity.body.id}/publications`,
        {},
        "POST",
        1,
      )
    ).status,
  ).toBe(202);
  await expect
    .poll(
      async () => {
        const current = await (
          await page.request.get("/v1/entities/" + entity.body.id)
        ).json();
        return !!current.active_version_id;
      },
      { timeout: 30000 },
    )
    .toBe(true);
  const micro = crypto.randomUUID();
  expect(
    (
      await post(page, base + `/apps/${app}/microapps`, {
        id: micro,
        slug: "deals",
        display_name: "Deals",
      })
    ).status,
  ).toBe(201);
  const starter = await (
      await page.request.get(`/v1/apps/${app}/starter`)
    ).json(),
    directory = await mkdtemp(join(tmpdir(), "lastapp-react-"));
  try {
    for (const [name, text] of Object.entries(starter.files)) {
      await mkdir(dirname(join(directory, name)), { recursive: true });
      await writeFile(join(directory, name), text as string);
    }
    await symlink(
      resolve("node_modules"),
      join(directory, "node_modules"),
      "dir",
    );
    execFileSync(
      process.execPath,
      [resolve("node_modules/typescript/bin/tsc"), "--noEmit"],
      { cwd: directory, stdio: "pipe" },
    );
    execFileSync(
      process.execPath,
      [resolve("node_modules/vite/bin/vite.js"), "build"],
      { cwd: directory, stdio: "pipe" },
    );
    await page.goto(`/console/tenants/${tenant}/apps/${app}/releases`);
    await expect(
      page.getByRole("heading", { name: "Releases", exact: true }),
    ).toBeVisible();
    const chooser = page.waitForEvent("filechooser");
    await page
      .getByRole("button", { name: "Upload build folder", exact: true })
      .click();
    await (await chooser).setFiles(join(directory, "dist"));
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByRole("heading", { name: "Create release", exact: true }),
    ).toBeVisible();
    await dialog.getByLabel("Release name", { exact: true }).fill("1.0.0");
    await dialog.getByText("Advanced", { exact: true }).click();
    const pkg = JSON.parse(starter.files["package.json"]);
    await dialog
      .getByLabel("Shared package versions", { exact: true })
      .fill(JSON.stringify(pkg.dependencies));
    await dialog
      .getByRole("button", { name: "Create release", exact: true })
      .click();
    await expect(dialog).toBeHidden();
    await expect(
      page.getByRole("heading", { name: "1.0.0", exact: true }),
    ).toBeVisible();
    const popupEvent = page.waitForEvent("popup");
    await page
      .getByRole("button", { name: "Open preview", exact: true })
      .click();
    const preview = await popupEvent;
    await preview.waitForLoadState();
    await expect(
      preview.getByRole("heading", { name: "Your App", exact: true }),
    ).toBeVisible();
    async function connect(clientPage: typeof page) {
      const launchEvent = clientPage.waitForEvent("popup");
      await clientPage
        .getByRole("button", { name: "Connect to Lastapp", exact: true })
        .click();
      const launch = await launchEvent;
      await launch.waitForLoadState();
      await launch
        .getByRole("button", { name: "Connect App", exact: true })
        .click();
      await expect(clientPage.getByRole("status")).toContainText("Connected.");
    }
    await connect(preview);
    await preview.evaluate(async () => {
      const client = (window as any).lastapp;
      await client.create("deal", { name: "Workspace fixture" });
    });
    const count = await preview.evaluate(async () => {
      const result = await (window as any).lastapp.query("deal", {
        include_count: true,
      });
      return result.count;
    });
    expect(count).toBe(1);
    await page.bringToFront();
    await expect(page.locator(".release-list .release-item")).toHaveCount(1);
    const layout = await page.evaluate(() => {
      const main = document.querySelector(".main")!,
        side = document.querySelector(".sidebar")!,
        content = document.querySelector(".content")!;
      return {
        mainX: main.getBoundingClientRect().x,
        sideWidth: side.getBoundingClientRect().width,
        contentX: content.getBoundingClientRect().x,
      };
    });
    expect(layout.mainX).toBeGreaterThanOrEqual(layout.sideWidth);
    expect(layout.contentX).toBeGreaterThanOrEqual(layout.sideWidth);
    await page.screenshot({
      path: "test-results/release-desktop.png",
      fullPage: false,
      animations: "disabled",
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.locator(".sidebar")).toHaveCSS(
      "transform",
      "matrix(1, 0, 0, 1, -244, 0)",
    );
    await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 390);
    await page.screenshot({
      path: "test-results/release-mobile.png",
      fullPage: true,
      animations: "disabled",
    });
    await page.setViewportSize({ width: 1280, height: 900 });
    await page
      .getByRole("button", { name: "Deploy to production", exact: true })
      .click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Deploy to production", exact: true })
      .click();
    await expect(page.locator(".deployment-status")).toContainText("active", {
      timeout: 45000,
    });
    // A real durable deployment remains visible after leaving/reloading the UI.
    await page.reload();
    await expect(page.locator(".deployment-status")).toContainText("active");
    const productionURL = await page
      .getByRole("link", { name: "Open production App" })
      .getAttribute("href");
    const production = await context.newPage();
    await production.goto(productionURL!);
    await expect(
      production.getByRole("heading", { name: "Your App", exact: true }),
    ).toBeVisible();
    await connect(production);
    expect(
      await production.evaluate(async () => {
        const client = (window as any).lastapp;
        const result = await client.query("deal", { include_count: true });
        return { count: result.count, app: client.appID };
      }),
    ).toEqual({
      count: 0,
      app: expect.not.stringMatching(new RegExp("^" + app + "$")),
    });
    await production.close();
    await preview.close();
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("deployment status survives reload and retries a transient failure", async ({
  page,
}) => {
  await login(page);
  const { tenant, base, owner } = await fixtureTenant(
    page,
    "Deployment recovery",
  );
  await logout(page);
  await login(page, "tenant-admin");
  const space = crypto.randomUUID(),
    app = crypto.randomUUID();
  expect(
    (
      await post(page, base + "/spaces", {
        id: space,
        slug: "recovery",
        display_name: "Recovery",
        initial_manager_user_id: owner.id,
        database: { mode: "managed", layout: "shared" },
      })
    ).status,
  ).toBe(201);
  expect(
    (
      await post(page, base + `/spaces/${space}/apps`, {
        id: app,
        slug: "recovery",
        display_name: "Recovery",
        initial_manager_user_id: owner.id,
      })
    ).status,
  ).toBe(201);
  let state = "queued",
    failures = 0,
    requests = 0;
  const job = {
    id: crypto.randomUUID(),
    target_app_id: app,
    url: "http://localhost:8787/",
    error_message: "Storage preparation failed safely.",
  };
  await page.route(`**/v1/apps/${app}/deployment`, async (route) => {
    requests++;
    if (failures > 0) {
      failures--;
      await route.fulfill({
        status: 503,
        json: { error: { message: "Temporary outage" } },
      });
      return;
    }
    await route.fulfill({
      json: {
        deployment: {
          ...job,
          state,
          error_message: state === "failed" ? job.error_message : null,
        },
      },
    });
  });
  await page.goto(`/console/tenants/${tenant}/apps/${app}/releases`);
  await expect(page.locator(".deployment-status")).toContainText("queued");
  await page.reload();
  await expect(page.locator(".deployment-status")).toContainText("queued");
  const before = requests;
  failures = 1;
  state = "active";
  await expect(page.getByRole("alert")).toContainText("Temporary outage", {
    timeout: 10000,
  });
  // No click or manual refresh: the same poll loop must recover by itself.
  await expect(page.locator(".deployment-status")).toContainText("active", {
    timeout: 15000,
  });
  expect(requests).toBeGreaterThanOrEqual(before + 2);
  await expect(page.getByRole("alert")).toHaveCount(0);
  state = "failed";
  await page.reload();
  await expect(page.locator(".deployment-status")).toContainText(
    job.error_message,
  );
  await page.screenshot({
    path: "test-results/deployment-recovery-desktop.png",
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".sidebar")).toHaveCSS(
    "transform",
    "matrix(1, 0, 0, 1, -244, 0)",
  );
  await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 390);
  await page.screenshot({
    path: "test-results/deployment-recovery-mobile.png",
    fullPage: true,
    animations: "disabled",
  });
});
