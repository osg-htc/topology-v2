import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// Exercises the register-a-resource -> proposal flow end to end through the UI.
// The RG field is a dropdown that requires an existing resource group, so the
// test picks a real one from the API.
test("register a resource creates a proposal", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");

  const rgs = await (await page.request.get("/api/v1/resource-groups")).json();
  test.skip(!rgs || rgs.length === 0, "needs at least one resource group (import data first)");
  const rgName: string = rgs[0].name;

  await page.goto("/proposals/new");
  const unique = `E2E_Res_${Date.now()}`;
  await page.getByPlaceholder("e.g. UChicago_OSGConnect_ap20").fill(unique);
  await page.getByPlaceholder("Search resource groups…").fill(rgName);
  // Host names must be valid DNS names (no underscores).
  await page.getByPlaceholder("host.example.org").fill(`${unique.toLowerCase().replace(/_/g, "-")}.example.org`);
  // A contact is required for a complete registration, and must be a real,
  // picked person -- typing alone never resolves to one. "Dev User" matches
  // the signed-in dev-login account itself (its display name when no
  // explicit one is set); which real match gets clicked doesn't matter,
  // only that it's a real one.
  await page.getByPlaceholder("Search people…").first().fill("Dev User");
  await page.getByRole("button", { name: "Dev User" }).first().click();

  const submit = page.getByRole("button", { name: "Submit for review" });
  await submit.click();

  await expect(page).toHaveURL(/\/proposals\/view/);
  await expect(page.getByText("pending")).toBeVisible();
  // The structured proposed-change view shows the resource name exactly (the
  // FQDN also contains it, so match exactly).
  await expect(page.getByText(unique, { exact: true })).toBeVisible();
});

// Direct regression test for the bug the contact picker rewrite fixes: a
// typed name that's never picked from the results list must never silently
// resolve to a contact -- submission must stay blocked, not proceed with an
// unlinked name.
test("a typed contact name that's never picked from results blocks submission", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");

  const rgs = await (await page.request.get("/api/v1/resource-groups")).json();
  test.skip(!rgs || rgs.length === 0, "needs at least one resource group (import data first)");
  const rgName: string = rgs[0].name;

  await page.goto("/proposals/new");
  const unique = `E2E_Unlinked_${Date.now()}`;
  await page.getByPlaceholder("e.g. UChicago_OSGConnect_ap20").fill(unique);
  await page.getByPlaceholder("Search resource groups…").fill(rgName);
  await page.getByPlaceholder("host.example.org").fill(`${unique.toLowerCase().replace(/_/g, "-")}.example.org`);
  // Type a name that won't match any real user, and never click a result.
  await page.getByPlaceholder("Search people…").first().fill("Nobody Real Zzyx");

  await page.getByText("Not linked to an account").waitFor();

  await page.getByRole("button", { name: "Submit for review" }).click();
  // Submission must be rejected client-side, not silently proceed.
  await expect(page.getByText("Please fix the highlighted fields.")).toBeVisible();
  await expect(page).not.toHaveURL(/\/proposals\/view/);
});

test("schema validation blocks an invalid resource (no FQDN)", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  // page.request shares the browser context's session cookie.
  const res = await page.request.post("/api/v1/proposals", {
    data: {
      entity_kind: "resource",
      operation: "create",
      proposed_state: { name: "X", resource_group: "RG", resource: { Active: true } },
    },
  });
  expect(res.status()).toBe(400);
  expect(await res.text()).toContain("FQDN");
});

// An administrator is both a valid creator and a valid reviewer, so this is
// exactly the case that hit the Actions-card bug: with no button gated to
// render on a decided proposal, the empty-state check keyed only on role
// (never on status) left the whole card blank for this viewer. Also checks
// that a second, still-pending proposal on the same resource is excluded
// from its entity history panel -- a still-pending edit isn't history yet.
test("approved proposal's Actions card explains it's decided; pending sibling stays out of entity history", async ({
  page,
}) => {
  await devLogin(page, "administrator", "admin@example.org");

  const rgs = await (await page.request.get("/api/v1/resource-groups")).json();
  test.skip(!rgs || rgs.length === 0, "needs at least one resource group (import data first)");
  const rgName: string = rgs[0].name;

  const unique = `E2E_Actions_${Date.now()}`;
  const host = `${unique.toLowerCase().replace(/_/g, "-")}.example.org`;

  const createRes = await page.request.post("/api/v1/proposals", {
    data: {
      entity_kind: "resource",
      operation: "create",
      submit: true,
      proposed_state: { name: unique, resource_group: rgName, resource: { FQDN: host, Active: true } },
    },
  });
  expect(createRes.ok()).toBeTruthy();
  const { id } = await createRes.json();

  const approveRes = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(approveRes.ok()).toBeTruthy();

  const proposal = await (await page.request.get(`/api/v1/proposals/${id}`)).json();
  const targetName: string = proposal.target_name;

  // A second, still-pending update on the same resource -- must not show on
  // its history panel below.
  const pendingRes = await page.request.post("/api/v1/proposals", {
    data: {
      entity_kind: "resource",
      operation: "update",
      target_name: targetName,
      submit: true,
      proposed_state: {
        name: unique,
        resource_group: rgName,
        resource: { FQDN: host, Active: true, Description: "a pending edit" },
      },
    },
  });
  expect(pendingRes.ok()).toBeTruthy();

  await page.goto(`/proposals/view?id=${id}`);
  await expect(page.getByText(/already been applied/)).toBeVisible();

  await page.goto(`/resources/detail?id=${targetName}`);
  await expect(page.getByText("Edit history")).toBeVisible();
  await expect(page.getByText("pending", { exact: true })).not.toBeVisible();
});
