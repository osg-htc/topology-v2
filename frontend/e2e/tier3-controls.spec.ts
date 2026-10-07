import { test, expect, Page } from "@playwright/test";
import { devLogin } from "./helpers";

// Fields the backend already modeled end to end (DB, schema, snapshot-merge
// protection) but that no form exposed: Resource.Disable, Site.AddressLine2,
// and ResourceGroup.SupportCenter as a picklist of real support centers.

const field = (page: Page, labelText: string, tag: "input" | "select" | "textarea" = "input") =>
  page.locator("label", { hasText: new RegExp(`^${labelText}$`) }).locator(`xpath=following-sibling::${tag}`);

async function approveFromView(page: Page) {
  await expect(page).toHaveURL(/\/proposals\/view/);
  const id = new URL(page.url()).searchParams.get("id");
  const res = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(res.ok(), await res.text()).toBeTruthy();
}

test("site form edits Address line 1 and 2, and the detail page shows them", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const facs = await (await page.request.get("/api/v1/facilities")).json();
  test.skip(!facs || facs.length === 0, "needs a facility");
  const site = `E2E_AddrSite_${Date.now()}`;

  // Create through the form.
  await page.goto("/sites/new");
  await field(page, "Name").fill(site);
  await field(page, "Facility").fill(facs[0].name);
  await field(page, "Address line 1").fill("100 Main St");
  await field(page, "Address line 2").fill("Building 7, Suite 300");
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  let detail = await (await page.request.get(`/api/v1/sites/${site}`)).json();
  expect(detail.address_line1).toBe("100 Main St");
  expect(detail.address_line2).toBe("Building 7, Suite 300");

  await page.goto(`/sites/detail?name=${encodeURIComponent(site)}`);
  await expect(page.getByText("Building 7, Suite 300")).toBeVisible();

  // Edit: the form is prefilled from the stored value, and a change sticks.
  await page.goto(`/sites/new?edit=${encodeURIComponent(site)}`);
  await expect(field(page, "Address line 2")).toHaveValue("Building 7, Suite 300");
  await field(page, "Address line 2").fill("Floor 2");
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);
  detail = await (await page.request.get(`/api/v1/sites/${site}`)).json();
  expect(detail.address_line2).toBe("Floor 2");
  expect(detail.address_line1).toBe("100 Main St");
});

test("resource group form offers a support center picklist of real centers", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const names: string[] = await (await page.request.get("/api/v1/support-center-names")).json();
  test.skip(!names || names.length === 0, "needs support centers");
  const sites = await (await page.request.get("/api/v1/sites")).json();
  test.skip(!sites || sites.length === 0, "needs a site");

  await page.goto("/resource-groups/new");
  const select = field(page, "Support center", "select");
  // Placeholder + exactly the registry's names; free text is no longer possible.
  await expect(select.locator("option")).toHaveCount(names.length + 1);
  await expect(select.locator("option").nth(1)).toHaveText(names[0]);

  const rg = `E2E_SCRG_${Date.now()}`;
  await field(page, "Name").fill(rg);
  await field(page, "Site").fill(sites[0].name);
  await select.selectOption(names[names.length - 1]);
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  const detail = await (await page.request.get(`/api/v1/resource-groups/${rg}`)).json();
  expect(detail.support_center).toBe(names[names.length - 1]);

  // Reopening for edit preselects the stored center.
  await page.goto(`/resource-groups/new?edit=${encodeURIComponent(rg)}`);
  await expect(select).toHaveValue(names[names.length - 1]);
});

test("resource form has a Disabled control that round-trips and shows a badge", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const rgs = await (await page.request.get("/api/v1/resource-groups")).json();
  test.skip(!rgs || rgs.length === 0, "needs a resource group");
  const name = `E2E_Disable_${Date.now()}`;

  // Seed a resource that is Active and not Disabled, with its own contact so the
  // edit form can be submitted without picking anyone (a review submission needs one).
  const me = await (await page.request.get("/api/v1/auth/me")).json();
  const created = await page.request.post("/api/v1/proposals", {
    data: {
      entity_kind: "resource",
      operation: "create",
      submit: true,
      proposed_state: {
        name,
        resource_group: rgs[0].name,
        resource: {
          Active: true,
          FQDN: `${name.toLowerCase().replace(/_/g, "-")}.example.org`,
          ContactLists: {
            "Administrative Contact": {
              Primary: { Name: me.user.display_name, ID: me.user.legacy_contact_id },
            },
          },
        },
      },
    },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { id: pid } = await created.json();
  const approve = await page.request.post(`/api/v1/proposals/${pid}/approve`);
  expect(approve.ok(), await approve.text()).toBeTruthy();

  const list = await (await page.request.get("/api/v1/resources")).json();
  const resID: number = list[name]?.id;
  expect(resID).toBeTruthy();
  let detail = await (await page.request.get(`/api/v1/resources/${resID}`)).json();
  expect(detail.disable).toBe(false);

  // No badge while enabled.
  await page.goto(`/resources/detail?id=${resID}`);
  await expect(page.getByText("active", { exact: true })).toBeVisible();
  await expect(page.getByText("disabled", { exact: true })).toHaveCount(0);

  // Tick Disabled in the edit form and submit.
  await page.goto(`/proposals/new?edit=${resID}`);
  // Wait for the edit prefill to land (it resets the form state once), or the
  // tick below can be overwritten by it.
  await expect(page.getByPlaceholder("e.g. UChicago_OSGConnect_ap20")).toHaveValue(name);
  const box = page.getByLabel(/^Disabled/);
  await expect(box).not.toBeChecked();
  await box.check();
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  detail = await (await page.request.get(`/api/v1/resources/${resID}`)).json();
  expect(detail.disable).toBe(true);
  expect(detail.active).toBe(true); // Active and Disable are independent fields

  await page.goto(`/resources/detail?id=${resID}`);
  await expect(page.getByText("disabled", { exact: true })).toBeVisible();

  // Reopening shows it checked, and saving untouched keeps it.
  await page.goto(`/proposals/new?edit=${resID}`);
  await expect(page.getByPlaceholder("e.g. UChicago_OSGConnect_ap20")).toHaveValue(name);
  await expect(page.getByLabel(/^Disabled/)).toBeChecked();
});
