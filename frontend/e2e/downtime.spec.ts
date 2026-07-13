import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// Exercises the register-a-downtime -> proposal flow through the UI, then
// confirms the approved downtime is applied and visible via the API.
test("register a downtime creates and applies a proposal", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");

  const resMap = await (await page.request.get("/api/v1/resources")).json();
  const names = Object.values(resMap ?? {}).map((r: any) => r.name);
  test.skip(names.length === 0, "needs at least one resource (import data first)");
  const resource: string = names[0];

  await page.goto(`/downtimes/new?resource=${encodeURIComponent(resource)}`);
  // Prefilled resource is disabled; fill the required time fields.
  await page.locator('input[type="datetime-local"]').first().fill("2027-01-01T10:00");
  await page.locator('input[type="datetime-local"]').nth(1).fill("2027-01-01T12:00");

  const submit = page.getByRole("button", { name: "Submit for review" });
  await expect(submit).toBeEnabled();
  await submit.click();

  await expect(page).toHaveURL(/\/proposals\/view/);
  await expect(page.getByText("pending")).toBeVisible();

  // Grab the proposal id from the URL and approve it via the API, then confirm
  // the downtime is now live on the resource.
  const id = new URL(page.url()).searchParams.get("id");
  const approve = await page.request.post(`/api/v1/proposals/${id}/approve`);
  expect(approve.ok()).toBeTruthy();

  const dts = await (
    await page.request.get(`/api/v1/downtimes?resource=${encodeURIComponent(resource)}`)
  ).json();
  const mine = dts.find((d: any) => d.start_time.includes("Jan 01, 2027"));
  expect(mine).toBeTruthy();
  expect(mine.resource).toBe(resource);
});

// end must be after start: the form should block submission and highlight it.
test("downtime form rejects an end before the start", async ({ page }) => {
  await devLogin(page, "administrator", "admin@example.org");
  const resMap = await (await page.request.get("/api/v1/resources")).json();
  const names = Object.values(resMap ?? {}).map((r: any) => r.name);
  test.skip(names.length === 0, "needs at least one resource");

  await page.goto(`/downtimes/new?resource=${encodeURIComponent(names[0])}`);
  await page.locator('input[type="datetime-local"]').first().fill("2027-01-02T10:00");
  await page.locator('input[type="datetime-local"]').nth(1).fill("2027-01-01T10:00");
  await page.getByRole("button", { name: "Submit for review" }).click();

  await expect(page).not.toHaveURL(/\/proposals\/view/);
  await expect(page.getByText(/end must be after start/i)).toBeVisible();
});
