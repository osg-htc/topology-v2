import { test, expect } from "@playwright/test";
import { devLogin } from "./helpers";

// A user adds an email on the account page, opens the (dev-returned) verify
// link, and the address flips from pending to verified.
test("email verification: request then confirm", async ({ page }) => {
  await devLogin(page, "user", `ev_${Date.now()}@example.org`);
  await page.goto("/account");

  await page.getByPlaceholder("you@example.org").fill("jane.doe@example.org");
  await page.getByRole("button", { name: "Send link" }).click();

  // Masked hint appears as pending, and dev mode surfaces the link.
  await expect(page.getByText("j***@example.org")).toBeVisible();
  await expect(page.getByText("pending")).toBeVisible();

  const link = page.getByRole("link", { name: /verify-email\?token=/ });
  await expect(link).toBeVisible();
  await link.click();

  await expect(page).toHaveURL(/\/verify-email/);
  await expect(page.getByText(/is now confirmed/)).toBeVisible();

  // The address is now verified.
  const list = await (await page.request.get("/api/v1/email-verifications")).json();
  expect(list[0].verified).toBe(true);
});

test("email verification rejects a bogus address", async ({ page }) => {
  await devLogin(page, "user", `ev2_${Date.now()}@example.org`);
  const res = await page.request.post("/api/v1/email-verifications", { data: { email: "not-an-email" } });
  expect(res.status()).toBe(400);
});
