import { Page, expect } from "@playwright/test";

// devLogin signs in via the development dev-login form (available when the
// server runs with APP_ENV=development, as the docker-compose stack does).
export async function devLogin(page: Page, role: string, email?: string) {
  await page.goto("/login");
  // The "Dev login" section only renders in development mode.
  await expect(page.getByText("Dev login")).toBeVisible();
  if (email) {
    await page.getByPlaceholder("you@example.org").fill(email);
  }
  await page.getByRole("combobox").selectOption(role);
  await page.getByRole("button", { name: "Dev sign in" }).click();
  // Redirects to the dashboard.
  await expect(page).toHaveURL(/\/$|\/$/);
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
}
