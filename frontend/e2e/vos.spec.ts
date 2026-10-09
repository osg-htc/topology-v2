import { test, expect, Page } from "@playwright/test";
import { devLogin } from "./helpers";

// VOs had no write path anywhere: the only way to create or change one was a
// full "Import from GitHub" resync. They now go through the same
// propose/approve workflow as every other entity.

async function approveFromView(page: Page) {
  await expect(page).toHaveURL(/\/proposals\/view/);
  const id = new URL(page.url()).searchParams.get("id");
  const res = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(res.ok(), await res.text()).toBeTruthy();
}

const field = (page: Page, labelText: string, tag: "input" | "textarea" | "select" = "input") =>
  page.locator("label", { hasText: new RegExp(`^${labelText}$`) }).locator(`xpath=following-sibling::${tag}`);

// Files and approves a VO proposal through the API (seeding).
async function seed(page: Page, operation: string, target: string | undefined, state: unknown) {
  const created = await page.request.post("/api/v1/proposals", {
    data: { entity_kind: "vo", operation, target_name: target, submit: true, proposed_state: state },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { id } = await created.json();
  return page.request.post(`/api/v1/proposals/${id}/approve`);
}

const docOf = async (page: Page, name: string) =>
  (await (await page.request.get(`/api/v1/vos/${encodeURIComponent(name)}/document`)).json()).vo;

test("create a VO through the form; it appears in the list, detail and the public vosummary feed", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const name = `E2E_VO_${Date.now()}`;

  await page.goto("/vos");
  await expect(page.getByRole("heading", { name: "Virtual organizations" })).toBeVisible();
  await page.getByRole("link", { name: "New VO" }).click();

  // The name is validated as a file name.
  await field(page, "Name").fill("bad name!");
  await expect(page.getByText(/starting with a letter or digit/)).toBeVisible();
  await field(page, "Name").fill(name);
  await field(page, "Long name").fill("E2E Virtual Organization");
  await field(page, "Community").fill("E2E community");
  await field(page, "Application description", "textarea").fill("made through the form");
  await field(page, "Primary URL").fill("https://example.org/e2e");
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  const doc = await docOf(page, name);
  expect(doc.LongName).toBe("E2E Virtual Organization");
  // Defaults (active, not disabled) are not written into the file...
  expect("Active" in doc).toBe(false);
  expect("Disable" in doc).toBe(false);
  // ...but the feed still says so.

  await page.goto(`/vos/detail?name=${encodeURIComponent(name)}`);
  await expect(page.getByText("made through the form")).toBeVisible();

  await page.goto("/vos");
  await page.getByPlaceholder(/Search by name/).fill(name);
  await expect(page.getByText(name, { exact: true })).toBeVisible();

  // The legacy feed the whole ecosystem reads reflects it.
  const feed = await (await page.request.get("/vosummary/xml")).text();
  expect(feed).toContain(`<Name>${name}</Name>`);
  expect(feed).toContain("E2E Virtual Organization");
  expect(feed).toMatch(new RegExp(`<Name>${name}</Name>[\\s\\S]*?<Active>true</Active>`));
});

test("editing through the form keeps OASIS and Credentials, which the form has no control for", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const name = `E2E_VOKeep_${Date.now()}`;
  const oasis = { UseOASIS: true, OASISRepoURLs: ["http://example.org/cvmfs/repo"] };
  const credentials = { TokenIssuers: [{ URL: "https://issuer.example.org", DefaultUnixUser: "e2e" }] };
  const made = await seed(page, "create", undefined, {
    name, vo: { LongName: "Keep Me", Community: "c", OASIS: oasis, Credentials: credentials },
  });
  expect(made.ok(), await made.text()).toBeTruthy();
  const before = await (await page.request.get(`/api/v1/vos/${encodeURIComponent(name)}/document`)).json();

  await page.goto(`/vos/new?edit=${encodeURIComponent(name)}`);
  const longName = field(page, "Long name");
  await expect(longName).toHaveValue("Keep Me"); // prefill landed
  await longName.fill("Keep Me, Edited");
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  const after = await (await page.request.get(`/api/v1/vos/${encodeURIComponent(name)}/document`)).json();
  expect(after.vo.LongName).toBe("Keep Me, Edited");
  expect(after.vo.OASIS).toEqual(oasis);
  expect(after.vo.Credentials).toEqual(credentials);
  expect(after.id).toBe(before.id);
});

test("parent VOs: set from the form, shown on the list, protected from delete and from cycles", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const ts = Date.now();
  const parent = `E2E_VOParent_${ts}`;
  const child = `E2E_VOChild_${ts}`;
  expect((await seed(page, "create", undefined, { name: parent, vo: { LongName: "Parent", Community: "c" } })).ok()).toBeTruthy();
  const parentId = (await (await page.request.get(`/api/v1/vos/${parent}`)).json()).id;

  await page.goto("/vos/new");
  await field(page, "Name").fill(child);
  await field(page, "Long name").fill("Child");
  await field(page, "Community").fill("c");
  await field(page, "Parent VO", "select").selectOption(parent);
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  const doc = await docOf(page, child);
  expect(doc.ParentVO).toEqual({ ID: parentId, Name: parent });
  const detail = await (await page.request.get(`/api/v1/vos/${parent}`)).json();
  expect(detail.child_vos).toEqual([child]);

  // The parent can't be deleted while it has a child, nor made a child of its own child.
  const blocked = await seed(page, "delete", parent, undefined);
  expect(blocked.ok()).toBeFalsy();
  expect(await blocked.text()).toContain("have it as their parent");
  const cycle = await seed(page, "update", parent, { name: parent, vo: { ParentVO: { Name: child } } });
  expect(cycle.ok()).toBeFalsy();
  expect(await cycle.text()).toContain("cycle");
});

test("the public VO detail withholds contact ids; the authenticated document has them", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const me = (await (await page.request.get("/api/v1/auth/me")).json()).user;
  test.skip(!me.legacy_contact_id, "needs a signed-in user with a contact id");
  const name = `E2E_VOPriv_${Date.now()}`;
  const made = await seed(page, "create", undefined, {
    name, vo: { LongName: "Private", Community: "c",
      Contacts: { "Administrative Contact": [{ ID: me.legacy_contact_id, Name: me.display_name }] } },
  });
  expect(made.ok(), await made.text()).toBeTruthy();

  const publicBody = await (await page.request.get(`/api/v1/vos/${name}`)).text();
  expect(publicBody).not.toContain(me.legacy_contact_id);
  expect(publicBody).toContain(me.display_name);
  const full = await (await page.request.get(`/api/v1/vos/${name}/document`)).text();
  expect(full).toContain(me.legacy_contact_id);

  // A person who isn't a known user can't be added as a new contact.
  const ghost = await seed(page, "update", name, {
    name, vo: { Contacts: { "Administrative Contact": [{ ID: me.legacy_contact_id, Name: me.display_name }, { ID: "ghost123", Name: "Ghost" }] } },
  });
  expect(ghost.ok()).toBeFalsy();
  expect(await ghost.text()).toContain("not linked to a known person");
});

test("deleting an unused VO works from the list", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const name = `E2E_VODel_${Date.now()}`;
  expect((await seed(page, "create", undefined, { name, vo: { LongName: "Delete me", Community: "c" } })).ok()).toBeTruthy();

  await page.goto("/vos");
  await page.getByPlaceholder(/Search by name/).fill(name);
  page.once("dialog", (d) => d.accept());
  await page.getByRole("button", { name: `Delete ${name}` }).click();
  await expect(page.getByText("delete requested")).toBeVisible();

  const mine = await (await page.request.get("/api/v1/proposals?entity_kind=vo&target_name=" + encodeURIComponent(name))).json();
  const del = mine.find((p: { operation: string }) => p.operation === "delete");
  expect(del).toBeTruthy();
  const approve = await page.request.post(`/api/v1/proposals/${del.id}/approve`);
  expect(approve.ok(), await approve.text()).toBeTruthy();
  expect((await page.request.get(`/api/v1/vos/${name}`)).status()).toBe(404);
});
