import { test, expect } from '@playwright/test'
import { mockOrderApi, statusUrl } from './fixtures/api-mock'

test.describe('Статус заказа — готовый результат', () => {
  test('после оплаты показывает 4 варианта трека', async ({ page }) => {
    await mockOrderApi(page, { initialState: 'completed', finalState: 'completed', pollsBeforeComplete: 1 })

    await page.goto(statusUrl())

    await expect(page.getByText('Ваша песня готова!')).toBeVisible()
    await expect(page.getByText('Скачать треки')).toBeVisible()
    await expect(page.getByRole('button', { name: /Вариант [1-4]/ })).toHaveCount(8)
  })
})

test.describe('Статус заказа — путь оплата → генерация → 4 трека', () => {
  test('поллинг от очереди до готовности с четырьмя версиями', async ({ page }) => {
    await mockOrderApi(page, {
      initialState: 'queued',
      finalState: 'completed',
      pollsBeforeComplete: 2,
    })

    await page.goto(statusUrl())

    await expect(page.getByText('В очереди')).toBeVisible()

    await expect(page.getByText('Ваша песня готова!')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByRole('button', { name: /Вариант [1-4]/ })).toHaveCount(8)
  })

  test('неоплаченный заказ предлагает перейти к оплате', async ({ page }) => {
    await mockOrderApi(page, { initialState: 'pending', finalState: 'pending', pollsBeforeComplete: 99 })

    await page.goto(statusUrl())

    await expect(page.getByText('Ожидание оплаты')).toBeVisible()
    await expect(page.getByRole('button', { name: /Перейти к оплате/ })).toBeVisible()
  })
})
