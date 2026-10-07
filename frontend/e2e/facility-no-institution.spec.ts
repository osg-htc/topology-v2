import { test, expect, Page } from "@playwright/test";
import { devLogin } from "./helpers";

// A facility with no institution on record (v1 tolerates InstitutionID: null;
// Gridplexus and NSF DC are real examples) must stay editable. Previously the
// form blocked submission and the apply step rejected it, so no change of any
// kind was possible for them.

type Fac = { name: string; institution_id?: string };

async function approveFromView(page: Page) {
  await expect(page).toHaveURL(/\/proposals\/view/);
  const id = new URL(page.url()).searchParams.get("id");
  const res = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(res.ok(), await res.text()).toBeTruthy();
}

test("a facility with no institution can be edited and submitted", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const facs: Fac[] = await (await page.request.get("/api/v1/facilities")).json();
  const noInst = facs.find((f) => !f.institution_id);
  test.skip(!noInst, "needs a facility with no institution (Gridplexus / NSF DC in the real data)");
  const name = noInst!.name;

  await page.goto(`/facilities/new?edit=${encodeURIComponent(name)}`);
  await expect(page.getByText("no institution on record")).toBeVisible();
  const nameBox = page.locator("label", { hasText: /^Name$/ }).locator("xpath=following-sibling::input");
  await expect(nameBox).toHaveValue(name); // prefill has landed

  // Resubmit unchanged: no institution picked, and it must go through. (An
  // unchanged resubmit leaves real data as found; the Go tests cover renames.)
  await page.getByRole("button", { name: "Submit for review" }).click();
  await approveFromView(page);

  const detail = await (await page.request.get(`/api/v1/facilities/${encodeURIComponent(name)}`)).json();
  expect(detail.name).toBe(name);
  expect(detail.institution_id ?? "").toBe("");
});

test("a facility that has an institution is not offered the optional-institution path", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const facs: Fac[] = await (await page.request.get("/api/v1/facilities")).json();
  const withInst = facs.find((f) => !!f.institution_id);
  test.skip(!withInst, "needs a facility with an institution");

  await page.goto(`/facilities/new?edit=${encodeURIComponent(withInst!.name)}`);
  const nameBox = page.locator("label", { hasText: /^Name$/ }).locator("xpath=following-sibling::input");
  await expect(nameBox).toHaveValue(withInst!.name);
  await expect(page.getByText("no institution on record")).toHaveCount(0);
});

test("a new facility still cannot be submitted without an institution", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  await page.goto("/facilities/new");
  await page.getByRole("textbox").first().fill(`E2E_NoInst_${Date.now()}`);
  await page.getByRole("button", { name: "Submit for review" }).click();
  await expect(page.getByText(/Pick an institution/i)).toBeVisible();
  await expect(page).not.toHaveURL(/\/proposals\/view/);
});
