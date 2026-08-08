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

  // Шаг 1 платной воронки: демо ещё не оплачено — вместо оплаты песни предлагаем
  // послушать фрагмент за 50 ₽. Полная цена в этот момент клиенту не выставляется.
  test('заказ с неоплаченным демо предлагает послушать фрагмент за 50 ₽', async ({ page }) => {
    await mockOrderApi(page, { initialState: 'demo-unpaid', finalState: 'demo-unpaid', pollsBeforeComplete: 99 })

    await page.goto(statusUrl())

    await expect(page.getByText('Ожидание оплаты')).toBeVisible()
    await expect(page.getByRole('button', { name: /Послушать демо за 50 ₽/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /Перейти к оплате/ })).toHaveCount(0)
  })

  // Шаг 2: демо оплачено и готово — 50 ₽ зачтены, к доплате остаётся 940 ₽.
  test('после оплаты демо предлагает доплатить остаток', async ({ page }) => {
    await mockOrderApi(page, { initialState: 'demo-paid', finalState: 'demo-paid', pollsBeforeComplete: 99 })

    await page.goto(statusUrl())

    await expect(page.getByRole('button', { name: /Доплатить 940 ₽/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /Послушать демо за/ })).toHaveCount(0)
  })
})
