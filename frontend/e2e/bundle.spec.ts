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
  // Facility: create new.
  await page.getByRole("button", { name: "Create new" }).nth(2).click();
  await page.getByPlaceholder("e.g. University of Chicago").fill(fac);

  await page.getByPlaceholder("host.example.org").fill(`${resName.toLowerCase().replace(/_/g, "-")}.example.org`);
  await page.getByPlaceholder("Search people…").first().fill("E2E Admin");
  await page.getByPlaceholder("ID", { exact: true }).first().fill("OSG1000016");

  await page.getByRole("button", { name: "Submit for review" }).click();
  await expect(page).toHaveURL(/\/proposals\/view/);
  await expect(page.getByText("pending")).toBeVisible();

  // Approve via the API, then confirm the full chain exists.
  const id = new URL(page.url()).searchParams.get("id");
  const approve = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(approve.ok()).toBeTruthy();

  const detail = await (await page.request.get(`/api/v1/resources/${resName}`)).json();
  expect(detail.name).toBe(resName);
  expect(detail.resource_group).toBe(rg);
  expect(detail.site).toBe(site);
  expect(detail.facility).toBe(fac);
});
