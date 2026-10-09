import { test, expect, Page } from "@playwright/test";
import { devLogin } from "./helpers";

// Support centers had no write path anywhere in the app: the only way to create
// or change one was a full "Import from GitHub" resync. They now go through the
// same propose/approve workflow as every other entity.

async function approveFromView(page: Page) {
  await expect(page).toHaveURL(/\/proposals\/view/);
  const id = new URL(page.url()).searchParams.get("id");
  const res = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(res.ok(), await res.text()).toBeTruthy();
}

const field = (page: Page, labelText: string, tag: "input" | "textarea" = "input") =>
  page.locator("label", { hasText: new RegExp(`^${labelText}$`) }).locator(`xpath=following-sibling::${tag}`);

// Files and approves a support-center proposal through the API (seeding).
async function seed(page: Page, operation: string, target: string | undefined, state: unknown) {
  const created = await page.request.post("/api/v1/proposals", {
    data: { entity_kind: "support_center", operation, target_name: target, submit: true, proposed_state: state },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { id } = await created.json();
  return page.request.post(`/api/v1/proposals/${id}/approve`);
}

test("create a support center through the form, then see it listed and in detail", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const name = `E2E_SC_${Date.now()}`;

  await page.goto("/support-centers");
  await expect(page.getByRole("heading", { name: "Support centers" })).toBeVisible();
  await page.getByRole("link", { name: "New support center" }).click();

  await field(page, "Name").fill(name);
  await field(page, "Long name").fill("E2E Long Name");
  await field(page, "Community").fill("E2E Community");
  await field(page, "Description", "textarea").fill("made through the form");
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  const detail = await (await page.request.get(`/api/v1/support-centers/${encodeURIComponent(name)}`)).json();
  expect(detail.long_name).toBe("E2E Long Name");
  expect(detail.community).toBe("E2E Community");
  expect(typeof detail.id).toBe("number");

  await page.goto(`/support-centers/detail?name=${encodeURIComponent(name)}`);
  await expect(page.getByText("made through the form")).toBeVisible();
  await page.goto("/support-centers");
  await page.getByPlaceholder(/Search by name/).fill(name);
  await expect(page.getByText(name, { exact: true })).toBeVisible();
});

test("editing through the form keeps Contacts, which the form has no control for", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const name = `E2E_SCKeep_${Date.now()}`;
  const contacts = { "Security Contact": [{ ID: "abc123", Name: "A Real Person" }] };
  const made = await seed(page, "create", undefined, { name, description: "before", extra: { Contacts: contacts } });
  expect(made.ok(), await made.text()).toBeTruthy();
  const before = await (await page.request.get(`/api/v1/support-centers/${encodeURIComponent(name)}`)).json();

  await page.goto(`/support-centers/new?edit=${encodeURIComponent(name)}`);
  const desc = field(page, "Description", "textarea");
  await expect(desc).toHaveValue("before"); // prefill landed
  await desc.fill("after");
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  const after = await (await page.request.get(`/api/v1/support-centers/${encodeURIComponent(name)}`)).json();
  expect(after.description).toBe("after");
  expect(after.extra?.Contacts).toEqual(contacts);
  expect(after.id).toBe(before.id);

  await page.goto(`/support-centers/detail?name=${encodeURIComponent(name)}`);
  await expect(page.getByText("A Real Person")).toBeVisible();
});

test("renaming carries the resource groups that use it; deleting an in-use one is blocked", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const sites = await (await page.request.get("/api/v1/sites")).json();
  test.skip(!sites || sites.length === 0, "needs a site");
  const ts = Date.now();
  const sc = `E2E_SCUse_${ts}`;
  const renamed = `E2E_SCUse_${ts}_renamed`;
  const rg = `E2E_SCUseRG_${ts}`;

  expect((await seed(page, "create", undefined, { name: sc })).ok()).toBeTruthy();
  const rgCreate = await page.request.post("/api/v1/proposals", {
    data: { entity_kind: "resource_group", operation: "create", submit: true,
      proposed_state: { name: rg, site: sites[0].name, support_center: sc, production: true } },
  });
  expect(rgCreate.ok(), await rgCreate.text()).toBeTruthy();
  const rgApprove = await page.request.post(`/api/v1/proposals/${(await rgCreate.json()).id}/approve`);
  expect(rgApprove.ok(), await rgApprove.text()).toBeTruthy();

  // Deleting while a resource group uses it is refused at apply time.
  const blocked = await seed(page, "delete", sc, undefined);
  expect(blocked.ok()).toBeFalsy();
  expect(await blocked.text()).toContain("still use this support center");

  // Rename through the form: the resource group follows.
  await page.goto(`/support-centers/new?edit=${encodeURIComponent(sc)}`);
  const nameBox = field(page, "Name");
  await expect(nameBox).toHaveValue(sc);
  await nameBox.fill(renamed);
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);
  const rgDetail = await (await page.request.get(`/api/v1/resource-groups/${encodeURIComponent(rg)}`)).json();
  expect(rgDetail.support_center).toBe(renamed);
  const list: { name: string }[] = await (await page.request.get("/api/v1/support-centers")).json();
  expect(list.some((s) => s.name === sc)).toBe(false);
  expect(list.some((s) => s.name === renamed)).toBe(true);
});

test("deleting an unused support center works from the list", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const name = `E2E_SCDel_${Date.now()}`;
  expect((await seed(page, "create", undefined, { name })).ok()).toBeTruthy();

  await page.goto("/support-centers");
  await page.getByPlaceholder(/Search by name/).fill(name);
  page.once("dialog", (d) => d.accept());
  await page.getByRole("button", { name: `Delete ${name}` }).click();
  await expect(page.getByText("delete requested")).toBeVisible();

  const mine = await (await page.request.get("/api/v1/proposals?entity_kind=support_center&target_name=" + encodeURIComponent(name))).json();
  const del = mine.find((p: { operation: string }) => p.operation === "delete");
  expect(del).toBeTruthy();
  const approve = await page.request.post(`/api/v1/proposals/${del.id}/approve`);
  expect(approve.ok(), await approve.text()).toBeTruthy();
  const gone = await page.request.get(`/api/v1/support-centers/${encodeURIComponent(name)}`);
  expect(gone.status()).toBe(404);
});
