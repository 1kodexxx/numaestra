const ORDER_ID_KEY = 'numaestra_order_id'
const ACCESS_TOKEN_KEY = 'numaestra_access_token'
const INVOICE_MAP_KEY = 'numaestra_invoice_map'
const IDEMPOTENCY_KEY = 'numaestra_idempotency_key'

// readInvoiceMap безопасно читает карту invoice→orderId. Повреждённый JSON
// (ручная правка localStorage, недописанная запись) не должен ронять оплату —
// возвращаем пустую карту вместо исключения.
function readInvoiceMap(): Record<string, string> {
  const raw = localStorage.getItem(INVOICE_MAP_KEY)
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, string>) : {}
  } catch {
    return {}
  }
}

export const orderStorage = {
  saveOrder(id: string, token: string) {
    localStorage.setItem(ORDER_ID_KEY, id)
    localStorage.setItem(ACCESS_TOKEN_KEY, token)
  },
  saveInvoiceOrder(invoiceId: number, orderId: string) {
    const map = readInvoiceMap()
    map[String(invoiceId)] = orderId
    localStorage.setItem(INVOICE_MAP_KEY, JSON.stringify(map))
  },
  getOrderIdByInvoice(invoiceId: number): string | null {
    return readInvoiceMap()[String(invoiceId)] ?? null
  },
  getOrderId(): string | null {
    return localStorage.getItem(ORDER_ID_KEY)
  },
  getAccessToken(): string | null {
    return localStorage.getItem(ACCESS_TOKEN_KEY)
  },
  // Idempotency-Key стабильный на сессию: генерируется при первом обращении,
  // сохраняется до успешного создания заказа, потом очищается.
  getOrCreateIdempotencyKey(): string {
    let key = sessionStorage.getItem(IDEMPOTENCY_KEY)
    if (!key) {
      key = crypto.randomUUID()
      sessionStorage.setItem(IDEMPOTENCY_KEY, key)
    }
    return key
  },
  clearIdempotencyKey() {
    sessionStorage.removeItem(IDEMPOTENCY_KEY)
  },

  // clear сбрасывает «текущий» заказ, но СОХРАНЯЕТ карту invoice→orderId:
  // она нужна, чтобы сопоставить заказ по InvId после возврата с оплаты, даже
  // если пользователь начал новый заказ (тогда ORDER_ID_KEY уже перезаписан).
  clear() {
    localStorage.removeItem(ORDER_ID_KEY)
    localStorage.removeItem(ACCESS_TOKEN_KEY)
  },
}
