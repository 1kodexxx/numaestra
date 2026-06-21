import { beforeEach, describe, expect, it } from 'vitest'
import { orderStorage } from './storage'

describe('orderStorage', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('сохраняет и читает id заказа и токен доступа', () => {
    orderStorage.saveOrder('order-1', 'token-abc')

    expect(orderStorage.getOrderId()).toBe('order-1')
    expect(orderStorage.getAccessToken()).toBe('token-abc')
  })

  it('возвращает null, если ничего не сохранено', () => {
    expect(orderStorage.getOrderId()).toBeNull()
    expect(orderStorage.getAccessToken()).toBeNull()
  })

  it('clear() удаляет id заказа и токен', () => {
    orderStorage.saveOrder('order-1', 'token-abc')
    orderStorage.clear()

    expect(orderStorage.getOrderId()).toBeNull()
    expect(orderStorage.getAccessToken()).toBeNull()
  })
})
