import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// Registering a resource while creating its parent group/site/facility inline
// produces one atomic bundle change request; approving it materializes the whole
// chain in dependency order.
test("inline parent creation submits an atomic bundle", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");

  const ts = Date.now();
  const fac = `E2E_Fac_${ts}`;
  const site = `E2E_Site_${ts}`;
  const rg = `E2E_RG_${ts}`;
  const resName = `E2E_Res_${ts}`;

  await page.goto("/proposals/new");
  await page.getByPlaceholder("e.g. UChicago_OSGConnect_ap20").fill(resName);

  // Switch the resource-group picker to "Create new" and build a fresh chain.
  await page.getByRole("button", { name: "Create new" }).first().click();
  await page.getByPlaceholder("e.g. UChicago_ClusterA").fill(rg);
  // Site: create new.
  await page.getByRole("button", { name: "Create new" }).nth(1).click();
  await page.getByPlaceholder("e.g. UChicago", { exact: true }).fill(site);
  // Facility: create new — a facility requires a real institution.
  await page.getByRole("button", { name: "Create new" }).nth(2).click();
  await page.getByPlaceholder("e.g. University of Chicago").fill(fac);
  const insts = await (await page.request.get("/api/v1/institutions?q=a")).json();
  test.skip(!insts || insts.length === 0, "needs cached institutions (admin → Sync)");
  const instName: string = insts[0].name;
  await page.getByPlaceholder("Search your institution by name…").fill(instName);
  await page.getByRole("button", { name: instName }).first().click();

  await page.getByPlaceholder("host.example.org").fill(`${resName.toLowerCase().replace(/_/g, "-")}.example.org`);
  // Contacts must be a real, picked person -- typing alone never resolves
  // to one. "Dev User" matches the signed-in dev-login account itself
  // (its display name when no explicit one is set); which real match gets
  // clicked doesn't matter, only that it's a real one.
  await page.getByPlaceholder("Search people…").first().fill("Dev User");
  await page.getByRole("button", { name: "Dev User" }).first().click();

  await page.getByRole("button", { name: "Submit for review" }).click();
  await expect(page).toHaveURL(/\/proposals\/view/);
  await expect(page.getByText("pending")).toBeVisible();

  // Approve via the API, then confirm the full chain exists.
  const id = new URL(page.url()).searchParams.get("id");
  const approve = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(approve.ok()).toBeTruthy();

  // /api/v1/resources/{id} takes the resource's numeric topology_id, not its
  // name -- look it up in the name-keyed list first to get that id, the same
  // two-step a real user follows (search/browse, then open the detail page).
  const list = await (await page.request.get("/api/v1/resources")).json();
  const resID = list[resName]?.id;
  expect(resID).toBeTruthy();

  const detail = await (await page.request.get(`/api/v1/resources/${resID}`)).json();
  expect(detail.name).toBe(resName);
  expect(detail.resource_group).toBe(rg);
  expect(detail.site).toBe(site);
  expect(detail.facility).toBe(fac);
});
