import { test, expect } from '@playwright/test'

test.describe('Мобильная навигация', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test('меню открывает ссылки на разделы', async ({ page }) => {
    await page.goto('/')

    await expect(page.getByRole('button', { name: 'Открыть меню' })).toBeVisible()
    await page.getByRole('button', { name: 'Открыть меню' }).click()

    const dialog = page.getByRole('dialog', { name: 'Навигация' })
    await expect(dialog).toBeVisible()
    await expect(dialog.getByRole('link', { name: /Как это работает/ })).toBeVisible()

    await dialog.getByRole('link', { name: /Отзывы/ }).click()
    await expect(page).toHaveURL(/\/reviews/)
  })
})
