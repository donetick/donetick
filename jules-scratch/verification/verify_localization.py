import re
from playwright.sync_api import Page, expect

def test_localization(page: Page):
    """
    This test verifies that the localization is working.
    It navigates to the settings page, switches the language to Chinese,
    and takes a screenshot. It then switches back to English and
    verifies that a key string is in English.
    """
    # 1. Arrange: Go to the settings page.
    # The server should be running on localhost:5173 based on vite defaults.
    page.goto("http://localhost:5173/#/settings")

    # Wait for the page to load by looking for a known element.
    expect(page.get_by_role("heading", name="Theme preferences")).to_be_visible()

    # 2. Act: Switch to Chinese.
    chinese_button = page.get_by_role("button", name="中文")
    chinese_button.click()

    # 3. Assert: Check for a translated string.
    # The "greeting" key should now be in Chinese.
    expect(page.get_by_text("你好 (中文)")).to_be_visible()

    # 4. Screenshot: Capture the Chinese version of the settings page.
    page.screenshot(path="jules-scratch/verification/verification_zh.png")

    # 5. Act: Switch back to English.
    english_button = page.get_by_role("button", name="English")
    english_button.click()

    # 6. Assert: Check for the English string.
    expect(page.get_by_text("Hello (English)")).to_be_visible()

    # 7. Screenshot: Capture the English version.
    page.screenshot(path="jules-scratch/verification/verification_en.png")
