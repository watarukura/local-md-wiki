import { test, expect } from "@playwright/test";

test.describe("Local MD Wiki E2E", () => {
  test.beforeEach(async ({ page }) => {
    // Standard setup for each test
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        console.log(`BROWSER ERROR: ${msg.text()}`);
      }
    });
    page.on("pageerror", (err) => console.log(`BROWSER PAGE ERROR: ${err.message}`));
  });

  test("Home page should display correctly", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle(/Local Markdown Wiki/);
    await expect(page.locator("#page-name")).toHaveValue("Home.md");
    // Wait for openPage to complete (edit button becomes visible)
    await expect(page.locator("#edit-button")).toBeVisible();
    await expect(page.locator(".sidebar h1", { hasText: "Pages" })).toBeVisible();
  });

  test("Should create a new page", async ({ page }) => {
    const timestamp = Date.now();
    const newPageName = `NewTestPage_${timestamp}.md`;

    await page.goto("/");
    await expect(page.locator("#edit-button")).toBeVisible();
    
    // Handle prompt dialog
    page.once("dialog", async (dialog) => {
      await dialog.accept(newPageName);
    });
    await page.click("#new-page-button");

    const editor = page.locator("#editor");
    await expect(editor).toBeVisible({ timeout: 15000 });
    
    // Wait for openPage to complete and show the new page name
    await expect(page.locator("#page-name")).toHaveValue(newPageName);
    
    const cmContent = page.locator(".cm-content");
    await cmContent.click();
    await cmContent.fill("# New Test Page Content");
    
    await page.click("#save-button");

    // After save, it should be in the sidebar (check data-page attribute)
    await expect(page.locator(`.page-list a[data-page="${newPageName}"]`)).toBeVisible();
    // And in the viewer (check the rendered title)
    await expect(page.locator(".viewer h1")).toContainText("New Test Page Content");
  });

  test("Should edit an existing page", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#edit-button")).toBeVisible();
    
    await page.click("#edit-button");
    
    const cmContent = page.locator(".cm-content");
    await cmContent.click();
    await cmContent.fill("# Edited Home\nThis is edited content.");
    await page.click("#save-button");
    
    await expect(page.locator(".viewer h1")).toContainText("Edited Home");
    await expect(page.locator(".viewer")).toContainText("This is edited content.");

    // Restore to original content
    await page.click("#edit-button");
    await cmContent.click();
    await cmContent.fill("# Test Home\n");
    await page.click("#save-button");
    await expect(page.locator(".viewer h1")).toContainText("Test Home");
  });

  test("Should toggle sidebar on mobile view", async ({ page }) => {
    // Set to mobile size
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto("/");
    await expect(page.locator("#edit-button")).toBeVisible();

    const layout = page.locator(".layout");
    await expect(layout).not.toHaveClass(/sidebar-open/);

    await page.click("#menu-button");
    await expect(layout).toHaveClass(/sidebar-open/);

    await page.click("#menu-button");
    await expect(layout).not.toHaveClass(/sidebar-open/);
  });

  test("Should support dark mode based on OS settings", async ({ page }) => {
    // Test light mode
    await page.emulateMedia({ colorScheme: "light" });
    await page.goto("/");
    await expect(page.locator("#edit-button")).toBeVisible();
    
    const bodyBgLight = await page.evaluate(() => 
      getComputedStyle(document.body).backgroundColor
    );
    expect(bodyBgLight).toBe("rgb(255, 255, 255)");

    // Test dark mode
    await page.emulateMedia({ colorScheme: "dark" });
    // In dark mode, --bg-color is #0d1117 (rgb(13, 17, 23))
    await expect(async () => {
      const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
      expect(bg).toBe("rgb(13, 17, 23)");
    }).toPass();
    
    await page.click("#edit-button");
    const editor = page.locator(".cm-editor");
    
    // Check background color of editor to verify dark theme
    const editorBg = await editor.evaluate((el) => 
      getComputedStyle(el).backgroundColor
    );
    expect(editorBg).not.toBe("rgb(255, 255, 255)");
  });
});
