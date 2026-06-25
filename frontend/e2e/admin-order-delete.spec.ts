import { test, expect } from '@playwright/test'
import { ORDER_ID } from './fixtures/api-mock'
import { mockAdminOrderApi } from './fixtures/admin-api-mock'

test.describe('Админка — удаление заказа', () => {
  test('удаляет заказ и возвращает к списку', async ({ page }) => {
    const api = await mockAdminOrderApi(page)

    page.on('dialog', (dialog) => dialog.accept())

    await page.goto(`/admin/orders/${ORDER_ID}`)

    await expect(page.getByRole('heading', { name: /Заказ #4242/ })).toBeVisible()
    await page.getByRole('button', { name: 'Удалить заказ' }).click()

    await expect(page).toHaveURL(/\/admin\/orders$/)
    expect(api.wasDeleted()).toBe(true)
  })

  test('не удаляет заказ при отмене confirm', async ({ page }) => {
    const api = await mockAdminOrderApi(page)

    page.on('dialog', (dialog) => dialog.dismiss())

    await page.goto(`/admin/orders/${ORDER_ID}`)

    await expect(page.getByRole('button', { name: 'Удалить заказ' })).toBeVisible()
    await page.getByRole('button', { name: 'Удалить заказ' }).click()

    await expect(page).toHaveURL(new RegExp(`/admin/orders/${ORDER_ID}`))
    expect(api.wasDeleted()).toBe(false)
  })
})
