import { orderStorage } from '@shared/lib/storage'
import { orderApi } from '@entities/order'

/**
 * Резолвим orderId по InvId — строго по самому InvId, без подмены чужим заказом:
 * 1) localStorage invoice map (тот же браузер)
 * 2) API /orders/by-invoice/:invoiceId (другой браузер / очищенный storage)
 *
 * Если InvId передан, но сопоставить не удалось — возвращаем null (НЕ откатываемся
 * на «последний заказ» getOrderId(): это привело бы на чужой/неоплаченный заказ).
 * На «последний заказ» откатываемся ТОЛЬКО когда InvId вовсе отсутствует.
 */
export async function resolveOrderId(invId: string | null): Promise<string | null> {
  if (invId) {
    const fromStorage = orderStorage.getOrderIdByInvoice(Number(invId))
    if (fromStorage) return fromStorage
    // Пробуем API — нет токена, ответ только UUID, без PII.
    try {
      const res = await orderApi.getOrderIdByInvoice(Number(invId))
      if (res?.id) {
        orderStorage.saveInvoiceOrder(Number(invId), res.id)
        return res.id
      }
    } catch {
      // Недоступен API — не подставляем чужой заказ, отдаём null.
    }
    return null
  }
  return orderStorage.getOrderId()
}
