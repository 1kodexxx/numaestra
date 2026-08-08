import type { Page, Route } from '@playwright/test'
import {
  ACCESS_TOKEN,
  ORDER_ID,
  completedOrder,
  demoPaidOrder,
  demoUnpaidOrder,
  pendingOrder,
  processingOrder,
  queuedOrder,
} from '../../src/test/fixtures/orders'

type OrderState = 'pending' | 'demo-unpaid' | 'demo-paid' | 'queued' | 'processing' | 'completed'

function orderForState(state: OrderState) {
  switch (state) {
    case 'pending':
      return pendingOrder()
    case 'demo-unpaid':
      return demoUnpaidOrder()
    case 'demo-paid':
      return demoPaidOrder()
    case 'queued':
      return queuedOrder()
    case 'processing':
      return processingOrder()
    case 'completed':
      return completedOrder()
  }
}

/** Мокает API заказов для E2E без реального backend. */
export async function mockOrderApi(
  page: Page,
  options: {
    initialState?: OrderState
    finalState?: OrderState
    pollsBeforeComplete?: number
  } = {},
) {
  const {
    initialState = 'queued',
    finalState = 'completed',
    pollsBeforeComplete = 2,
  } = options

  let getCount = 0

  await page.route('**/api/v1/orders/**', async (route: Route) => {
    const url = route.request().url()
    const method = route.request().method()

    // Отдельная ветка: '/demo-payment-url' НЕ содержит подстроку '/payment-url'
    // (перед payment дефис, а не слеш), поэтому общий обработчик его не поймает.
    if (method === 'GET' && url.includes('/demo-payment-url')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ payment_url: 'https://auth.robokassa.ru/pay/e2e-demo' }),
      })
      return
    }

    if (method === 'GET' && url.includes('/payment-url')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ payment_url: 'https://auth.robokassa.ru/pay/e2e-test' }),
      })
      return
    }

    if (method === 'GET' && url.match(/\/api\/v1\/orders\/[^/]+$/)) {
      getCount += 1
      const state = getCount >= pollsBeforeComplete ? finalState : initialState
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(orderForState(state)),
      })
      return
    }

    if (method === 'GET' && url.endsWith('/api/v1/orders/')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      })
      return
    }

    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'not mocked' }),
    })
  })
}

export function statusUrl() {
  return `/status/${ORDER_ID}?token=${ACCESS_TOKEN}`
}

export { ORDER_ID, ACCESS_TOKEN }
