import { test, expect, type Page } from "@playwright/test";
import {
  login,
  nav,
  people,
  choose,
  post,
  fixtureTenant,
  pick,
} from "./helpers";
const roleIds = {
  platform: "00000000-0000-4000-8000-000000000001",
  tenant: "00000000-0000-4000-8000-000000000002",
  teamAdmin: "00000000-0000-4000-8000-000000000003",
  reviewer: "00000000-0000-4000-8000-000000000004",
};
async function read(page: Page, path: string) {
  const r = await page.request.get(path);
  expect(r.status()).toBe(200);
  return r.json();
}
async function create(page: Page, path: string, data: unknown) {
  const r = await post(page, path, data);
  expect(r.status, JSON.stringify(r.body)).toBe(201);
  return r.body;
}
async function tenantPage(
  browser: import("@playwright/test").Browser,
  tenant: string,
) {
  const context = await browser.newContext(),
    page = await context.newPage();
  await login(page, "tenant-admin");
  await page.goto(`/console/tenants/${tenant}/apps`);
  await expect(
    page.getByRole("heading", { name: "Apps", exact: true, level: 1 }),
  ).toBeVisible();
  return { context, page };
}
async function makeSpace(page: Page, base: string, owner: string) {
  const id = crypto.randomUUID();
  await create(page, base + "/spaces", {
    id,
    slug: "review-space",
    display_name: "Review Space",
    initial_manager_user_id: owner,
    database: { mode: "managed" },
  });
  return id;
}

test("existing administrative roles can be revoked while account, membership and team access stay disabled", async ({
  page,
  browser,
}) => {
  await login(page);
  const suffix = crypto.randomUUID(),
    name = "Revocation " + suffix;
  const user = await create(page, "/admin/v1/users", {
    display_name: name,
    email: "revoke-" + suffix + "@example.test",
    password: "A disposable review account password!",
  });
  const grant = await create(page, "/admin/v1/role-assignments", {
    id: crypto.randomUUID(),
    user_id: user.id,
    role_id: roleIds.platform,
  });
  try {
    expect(
      (
        await post(
          page,
          "/admin/v1/users/" + user.id,
          { state: "disabled" },
          "PATCH",
          1,
        )
      ).status,
    ).toBe(200);
    await nav(page, "Users");
    await page
      .getByRole("textbox", { name: "Search records", exact: true })
      .fill(name);
    const checkbox = page.getByRole("checkbox", {
      name: "Platform administrator for " + name,
      exact: true,
    });
    await expect(checkbox).toBeChecked();
    await expect(checkbox).toBeEnabled();
    await checkbox.click();
    await expect(checkbox).not.toBeChecked();
    await expect(checkbox).toBeDisabled();
    expect(
      (await read(page, "/admin/v1/users?q=" + user.id)).items[0].state,
    ).toBe("disabled");
  } finally {
    await post(
      page,
      "/admin/v1/role-assignments/" + grant.id,
      undefined,
      "DELETE",
      grant.revision,
    );
  }
  const { tenant, base, outsider } = await fixtureTenant(
    page,
    "Disabled-role removal",
  );
  const member = await tenantPage(browser, tenant),
    admin = member.page;
  await create(admin, base + "/memberships", { user_id: outsider.id });
  await create(admin, base + "/role-assignments", {
    id: crypto.randomUUID(),
    user_id: outsider.id,
    role_id: roleIds.tenant,
  });
  expect(
    (
      await post(
        admin,
        base + "/memberships/" + outsider.id,
        { state: "disabled" },
        "PATCH",
        1,
      )
    ).status,
  ).toBe(200);
  await people(admin);
  const row = admin.getByRole("row").filter({ hasText: outsider.display_name }),
    tenantRole = row.getByRole("checkbox", { name: /Tenant administrator/ });
  await expect(tenantRole).toBeEnabled();
  await tenantRole.click();
  await expect(tenantRole).not.toBeChecked();
  await expect(tenantRole).toBeDisabled();
  await expect(
    row.getByRole("button", { name: "Enable Tenant access", exact: true }),
  ).toBeVisible();
  await row
    .getByRole("button", { name: "Enable Tenant access", exact: true })
    .click();
  await expect(
    row.getByRole("button", { name: "Disable Tenant access", exact: true }),
  ).toBeVisible();
  await expect(tenantRole).not.toBeChecked();
  const team = crypto.randomUUID(),
    path = base + "/teams/" + team;
  await create(admin, base + "/teams", {
    id: team,
    slug: "offboarding",
    display_name: "Offboarding",
  });
  expect(
    (await post(admin, path + "/members", { user_id: outsider.id }, "POST", 1))
      .status,
  ).toBe(200);
  for (const role_id of [roleIds.teamAdmin, roleIds.reviewer])
    await create(admin, path + "/role-assignments", {
      id: crypto.randomUUID(),
      user_id: outsider.id,
      role_id,
    });
  await admin.getByRole("tab", { name: "Teams", exact: true }).click();
  await admin
    .getByRole("row")
    .filter({ hasText: "Offboarding" })
    .getByRole("button", { name: "Open team", exact: true })
    .click();
  await admin
    .getByRole("button", { name: "Disable team access", exact: true })
    .click();
  const reviewer = admin.getByRole("checkbox", {
    name: "Reviewer for " + outsider.display_name,
    exact: true,
  });
  await expect(reviewer).toBeEnabled();
  await reviewer.click();
  await expect(reviewer).not.toBeChecked();
  await expect(reviewer).toBeDisabled();
  const current = await read(admin, path);
  expect(
    (await post(admin, path, { state: "disabled" }, "PATCH", current.revision))
      .status,
  ).toBe(200);
  await admin.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(
    admin.getByRole("heading", { name: /Offboarding.*disabled/ }),
  ).toBeVisible();
  const teamAdmin = admin.getByRole("checkbox", {
    name: "Team administrator for " + outsider.display_name,
    exact: true,
  });
  await expect(teamAdmin).toBeEnabled();
  await teamAdmin.click();
  await expect(teamAdmin).not.toBeChecked();
  expect((await read(admin, path)).state).toBe("disabled");
  expect((await read(admin, path + "/members")).items[0].state).toBe(
    "disabled",
  );
  await member.context.close();
});

test("platform can add a verified SSO identity as an ordinary member and access pickers search emails", async ({
  page,
  browser,
}) => {
  await login(page);
  const { tenant, base, owner, outsider } = await fixtureTenant(
    page,
    "SSO membership",
  );
  await nav(page, "Users");
  await page
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill(outsider.email);
  await page
    .getByRole("row")
    .filter({ hasText: outsider.email })
    .getByRole("button", { name: "Add to Tenant", exact: true })
    .click();
  await choose(page, "Tenant", tenant);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Add to Tenant", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const membership = (await read(page, base + "/memberships?q=" + outsider.id))
    .items[0];
  expect(membership.state).toBe("active");
  expect(membership.administrator_id).toBeNull();
  expect(
    (await read(page, "/admin/v1/users?q=" + outsider.id)).items[0]
      .oidc_subject,
  ).toBe("outsider");
  await page.goto(`/console/platform/tenants/${tenant}/administrators`);
  await page
    .getByRole("textbox", { name: "Search records", exact: true })
    .fill(owner.email);
  await expect(
    page.getByRole("row").filter({ hasText: owner.email }),
  ).toBeVisible();
  const member = await tenantPage(browser, tenant),
    admin = member.page;
  expect((await admin.request.get("/admin/v1/users")).status()).toBe(403);
  await people(admin);
  await expect(
    admin
      .getByRole("row")
      .filter({ hasText: outsider.email })
      .getByRole("checkbox", { name: /Tenant administrator/ }),
  ).not.toBeChecked();
  const space = await makeSpace(admin, base, owner.id),
    app = crypto.randomUUID();
  await create(admin, base + `/spaces/${space}/apps`, {
    id: app,
    slug: "sso-app",
    display_name: "SSO App",
    initial_manager_user_id: owner.id,
  });
  const access = base + "/apps/" + app;
  await create(admin, access + "/roles", {
    id: crypto.randomUUID(),
    code: "sso-reviewer",
    display_name: "SSO reviewer",
  });
  await admin.goto(`/console/tenants/${tenant}/apps/${app}/access`);
  await admin.getByRole("button", { name: "Add access", exact: true }).click();
  await choose(admin, "Role", "SSO reviewer");
  await choose(admin, "Person", outsider.email);
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Add access", exact: true })
    .click();
  await expect(admin.getByRole("dialog")).toHaveCount(0);
  await expect(
    admin.getByRole("listitem").filter({ hasText: outsider.display_name }),
  ).toContainText("SSO reviewer");
  expect(
    (await read(admin, base + "/memberships?q=" + outsider.id)).items[0]
      .administrator_id,
  ).toBeNull();
  await member.context.close();
});

test("access refresh preserves stale editors and App compute preferences survive plan changes", async ({
  page,
  browser,
}) => {
  await login(page);
  const { tenant, base, owner, outsider } = await fixtureTenant(
    page,
    "Refresh and compute",
  );
  const plan = {
    id: crypto.randomUUID(),
    code: "compute-" + crypto.randomUUID(),
    display_name: "Review compute",
    allow_shared_database: true,
    allow_dedicated_database: true,
    allow_managed_database: true,
    allow_registered_database: true,
    allow_dedicated_compute: true,
    default_layout: "shared",
  };
  await create(page, "/admin/v1/plans", plan);
  const t = await read(page, base);
  expect(
    (
      await post(
        page,
        "/admin/v1/tenants/" + tenant,
        { plan_id: plan.id },
        "PATCH",
        t.revision,
      )
    ).status,
  ).toBe(200);
  const member = await tenantPage(browser, tenant),
    admin = member.page;
  await create(admin, base + "/memberships", { user_id: outsider.id });
  const space = await makeSpace(admin, base, owner.id),
    app = crypto.randomUUID();
  await create(admin, base + `/spaces/${space}/apps`, {
    id: app,
    slug: "compute-app",
    display_name: "Compute App",
    initial_manager_user_id: owner.id,
    compute_mode: "dedicated",
  });
  const spacePath = base + "/spaces/" + space,
    role = await create(admin, spacePath + "/roles", {
      id: crypto.randomUUID(),
      code: "original",
      display_name: "Original reviewer",
    });
  await admin.goto(`/console/tenants/${tenant}/spaces/${space}/access`);
  await expect(
    admin.getByRole("row").filter({ hasText: "Original reviewer" }),
  ).toBeVisible();
  await create(admin, spacePath + "/roles", {
    id: crypto.randomUUID(),
    code: "outside",
    display_name: "External creation",
  });
  await admin.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(
    admin.getByRole("row").filter({ hasText: "External creation" }),
  ).toBeVisible();
  await admin
    .getByRole("row")
    .filter({ hasText: "Original reviewer" })
    .getByRole("button", { name: "Edit role", exact: true })
    .click();
  await admin.getByLabel("Name", { exact: true }).fill("My draft");
  expect(
    (
      await post(
        admin,
        spacePath + "/roles/" + role.id,
        { display_name: "Concurrent rename" },
        "PATCH",
        role.revision,
      )
    ).status,
  ).toBe(200);
  await admin
    .getByRole("button", { name: "Refresh", exact: true })
    .dispatchEvent("click");
  await expect(
    admin.getByRole("row").filter({ hasText: "Concurrent rename" }),
  ).toBeVisible();
  await expect(admin.getByLabel("Name", { exact: true })).toHaveValue(
    "My draft",
  );
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Save", exact: true })
    .click();
  await expect(admin.getByRole("dialog").getByRole("alert")).toContainText(
    /changed|revision|refresh/i,
  );
  await admin
    .getByRole("dialog")
    .getByRole("button", { name: "Cancel", exact: true })
    .click();
  await create(admin, spacePath + "/roles", {
    id: crypto.randomUUID(),
    code: "background",
    display_name: "Background creation",
  });
  await expect(
    admin.getByRole("row").filter({ hasText: "Background creation" }),
  ).toBeVisible({ timeout: 8000 });
  for (const [kind, id] of [
    ["spaces", space],
    ["apps", app],
  ]) {
    const scope = base + "/" + kind + "/" + id;
    await admin.goto(`/console/tenants/${tenant}/${kind}/${id}/access`);
    // The owner manages the scope (an App also lists its Space's
    // assignments, inherited).
    await expect(
      admin
        .getByRole("listitem")
        .filter({ hasText: owner.display_name })
        .filter({
          hasText: kind === "spaces" ? "Space manager" : "App manager",
        }),
    ).toBeVisible();
    const r = await create(admin, scope + "/roles", {
      id: crypto.randomUUID(),
      code: "refresh-grant",
      display_name: "Refreshed assignment",
    });
    await create(admin, scope + "/role-assignments", {
      id: crypto.randomUUID(),
      role_id: r.id,
      user_id: outsider.id,
    });
    await admin.getByRole("button", { name: "Refresh", exact: true }).click();
    await expect(
      admin.getByRole("listitem").filter({ hasText: outsider.display_name }),
    ).toContainText("Refreshed assignment");
  }
  expect(
    (
      await post(
        page,
        "/admin/v1/plans/" + plan.id,
        { ...plan, allow_dedicated_compute: false },
        "PATCH",
        1,
      )
    ).status,
  ).toBe(200);
  await admin.goto(`/console/tenants/${tenant}/apps/${app}/settings`);
  await expect(
    admin.getByRole("heading", { name: "Compute App", exact: true, level: 1 }),
  ).toBeVisible();
  await admin.getByText("Advanced", { exact: true }).click();
  // The fact's value is the definition paired with its term.
  await expect(
    admin
      .getByRole("term")
      .filter({ hasText: /^Compute preference$/ })
      .locator("xpath=following-sibling::dd[1]"),
  ).toHaveText("dedicated");
  await admin.getByRole("button", { name: "Edit App", exact: true }).click();
  let dialog = admin.getByRole("dialog");
  await dialog.getByText("Advanced", { exact: true }).click();
  await expect(
    dialog.getByRole("combobox", { name: "Compute preference", exact: true }),
  ).toHaveAttribute("data-value", "dedicated");
  await dialog.getByLabel("Name", { exact: true }).fill("Kept dedicated");
  await dialog
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
  expect((await read(admin, base + "/apps/" + app)).compute_mode).toBe(
    "dedicated",
  );
  await admin.getByRole("button", { name: "Edit App", exact: true }).click();
  dialog = admin.getByRole("dialog");
  await dialog.getByText("Advanced", { exact: true }).click();
  await pick(
    dialog.getByLabel("Compute preference", { exact: true }),
    "shared",
  );
  await dialog
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
  expect((await read(admin, base + "/apps/" + app)).compute_mode).toBe(
    "shared",
  );
  await admin.getByRole("button", { name: "Edit App", exact: true }).click();
  dialog = admin.getByRole("dialog");
  await dialog.getByText("Advanced", { exact: true }).click();
  await expect(
    dialog
      .getByLabel("Compute preference", { exact: true })
      .getByRole("option", { name: "Dedicated", exact: true }),
  ).toHaveCount(0);
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await member.context.close();
});
