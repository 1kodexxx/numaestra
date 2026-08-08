import { orderStorage } from '@shared/lib/storage'
import { orderApi } from '@entities/order'

/** Какая платёжная полоса заказа была оплачена. */
export type InvoiceKind = 'main' | 'demo'

export interface ResolvedPayment {
  orderId: string | null
  /** 'demo' — оплачен фрагмент, заказ остаётся неоплаченным. По умолчанию 'main'. */
  kind: InvoiceKind
}

/**
 * Резолвим orderId по InvId — строго по самому InvId, без подмены чужим заказом:
 * 1) localStorage invoice map (тот же браузер)
 * 2) API /orders/by-invoice/:invoiceId (другой браузер / очищенный storage)
 *
 * Если InvId передан, но сопоставить не удалось — возвращаем null (НЕ откатываемся
 * на «последний заказ» getOrderId(): это привело бы на чужой/неоплаченный заказ).
 * На «последний заказ» откатываемся ТОЛЬКО когда InvId вовсе отсутствует.
 *
 * Вид платежа (демо или песня) локальная карта хранит вместе с заказом, поэтому
 * быстрый путь по-прежнему обходится без обращения к API.
 */
export async function resolvePayment(invId: string | null): Promise<ResolvedPayment> {
  if (!invId) return { orderId: orderStorage.getOrderId(), kind: 'main' }

  const invoiceId = Number(invId)
  const fromStorage = orderStorage.getInvoiceEntry(invoiceId)
  if (fromStorage) return fromStorage

  // Нет токена, ответ только UUID и вид платежа — без PII.
  try {
    const res = await orderApi.getOrderIdByInvoice(invoiceId)
    if (res?.id) {
      const kind: InvoiceKind = res.kind === 'demo' ? 'demo' : 'main'
      orderStorage.saveInvoiceOrder(invoiceId, res.id, kind)
      return { orderId: res.id, kind }
    }
  } catch {
    // Недоступен API — не подставляем чужой заказ, отдаём null.
  }

  return { orderId: null, kind: 'main' }
}

/** Совместимость: только идентификатор заказа, без вида платежа. */
export async function resolveOrderId(invId: string | null): Promise<string | null> {
  const { orderId } = await resolvePayment(invId)
  return orderId
}
