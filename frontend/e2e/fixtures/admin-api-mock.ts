import type { Page, Route } from '@playwright/test'
import { ORDER_ID, makeTracks } from '../../src/test/fixtures/orders'

export function adminOrderDetail() {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    email: 'e2e@example.com',
    phone: '',
    brief: 'E2E заказ',
    amount_kopecks: 200_000,
    payment_status: 'paid',
    generation_status: 'completed',
    tracks: makeTracks(),
    created_at: new Date().toISOString(),
  }
}

/** Мокает админ-сессию и API заказа для E2E без backend. */
export async function mockAdminOrderApi(page: Page) {
  let deleted = false

  await page.route('**/api/v1/admin/**', async (route: Route) => {
    const url = route.request().url()
    const method = route.request().method()

    if (method === 'GET' && url.endsWith('/admin/me')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ login: 'e2e-admin' }),
      })
      return
    }

    if (method === 'GET' && url.match(/\/admin\/orders\/[^/?]+$/)) {
      if (deleted) {
        await route.fulfill({
          status: 404,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'заказ не найден' }),
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(adminOrderDetail()),
      })
      return
    }

    if (method === 'DELETE' && url.match(/\/admin\/orders\/[^/?]+$/)) {
      deleted = true
      await route.fulfill({ status: 204, body: '' })
      return
    }

    if (method === 'GET' && url.includes('/admin/stats')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ orders_total: 1, orders_paid: 1 }),
      })
      return
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([]),
    })
  })

  return {
    wasDeleted: () => deleted,
  }
}
